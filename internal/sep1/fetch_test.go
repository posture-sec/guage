package sep1

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The offline method has two seams, both unexported fields assigned only from
// tests in this package, so nothing a caller outside it can reach turns the guard
// off. The resolver seam replaces DNS only: every hostname check, every address
// classification, the redirect re-validation, the size cap and the parse still run
// for real. The dial seam replaces only the last-mile socket, and the address it
// is handed is itself an assertion, because production must hand it a numeric
// answer the guard already classified.

type fakeResolver struct {
	mu      sync.Mutex
	answers map[string][]string
	err     error
	calls   map[string]int
}

func newResolver(answers map[string][]string) *fakeResolver {
	return &fakeResolver{answers: answers, calls: map[string]int{}}
}

func (f *fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[host]++
	if f.err != nil {
		return nil, f.err
	}
	answers, ok := f.answers[host]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: host}
	}
	out := make([]net.IPAddr, 0, len(answers))
	for _, a := range answers {
		ip := net.ParseIP(a)
		if ip == nil {
			return nil, &net.DNSError{Err: "invalid address", Name: host}
		}
		out = append(out, net.IPAddr{IP: ip})
	}
	return out, nil
}

func (f *fakeResolver) lookups(host string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[host]
}

type dialRecord struct {
	mu     sync.Mutex
	addrs  []string
	target string
	err    error
}

func (d *dialRecord) addresses() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.addrs...)
}

func (d *dialRecord) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.addrs)
}

// gated stands in for the socket only. A recorded address proves what the guard
// passed down; an empty target means nothing may be dialed at all.
func (d *dialRecord) gated(ctx context.Context, network, addr string) (net.Conn, error) {
	d.mu.Lock()
	d.addrs = append(d.addrs, addr)
	target := d.target
	d.mu.Unlock()
	if d.err != nil {
		return nil, d.err
	}
	if target == "" {
		return nil, errors.New("test: refusing to dial")
	}
	var netDialer net.Dialer
	return netDialer.DialContext(ctx, network, target)
}

func tomlBody(value string) string {
	return fmt.Sprintf("NETWORK_PASSPHRASE = %q\n", value)
}

func clientFor(t *testing.T, server *httptest.Server, answers map[string][]string) (*Client, *fakeResolver, *dialRecord) {
	t.Helper()
	c, err := NewClient(Options{UserAgent: "test", Timeout: 2 * time.Second, Backoff: time.Millisecond, MaxRetries: 1})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	c.tlsConfig.RootCAs = pool
	res := newResolver(answers)
	rec := &dialRecord{target: server.Listener.Addr().String()}
	c.resolver = res
	c.dial = rec.gated
	return c, res, rec
}

func TestFetchHappyPath(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/stellar.toml" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		requests.Add(1)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		fmt.Fprint(w, tomlBody(mainnetPass))
	}))
	defer server.Close()

	c, res, rec := clientFor(t, server, map[string][]string{"example.com": {"93.184.216.34"}})
	got := c.Fetch(context.Background(), "example.com")
	if !got.Reachable() {
		t.Fatalf("Fetch = %+v, want reachable", got)
	}
	if got.HomeDomain != "example.com" {
		t.Fatalf("HomeDomain = %q, want example.com", got.HomeDomain)
	}
	if got.Identity.NetworkPassphrase == nil || *got.Identity.NetworkPassphrase != mainnetPass {
		t.Fatalf("passphrase = %v, want the one served", got.Identity.NetworkPassphrase)
	}
	if !got.CORSPresent {
		t.Fatal("CORSPresent = false, want the header noted")
	}
	if got.Hops != 0 || got.Status != http.StatusOK || got.Bytes == 0 {
		t.Fatalf("Fetch = %+v, want no hops, status 200 and a body length", got)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want exactly one", requests.Load())
	}
	if rec.count() != 1 || rec.addresses()[0] != "93.184.216.34:443" {
		t.Fatalf("dial addresses = %v, want the validated numeric answer", rec.addresses())
	}
	if res.lookups("example.com") != 1 {
		t.Fatalf("lookups = %d, want one", res.lookups("example.com"))
	}
}

