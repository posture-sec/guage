package sep1

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestRedirectRefusals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		loc  string
	}{
		{"downgrade to http", "http://example.com/x"},
		{"userinfo in target", "https://user:pw@example.com/x"},
		{"non-default port", "https://example.com:8443/x"},
		{"non-ascii host", "https://ex\u00e4mple.com/x"},
		{"trailing-dot host", "https://example.com./x"},
		{"bracketed ip literal", "https://[::1]/x"},
		{"scheme-relative to trailing-dot host", "//example.com./x"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var mu sync.Mutex
			paths := map[string]int{}
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				paths[r.URL.Path]++
				mu.Unlock()
				if r.URL.Path == "/.well-known/stellar.toml" {
					w.Header().Set("Location", tc.loc)
					w.WriteHeader(http.StatusFound)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			c, _, rec := clientFor(t, server, map[string][]string{"example.com": {"93.184.216.34"}})
			got := c.Fetch(t.Context(), "example.com")

			if got.FailedAt != StepRedirect {
				t.Fatalf("FailedAt = %v (%q), want redirect", got.FailedAt, got.Detail)
			}
			if got.Detail != "the document moved somewhere Gauge will not follow" {
				t.Fatalf("Detail = %q, want no echoed Location bytes", got.Detail)
			}
			if strings.Contains(got.Detail, tc.loc) {
				t.Fatalf("Detail = %q contains the refused Location %q", got.Detail, tc.loc)
			}
			if rec.count() != 1 {
				t.Fatalf("dials = %d, want exactly the initial request and no hop", rec.count())
			}
			mu.Lock()
			refusedHits := paths["/x"]
			mu.Unlock()
			if refusedHits != 0 {
				t.Fatalf("refused target path /x received %d requests, want none", refusedHits)
			}
		})
	}

	// A relative redirect to the same host succeeds, proving the harness can serve two hops.
	t.Run("relative redirect succeeds", func(t *testing.T) {
		t.Parallel()

		var mu sync.Mutex
		paths := map[string]int{}
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			paths[r.URL.Path]++
			mu.Unlock()
			switch r.URL.Path {
			case "/.well-known/stellar.toml":
				w.Header().Set("Location", "/elsewhere")
				w.WriteHeader(http.StatusFound)
			case "/elsewhere":
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Write([]byte("NETWORK_PASSPHRASE = \"Public Global Stellar Network ; September 2015\"\n"))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		c, _, rec := clientFor(t, server, map[string][]string{"example.com": {"93.184.216.34"}})
		got := c.Fetch(t.Context(), "example.com")
		if !got.Reachable() || got.FailedAt != StepOK {
			t.Fatalf("Fetch = %+v, want reachable after one hop", got)
		}
		if got.Hops != 1 {
			t.Fatalf("Hops = %d, want 1", got.Hops)
		}
		if rec.count() != 2 {
			t.Fatalf("dials = %d, want two hops dialed", rec.count())
		}
		mu.Lock()
		hits := paths["/elsewhere"]
		mu.Unlock()
		if hits != 1 {
			t.Fatalf("/elsewhere received %d requests, want 1", hits)
		}
	})
}
