package sep1

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/posture-sec/guage/internal/sanitize"
)

// tomlPath is the fixed location SEP-1 v2.7.0 defines. Gauge never fetches a path
// a chain-derived value told it to fetch.
const tomlPath = "/.well-known/stellar.toml"

const (
	defaultTimeout    = 10 * time.Second
	defaultMaxBody    = 100 * 1024
	defaultMaxHops    = 3
	defaultMaxRetries = 2
	defaultBackoff    = 200 * time.Millisecond
	maxRetryAfter     = 5 * time.Second
	maxBackoff        = 5 * time.Second
	tlsVersionFloor   = uint16(tls.VersionTLS12)
)

// ErrOptions reports a fetcher configuration Gauge refuses to build.
var ErrOptions = errors.New("sep1: invalid fetcher options")

// Resolver is the DNS surface the guard needs. *net.Resolver satisfies it, and a
// test can hand back addresses that make the guard's decisions observable without
// touching a resolver.
type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// Options configures the identity fetcher. Every zero value selects the default
// named above; a negative value is rejected.
type Options struct {
	UserAgent  string
	Timeout    time.Duration
	MaxBody    int64
	MaxHops    int
	MaxRetries int
	Backoff    time.Duration
}

// Result is what one identity check observed. It deliberately carries no
// net/http, url or time types, so internal/rules can read it while staying a pure
// consumer of observations.
type Result struct {
	// HomeDomain is the validated host this result is about, or the sanitized
	// field that failed validation.
	HomeDomain string
	// FailedAt names the step that decided the outcome. StepOK means the document
	// was fetched under the guard and parsed.
	FailedAt Step
	// Detail is a fixed phrase naming the failure. It is never remote content: a
	// Location header and a TOML parse message can both carry escape sequences, and
	// the body itself must never leave the parser.
	Detail   string
	Status   int
	Hops     int
	Bytes    int
	Identity Identity
	// CORSPresent notes the Access-Control-Allow-Origin header SEP-1 mandates. It
	// matters to browsers, not to a CLI run, so it is a fact and never a finding.
	CORSPresent bool
}

// Reachable reports whether a stellar.toml was fetched under the guard and parsed
// with at least one identity key.
func (r Result) Reachable() bool { return r.FailedAt == StepOK }

// Fetcher acquires the identity document for one home domain. Only the guarded
// Client implements it in this repository; a test may substitute a fake to drive
// orchestration, which proves nothing about the guard.
type Fetcher interface {
	Fetch(ctx context.Context, homeDomain string) Result
}

// Client fetches stellar.toml documents through the dial-time guard.
type Client struct {
	opts      Options
	resolver  Resolver
	dial      dialFunc
	tlsConfig *tls.Config
	client    *http.Client
	sleep     func(context.Context, time.Duration) error
}