func TestFetchRefusesEveryBlockedClass(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"loopback v4":     "127.0.0.1",
		"loopback v6":     "::1",
		"loopback mapped": "::ffff:127.0.0.1",
		"private v4":      "10.0.0.8",
		"private v4 b":    "192.168.1.9",
		"metadata":        "169.254.169.254",
		"link-local v6":   "fe80::1",
		"unique local":    "fc00::1",
		"carrier nat":     "100.64.0.1",
		"unspecified v4":  "0.0.0.0",
		"unspecified v6":  "::",
		"nat64 wrapped":   "64:ff9b::a00:1",
		"sixtofour":       "2002:0a00:0001::1",
		"multicast":       "224.0.0.1",
	}
	for name, addr := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				t.Error("the test server was reached for a blocked address")
			}))
			defer server.Close()
			c, _, rec := clientFor(t, server, map[string][]string{"blocked.example": {addr}})
			got := c.Fetch(context.Background(), "blocked.example")
			if got.FailedAt != StepGuard {
				t.Fatalf("FailedAt = %v (%q), want guard", got.FailedAt, got.Detail)
			}
			if rec.count() != 0 {
				t.Fatalf("dial addresses = %v, want the guard to stop before any dial", rec.addresses())
			}
			if got.Reachable() {
				t.Fatal("Reachable() = true for a blocked address")
			}
		})
	}
}

func TestFetchBlocksWhenAnyAnswerIsBlocked(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("a rebinding answer must not be served")
	}))
	defer server.Close()
	// A hostile zone that lists a good address first and the metadata address
	// second. Every answer is classified, so the second one stops the dial that
	// would otherwise have used the first.
	c, _, rec := clientFor(t, server, map[string][]string{"rebind.example": {"93.184.216.34", "169.254.169.254"}})
	got := c.Fetch(context.Background(), "rebind.example")
	if got.FailedAt != StepGuard {
		t.Fatalf("FailedAt = %v (%q), want guard", got.FailedAt, got.Detail)
	}
	if rec.count() != 0 {
		t.Fatalf("dial addresses = %v, want no dial at all", rec.addresses())
	}
}

func TestFetchValidatesEachRedirectTarget(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		header  string
		status  int
		step    Step
		dials   int
		answers map[string][]string
	}{
		"downgrade to http": {header: "http://example.com/x", status: http.StatusFound, step: StepRedirect, dials: 1},
		"ip literal target": {header: "https://192.0.2.1/.well-known/stellar.toml", status: http.StatusFound, step: StepRedirect, dials: 1},
		"userinfo target":   {header: "https://user:pw@example.com/.well-known/stellar.toml", status: http.StatusFound, step: StepRedirect, dials: 1},
		"port target":       {header: "https://example.com:8443/.well-known/stellar.toml", status: http.StatusFound, step: StepRedirect, dials: 1},
		"non ascii target":  {header: "https://ex\u00e4mple.com/.well-known/stellar.toml", status: http.StatusFound, step: StepRedirect, dials: 1},
		"no location":       {header: "", status: http.StatusFound, step: StepRedirect, dials: 1},
		"relative target":   {header: "/elsewhere", status: http.StatusFound, step: StepOK, dials: 2},
		"private target": {header: "https://internal.example/.well-known/stellar.toml", status: http.StatusFound, step: StepGuard, dials: 1,
			answers: map[string][]string{"example.com": {"93.184.216.34"}, "internal.example": {"10.0.0.5"}}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mux := http.NewServeMux()
			mux.HandleFunc("/.well-known/stellar.toml", func(w http.ResponseWriter, _ *http.Request) {
				if tc.header == "" {
					w.WriteHeader(tc.status)
					return
				}
				w.Header().Set("Location", tc.header)
				w.WriteHeader(tc.status)
			})
			mux.HandleFunc("/elsewhere", func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, tomlBody(mainnetPass))
			})
			server := httptest.NewTLSServer(mux)
			defer server.Close()

			answers := tc.answers
			if answers == nil {
				answers = map[string][]string{"example.com": {"93.184.216.34"}}
			}
			c, _, rec := clientFor(t, server, answers)
			got := c.Fetch(context.Background(), "example.com")
			if got.FailedAt != tc.step {
				t.Fatalf("FailedAt = %v (%q), want %v", got.FailedAt, got.Detail, tc.step)
			}
			if rec.count() != tc.dials {
				t.Fatalf("dial addresses = %v, want %d dials", rec.addresses(), tc.dials)
			}
			if tc.step == StepGuard && got.HomeDomain != "example.com" {
				t.Fatalf("HomeDomain = %q, want the domain that was asked about", got.HomeDomain)
			}
			if tc.step == StepOK {
				if got.Hops != 1 || !got.Reachable() {
					t.Fatalf("Fetch = %+v, want one hop then success", got)
				}
			}
		})
	}
}

