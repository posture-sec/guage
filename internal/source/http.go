package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimeout       = 10 * time.Second
	defaultMaxBody       = 1 << 20
	defaultMaxRetries    = 3
	defaultBackoff       = 200 * time.Millisecond
	defaultMaxConcurrent = 8
	maxRetryAfter        = 5 * time.Second
	maxBackoff           = 5 * time.Second
)

// Options configures a Horizon client. The zero value of every optional field
// selects the default; negative values are rejected.
type Options struct {
	Endpoint      string
	UserAgent     string
	Timeout       time.Duration
	MaxBody       int64
	MaxRetries    int
	Backoff       time.Duration
	MaxConcurrent int
}

// Horizon reads account state from a Horizon endpoint over one shared client.
type Horizon struct {
	endpoint   string
	userAgent  string
	timeout    time.Duration
	maxBody    int64
	maxRetries int
	backoff    time.Duration
	sem        chan struct{}
	client     *http.Client
	sleep      func(context.Context, time.Duration) error
}

// NewHorizon validates the endpoint and returns a client for it. Plain HTTP is
// accepted only for loopback addresses, which keeps tests offline without
// widening what a policy may name.
func NewHorizon(opts Options) (*Horizon, error) {
	endpoint, err := normalizeEndpoint(opts.Endpoint)
	if err != nil {
		return nil, err
	}
	if opts.Timeout < 0 || opts.MaxBody < 0 || opts.MaxRetries < 0 || opts.Backoff < 0 || opts.MaxConcurrent < 0 {
		return nil, fmt.Errorf("%w: durations and counts cannot be negative", ErrOptions)
	}

	h := &Horizon{
		endpoint:   endpoint,
		userAgent:  opts.UserAgent,
		timeout:    opts.Timeout,
		maxBody:    opts.MaxBody,
		maxRetries: opts.MaxRetries,
		backoff:    opts.Backoff,
		sleep:      sleepContext,
	}
	if h.timeout == 0 {
		h.timeout = defaultTimeout
	}
	if h.maxBody == 0 {
		h.maxBody = defaultMaxBody
	}
	if h.maxRetries == 0 {
		h.maxRetries = defaultMaxRetries
	}
	if h.backoff == 0 {
		h.backoff = defaultBackoff
	}
	maxConcurrent := opts.MaxConcurrent
	if maxConcurrent == 0 {
		maxConcurrent = defaultMaxConcurrent
	}
	h.sem = make(chan struct{}, maxConcurrent)
	h.client = &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return h, nil
}

// attempt reports whether a failed request may be retried and how long the
// server asked the client to wait first.
type attempt struct {
	retry bool
	wait  time.Duration
}

// get performs a bounded number of attempts against one path and decodes the
// first successful body into out. Bodies of failed responses are never read.
func (h *Horizon) get(ctx context.Context, path string, out any) error {
	var last error
	var wait time.Duration
	for i := 0; i <= h.maxRetries; i++ {
		if i > 0 {
			delay := wait
			if delay <= 0 {
				delay = backoffFor(h.backoff, i)
			}
			if err := h.sleep(ctx, delay); err != nil {
				return fmt.Errorf("source: %s: %w", path, err)
			}
		}
		result, err := h.once(ctx, path, out)
		if err == nil {
			return nil
		}
		last = err
		if !result.retry || ctx.Err() != nil {
			return err
		}
		wait = result.wait
	}
	return fmt.Errorf("source: %s: gave up after %d attempts: %w", path, h.maxRetries+1, last)
}

func (h *Horizon) once(ctx context.Context, path string, out any) (attempt, error) {
	select {
	case h.sem <- struct{}{}:
	case <-ctx.Done():
		return attempt{}, fmt.Errorf("source: %s: %w", path, ctx.Err())
	}
	defer func() { <-h.sem }()

	reqCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, h.endpoint+path, nil)
	if err != nil {
		return attempt{}, fmt.Errorf("source: %s: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")
	if h.userAgent != "" {
		req.Header.Set("User-Agent", h.userAgent)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return attempt{}, fmt.Errorf("source: %s: %w", path, ctx.Err())
		}
		return attempt{retry: true}, fmt.Errorf("source: %s: %w", path, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode <= 299:
	case resp.StatusCode == http.StatusNotFound:
		return attempt{}, fmt.Errorf("source: %s: %w", path, ErrNotFound)
	case resp.StatusCode == http.StatusTooManyRequests:
		return attempt{retry: true, wait: retryAfter(resp.Header)}, fmt.Errorf("source: %s: %w: %d", path, ErrStatus, resp.StatusCode)
	case resp.StatusCode >= 500 && resp.StatusCode <= 599:
		return attempt{retry: true}, fmt.Errorf("source: %s: %w: %d", path, ErrStatus, resp.StatusCode)
	default:
		return attempt{}, fmt.Errorf("source: %s: %w: %d", path, ErrStatus, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, h.maxBody+1))
	if err != nil {
		if reqCtx.Err() != nil {
			return attempt{retry: true}, fmt.Errorf("source: %s: %w", path, reqCtx.Err())
		}
		return attempt{retry: true}, fmt.Errorf("source: %s: reading the body: %w", path, err)
	}
	if int64(len(body)) > h.maxBody {
		return attempt{}, fmt.Errorf("source: %s: %w", path, ErrTooLarge)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return attempt{}, fmt.Errorf("source: %s: %w", path, ErrDecode)
	}
	return attempt{}, nil
}

// retryAfter reads the integer-seconds form, which is the only form Gauge
// honours. A date value falls back to the client's own backoff, and an
// excessive delay is capped so a hostile header cannot stall a run.
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

func normalizeEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("%w: the endpoint is empty", ErrEndpoint)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: the endpoint is not a URL", ErrEndpoint)
	}
	if u.User != nil {
		return "", fmt.Errorf("%w: the endpoint carries credentials", ErrEndpoint)
	}
	if u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", fmt.Errorf("%w: the endpoint is a base URL, not a link", ErrEndpoint)
	}
	if strings.TrimRight(u.Path, "/") != "" {
		return "", fmt.Errorf("%w: the endpoint is a base URL, not a path", ErrEndpoint)
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("%w: the endpoint has no host", ErrEndpoint)
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
	case "http":
		if !isLoopback(host) {
			return "", fmt.Errorf("%w: plain HTTP is allowed only for loopback", ErrEndpoint)
		}
	default:
		return "", fmt.Errorf("%w: the scheme must be https", ErrEndpoint)
	}
	return strings.ToLower(u.Scheme) + "://" + u.Host, nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