// NewClient builds a guarded fetcher. The transport never reads the proxy
// environment: with Proxy unset and ProxyFromEnvironment off, an HTTPS_PROXY value
// would otherwise make the guard validate the proxy instead of the target, which is
// a complete bypass.
func NewClient(opts Options) (*Client, error) {
	if opts.Timeout < 0 || opts.MaxBody < 0 || opts.MaxHops < 0 || opts.MaxRetries < 0 || opts.Backoff < 0 {
		return nil, fmt.Errorf("%w: durations and counts cannot be negative", ErrOptions)
	}
	c := &Client{opts: opts}
	if c.opts.Timeout == 0 {
		c.opts.Timeout = defaultTimeout
	}
	if c.opts.MaxBody == 0 {
		c.opts.MaxBody = defaultMaxBody
	}
	if c.opts.MaxHops == 0 {
		c.opts.MaxHops = defaultMaxHops
	}
	if c.opts.MaxRetries == 0 {
		c.opts.MaxRetries = defaultMaxRetries
	}
	if c.opts.Backoff == 0 {
		c.opts.Backoff = defaultBackoff
	}
	c.resolver = net.DefaultResolver
	c.dial = (&net.Dialer{Timeout: c.opts.Timeout}).DialContext
	c.sleep = sleepContext
	c.tlsConfig = &tls.Config{MinVersion: tlsVersionFloor}
	c.client = &http.Client{
		Transport: &http.Transport{
			DialContext:       c.gatedDial,
			TLSClientConfig:   c.tlsConfig,
			Proxy:             nil,
			DisableKeepAlives: true,
			MaxConnsPerHost:   1,
		},
		// No hop is followed by the client itself: Fetch validates every target
		// first. The same shape internal/source/http.go uses.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return c, nil
}

// Fetch reads https://{homeDomain}/.well-known/stellar.toml under the guard and
// never returns an error: an unreachable document is an observation, and the step
// that failed says which. Blocked addresses, refused hops and oversized bodies are
// reported, and none of them is retried into a stall.
func (c *Client) Fetch(ctx context.Context, homeDomain string) Result {
	domain, ok := HomeDomain(homeDomain)
	if !ok {
		return Result{HomeDomain: sanitize.Text(homeDomain), FailedAt: StepHostname,
			Detail: "the home domain is not a host name Gauge will contact"}
	}
	res := Result{HomeDomain: domain}

	// One deadline covers the whole hop chain, so three redirects cannot silently
	// quadruple the stated budget.
	reqCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()

	target := "https://" + domain + tomlPath
	for {
		out := c.get(reqCtx, target, &res)
		if out.next == "" {
			if out.step == StepOK {
				c.applyBody(&res, out.body)
				return res
			}
			res.FailedAt, res.Detail = out.step, out.detail
			return res
		}
		if res.Hops >= c.opts.MaxHops {
			res.FailedAt = StepRedirect
			res.Detail = fmt.Sprintf("the document moved more than %d times", c.opts.MaxHops)
			return res
		}
		res.Hops++
		target = out.next
	}
}

// applyBody parses a kept body and records what the document said.
func (c *Client) applyBody(res *Result, body []byte) {
	res.Bytes = len(body)
	id, step, detail := Parse(body)
	res.Identity = id
	if step != StepOK {
		res.FailedAt = step
		res.Detail = detail
	}
}

// outcome is one URL's result: the step that decided it, and at most one of a kept
// body or a validated next target to follow.
type outcome struct {
	step   Step
	detail string
	body   []byte
	next   string
	retry  bool
	wait   time.Duration
}

// get performs the bounded attempts for one URL, in the shape
// internal/source/http.go established: only a transport or status failure is
// retried, a Retry-After delay wins over the client's own backoff, and a cancelled
// context stops immediately.
func (c *Client) get(ctx context.Context, target string, res *Result) outcome {
	var last outcome
	var wait time.Duration
	for attempt := 0; attempt <= c.opts.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := wait
			if delay <= 0 {
				delay = backoffFor(c.opts.Backoff, attempt)
			}
			if err := c.sleep(ctx, delay); err != nil {
				return outcome{step: StepTransport, detail: "the deadline ended before the retry"}
			}
		}
		out := c.once(ctx, target, res)
		if out.step != StepTransport && out.step != StepStatus {
			return out
		}
		last = out
		if !out.retry || ctx.Err() != nil {
			return out
		}
		wait = out.wait
	}
	return last
}

// once performs exactly one HTTP request against target.
func (c *Client) once(ctx context.Context, target string, res *Result) outcome {
	if err := ctx.Err(); err != nil {
		return outcome{step: StepTransport, detail: "the request was cancelled"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return outcome{step: StepHostname, detail: "the host name cannot be requested"}
	}
	if c.opts.UserAgent != "" {
		req.Header.Set("User-Agent", c.opts.UserAgent)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		var blocked *blockedError
		var resolve *resolveError
		switch {
		case errors.As(err, &blocked):
			return outcome{step: StepGuard, detail: blocked.explain()}
		case errors.As(err, &resolve):
			return outcome{step: StepResolve, detail: "the host name does not resolve to an address"}
		case ctx.Err() != nil:
			return outcome{step: StepTransport, detail: "the request ended before the document arrived"}
		default:
			return outcome{step: StepTransport, detail: "the document could not be reached", retry: true}
		}
	}
	defer resp.Body.Close()
	res.Status = resp.StatusCode
	res.CORSPresent = resp.Header.Get("Access-Control-Allow-Origin") == "*"

	if isRedirect(resp.StatusCode) {
		next, err := c.followTarget(resp)
		if err != nil {
			return outcome{step: StepRedirect, detail: "the document moved somewhere Gauge will not follow"}
		}
		return outcome{step: StepOK, next: next}
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode <= 299:
	case resp.StatusCode == http.StatusTooManyRequests:
		return outcome{step: StepStatus, detail: "the server asked this client to wait",
			retry: true, wait: retryAfter(resp.Header)}
	case resp.StatusCode >= 500 && resp.StatusCode <= 599:
		return outcome{step: StepStatus, detail: "the server failed", retry: true}
	default:
		return outcome{step: StepStatus, detail: "the server refused the document"}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, c.opts.MaxBody+1))
	if err != nil {
		return outcome{step: StepTransport, detail: "the document ended before it was complete",
			retry: ctx.Err() == nil}
	}
	if int64(len(body)) > c.opts.MaxBody {
		// Nothing is kept from an oversized body, and Bytes stays unset so a report
		// never implies a document was read.
		return outcome{step: StepSize, detail: fmt.Sprintf("the document is larger than %d bytes", c.opts.MaxBody)}
	}
	return outcome{step: StepOK, body: body}
}