func TestFetchHopCap(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		redirects int
		step      Step
		hops      int
	}{
		"three hops allowed": {redirects: 3, step: StepOK, hops: 3},
		"four hops refused":  {redirects: 4, step: StepRedirect, hops: 3},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mux := http.NewServeMux()
			mux.HandleFunc("/.well-known/stellar.toml", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "/h1")
				w.WriteHeader(http.StatusFound)
			})
			for i := 1; i <= tc.redirects; i++ {
				from := fmt.Sprintf("/h%d", i)
				next := tomlBody(mainnetPass)
				if i < tc.redirects {
					next = fmt.Sprintf("/h%d", i+1)
				}
				mux.HandleFunc(from, func(w http.ResponseWriter, _ *http.Request) {
					if strings.HasPrefix(next, "/") {
						w.Header().Set("Location", next)
						w.WriteHeader(http.StatusFound)
						return
					}
					fmt.Fprint(w, next)
				})
			}
			mux.HandleFunc("/beyond", func(_ http.ResponseWriter, _ *http.Request) {
				t.Error("a request beyond the hop cap was sent")
			})
			server := httptest.NewTLSServer(mux)
			defer server.Close()
			c, _, _ := clientFor(t, server, map[string][]string{"example.com": {"93.184.216.34"}})
			got := c.Fetch(context.Background(), "example.com")
			if got.FailedAt != tc.step {
				t.Fatalf("FailedAt = %v (%q), want %v", got.FailedAt, got.Detail, tc.step)
			}
			if got.Hops != tc.hops {
				t.Fatalf("Hops = %d, want %d", got.Hops, tc.hops)
			}
		})
	}
}

func TestFetchSizeCapBoundary(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		total int
		step  Step
	}{
		"under the cap":    {100*1024 - 1, StepOK},
		"exactly the cap":  {100 * 1024, StepOK},
		"one over the cap": {100*1024 + 1, StepSize},
		"far over the cap": {400 * 1024, StepSize},
		"no body at all":   {0, StepNoKeys},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			body := ""
			if tc.total > 0 {
				body = tomlBody(strings.Repeat("p", tc.total-24))
				if len(body) != tc.total {
					t.Fatalf("fixture length = %d, want %d", len(body), tc.total)
				}
			}
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, body)
			}))
			defer server.Close()
			c, _, _ := clientFor(t, server, map[string][]string{"example.com": {"93.184.216.34"}})
			got := c.Fetch(context.Background(), "example.com")
			if got.FailedAt != tc.step {
				t.Fatalf("FailedAt = %v (%q), want %v", got.FailedAt, got.Detail, tc.step)
			}
			if got.Bytes > 100*1024 {
				t.Fatalf("Bytes = %d, want the cap applied before the body is kept", got.Bytes)
			}
		})
	}
}

func TestFetchTruncatedBodyIsTransport(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("the test server cannot be hijacked")
			return
		}
		conn, buf, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()
		fmt.Fprint(buf, "HTTP/1.1 200 OK\r\nContent-Length: 200000\r\n\r\n"+strings.Repeat("p", 1000))
		buf.Flush()
	}))
	defer server.Close()
	c, _, _ := clientFor(t, server, map[string][]string{"example.com": {"93.184.216.34"}})
	got := c.Fetch(context.Background(), "example.com")
	if got.FailedAt != StepTransport {
		t.Fatalf("FailedAt = %v (%q), want transport", got.FailedAt, got.Detail)
	}
}