// followTarget turns a Location header into the next URL, or refuses it. Every hop
// is required to be an https origin with no credentials and no unusual port, so a
// document can neither downgrade the transport nor move to a host Gauge would have
// refused to name in the first place. The target is rebuilt from validated pieces
// and carries only printable ASCII: a byte that would land in the request line is
// refused rather than sent, and the header value is never echoed into a Detail.
func (c *Client) followTarget(resp *http.Response) (string, error) {
	if resp.Request == nil || resp.Request.URL == nil {
		return "", errors.New("sep1: a redirect arrived without the request it came from")
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", errors.New("sep1: a redirect named no target")
	}
	if !printableASCII(loc) {
		return "", errors.New("sep1: a redirect target carries characters Gauge will not send")
	}
	u, err := resp.Request.URL.Parse(loc)
	if err != nil {
		return "", err
	}
	if u.Scheme != "https" {
		return "", errors.New("sep1: a redirect off the document target is not https")
	}
	if u.User != nil {
		return "", errors.New("sep1: a redirect target carries credentials")
	}
	if port := u.Port(); port != "" && port != "443" {
		return "", errors.New("sep1: a redirect target names a port")
	}
	domain, err := ValidateHostname(u.Hostname())
	if err != nil {
		return "", err
	}
	target := "https://" + domain + requestPath(u)
	if u.RawQuery != "" {
		target += "?" + u.RawQuery
	}
	return target, nil
}

func requestPath(u *url.URL) string {
	path := u.EscapedPath()
	if path == "" {
		return "/"
	}
	return path
}

// printableASCII reports whether a value is safe to place in a request line. Bytes
// below the space, the delete character and anything non-ASCII are refused: Go does
// no IDNA mapping, so an internationalized hop is not something Gauge will send.
func printableASCII(value string) bool {
	for i := 0; i < len(value); i++ {
		if b := value[i]; b < 0x20 || b == 0x7f || b >= 0x80 {
			return false
		}
	}
	return true
}

// gatedDial is where the guard is enforced. It resolves the name the transport was
// asked to reach, classifies every address that answer contains, and then dials the
// numeric address it validated rather than the name. The socket therefore goes to
// the same IP that was checked, on every dial including a re-dial after a redirect,
// which is what a short time-to-live cannot buy. SNI and the Host header stay
// derived from the URL host, so certificate verification is untouched.
//
// resolver and dial are the two offline seams, and both are unexported: no caller
// outside this package can build a Client that skips the guard.
func (c *Client) gatedDial(ctx context.Context, network, addr string) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, fmt.Errorf("sep1: the guard dials TCP addresses only, not %q", network)
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("sep1: the guard expects a host and a port: %w", err)
	}
	domain, err := ValidateHostname(host)
	if err != nil {
		return nil, err
	}
	answers, err := c.resolver.LookupIPAddr(ctx, domain)
	if err != nil {
		return nil, &resolveError{host: domain, err: err}
	}
	if len(answers) == 0 {
		return nil, &resolveError{host: domain, err: errors.New("no addresses returned")}
	}
	for _, answer := range answers {
		class := ClassifyIP(answer.IP)
		if !class.Dialable() {
			return nil, &blockedError{host: domain, address: answer.String(), class: class}
		}
	}
	// Every answer was public, so any of them is safe to reach. The address handed
	// to the dialer is rebuilt from the parsed IP, never from the caller's string.
	return c.dial(ctx, network, net.JoinHostPort(answers[0].IP.String(), port))
}

// blockedError says the guard refused an address the resolver returned. It travels
// back through the transport, so its text is written for a log line: host is a
// validated name and address is a numeric one.
type blockedError struct {
	host    string
	address string
	class   Class
}

func (e *blockedError) Error() string {
	return fmt.Sprintf("sep1: %s resolves to %s, which is %s", e.host, e.address, e.class)
}

// explain is the report-facing form. It names the class rather than the address, so
// a hostile zone cannot use a Detail to print its own inventory.
func (e *blockedError) explain() string {
	return fmt.Sprintf("the host name resolves to a %s address, which Gauge will not dial", e.class)
}

// resolveError says the host name produced no address to classify.
type resolveError struct {
	host string
	err  error
}

func (e *resolveError) Error() string {
	return fmt.Sprintf("sep1: %s does not resolve: %v", e.host, e.err)
}

func (e *resolveError) Unwrap() error { return e.err }

func isRedirect(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

// retryAfter reads the integer-seconds form, which is the only form Gauge honours,
// and caps it so a hostile header cannot stall a run. Same rule as
// internal/source/http.go.
func retryAfter(header http.Header) time.Duration {
	value := strings.TrimSpace(header.Get("Retry-After"))
	if value == "" {
		return 0
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return 0
	}
	delay := time.Duration(seconds) * time.Second
	if delay > maxRetryAfter {
		return maxRetryAfter
	}
	return delay
}

func backoffFor(base time.Duration, attempt int) time.Duration {
	delay := base
	for i := 1; i < attempt; i++ {
		if delay > maxBackoff/2 {
			return maxBackoff
		}
		delay *= 2
	}
	if delay > maxBackoff {
		return maxBackoff
	}
	return delay
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