func TestFetchStatusCodes(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		status int
		step   Step
		retry  bool
	}{
		"not found":       {http.StatusNotFound, StepStatus, false},
		"forbidden":       {http.StatusForbidden, StepStatus, false},
		"too many":        {http.StatusTooManyRequests, StepStatus, true},
		"server error":    {http.StatusInternalServerError, StepStatus, true},
		"no content":      {http.StatusNoContent, StepNoKeys, false},
		"ok with html ct": {http.StatusOK, StepOK, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var requests atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				// Content type is never a validity signal: SEP-1 only recommends
				// text/plain and a real domain answers 200 with an HTML page.
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if tc.status != http.StatusOK {
					w.WriteHeader(tc.status)
					return
				}
				fmt.Fprint(w, tomlBody(mainnetPass))
			}))
			defer server.Close()
			c, _, _ := clientFor(t, server, map[string][]string{"example.com": {"93.184.216.34"}})
			got := c.Fetch(context.Background(), "example.com")
			if got.FailedAt != tc.step {
				t.Fatalf("FailedAt = %v (%q), want %v", got.FailedAt, got.Detail, tc.step)
			}
			if tc.step == StepOK {
				if got.Status != http.StatusOK {
					t.Fatalf("Status = %d, want 200", got.Status)
				}
				return
			}
			if tc.step == StepNoKeys {
				return
			}
			if got.Status != tc.status {
				t.Fatalf("Status = %d, want %d", got.Status, tc.status)
			}
			want := int32(1)
			if tc.retry {
				want = 2
			}
			if requests.Load() != want {
				t.Fatalf("requests = %d, want %d for the retry rule", requests.Load(), want)
			}
		})
	}
}

func TestFetchSlowResponseHitsTheDeadline(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		fmt.Fprint(w, tomlBody(mainnetPass))
	}))
	defer server.Close()
	defer close(release)

	c, _, _ := clientFor(t, server, map[string][]string{"example.com": {"93.184.216.34"}})
	c.opts.Timeout = 50 * time.Millisecond
	start := time.Now()
	got := c.Fetch(context.Background(), "example.com")
	if got.FailedAt != StepTransport {
		t.Fatalf("FailedAt = %v (%q), want transport", got.FailedAt, got.Detail)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("elapsed = %v, want the deadline to bound the whole chain", elapsed)
	}
}

func TestFetchCancelHonouredBeforeAnyRequest(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("a cancelled fetch must not request anything")
	}))
	defer server.Close()
	c, res, rec := clientFor(t, server, map[string][]string{"example.com": {"93.184.216.34"}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := c.Fetch(ctx, "example.com")
	if got.FailedAt == StepOK {
		t.Fatalf("Fetch = %+v, want a failure after cancellation", got)
	}
	if rec.count() != 0 || res.lookups("example.com") != 0 {
		t.Fatalf("dials = %v lookups = %d, want none", rec.addresses(), res.lookups("example.com"))
	}
}

func TestFetchBadHostnameIsRefusedBeforeDNS(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"empty":        "",
		"space only":   " ",
		"ip literal":   "192.0.2.1",
		"single label": "localhost",
		"non ascii":    "ex\u00e4mple.com",
		"port":         "example.com:443",
		"control char": "exampl\te.com",
	}
	for name, domain := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				t.Error("a refused host must never be resolved or dialed")
			}))
			defer server.Close()
			c, res, rec := clientFor(t, server, map[string][]string{})
			got := c.Fetch(context.Background(), domain)
			if got.FailedAt != StepHostname {
				t.Fatalf("FailedAt = %v (%q), want hostname", got.FailedAt, got.Detail)
			}
			if rec.count() != 0 {
				t.Fatalf("dial addresses = %v, want no socket", rec.addresses())
			}
			if total := res.lookups(domain); total != 0 {
				t.Fatalf("lookups = %d, want the host refused before any lookup", total)
			}
		})
	}
}

func TestFetchResolvesNothingOnDNSError(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("an unresolvable host must not be dialed")
	}))
	defer server.Close()
	c, _, rec := clientFor(t, server, map[string][]string{})
	got := c.Fetch(context.Background(), "missing.example")
	if got.FailedAt != StepResolve {
		t.Fatalf("FailedAt = %v (%q), want resolve", got.FailedAt, got.Detail)
	}
	if rec.count() != 0 {
		t.Fatalf("dial addresses = %v, want none", rec.addresses())
	}
}

func TestFetchIgnoresProxyEnvironment(t *testing.T) {
	// t.Setenv touches the process environment, so this one is not parallel.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, tomlBody(mainnetPass))
	}))
	defer server.Close()
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("https_proxy", "http://127.0.0.1:1")

	c, _, rec := clientFor(t, server, map[string][]string{"example.com": {"93.184.216.34"}})
	got := c.Fetch(context.Background(), "example.com")
	if !got.Reachable() {
		t.Fatalf("Fetch = %+v, want the proxy environment ignored", got)
	}
	if len(rec.addresses()) != 1 || rec.addresses()[0] != "93.184.216.34:443" {
		t.Fatalf("dial addresses = %v, want the target and never a proxy", rec.addresses())
	}
}

func TestFetchDetailCarriesNoRemoteContent(t *testing.T) {
	t.Parallel()
	// A bidirectional override survives a header value; an escape byte does not,
	// because Go refuses to write or read one, so the ESC case never reaches this
	// code and would prove nothing. What matters is that the characters that do
	// arrive are neither sent onward nor echoed back.
	hostile := "https://example.com/\u202e"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", hostile)
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	c, _, rec := clientFor(t, server, map[string][]string{"example.com": {"93.184.216.34"}})
	got := c.Fetch(context.Background(), "example.com")
	if got.FailedAt != StepRedirect {
		t.Fatalf("FailedAt = %v (%q), want redirect", got.FailedAt, got.Detail)
	}
	if strings.ContainsAny(got.Detail, "\x1b\u202e") || got.Detail == hostile {
		t.Fatalf("Detail = %q, want no escape sequences and no echoed remote content", got.Detail)
	}
	if rec.count() != 1 {
		t.Fatalf("dial addresses = %v, want the refused hop never requested", rec.addresses())
	}
}

func TestGatedDialRefusesNonTCPNetwork(t *testing.T) {
	t.Parallel()
	c, err := NewClient(Options{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.resolver = newResolver(map[string][]string{"example.com": {"93.184.216.34"}})
	var dialed atomic.Int32
	c.dial = func(context.Context, string, string) (net.Conn, error) {
		dialed.Add(1)
		return nil, errors.New("must not dial")
	}
	c.tlsConfig.RootCAs = x509.NewCertPool()
	if _, err := c.gatedDial(context.Background(), "unix", "/tmp/socket"); err == nil {
		t.Fatal("gatedDial accepted a unix network")
	}
	if dialed.Load() != 0 {
		t.Fatalf("dialed = %d, want the network check first", dialed.Load())
	}
	if _, err := c.gatedDial(context.Background(), "tcp", "example.com:443"); err != nil {
		t.Logf("dial error: %v", err)
	}
	if got := dialed.Load(); got != 1 {
		t.Fatalf("dialed = %d, want the validated address handed to the seam", got)
	}
}

func TestNewClientRejectsNegativeOptions(t *testing.T) {
	t.Parallel()
	for name, opts := range map[string]Options{
		"negative timeout":  {Timeout: -1},
		"negative body cap": {MaxBody: -1},
		"negative hop cap":  {MaxHops: -1},
		"negative retries":  {MaxRetries: -1},
		"negative backoff":  {Backoff: -1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewClient(opts); err == nil {
				t.Fatalf("NewClient(%+v) = nil error, want a rejection", opts)
			}
		})
	}
	c, err := NewClient(Options{})
	if err != nil {
		t.Fatalf("NewClient zero options: %v", err)
	}
	if c.opts.MaxBody != 100*1024 || c.opts.MaxHops != 3 || c.opts.Timeout != defaultTimeout {
		t.Fatalf("defaults = %+v, want the SEP-1 cap and three hops", c.opts)
	}
	if c.resolver == nil || c.dial == nil || c.tlsConfig == nil {
		t.Fatal("NewClient left a production dependency unset")
	}
	if c.tlsConfig.InsecureSkipVerify {
		t.Fatal("the client skips certificate verification")
	}
	if c.tlsConfig.MinVersion < tlsVersionFloor {
		t.Fatalf("MinVersion = %v, want TLS 1.2 or above", c.tlsConfig.MinVersion)
	}
}
