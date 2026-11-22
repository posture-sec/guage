package sep1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHomeDomain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		host  string
		ok    bool
	}{
		{" example.com ", "example.com", true},
		{"EXAMPLE.COM", "example.com", true},
		{" ", "", false},
		{"", "", false},
		{"\texample.com\t", "example.com", true},
		{"example.com.", "", false},
		{"localhost", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got, ok := HomeDomain(tc.input)
			if got != tc.host || ok != tc.ok {
				t.Fatalf("HomeDomain(%q) = (%q, %v), want (%q, %v)", tc.input, got, ok, tc.host, tc.ok)
			}
		})
	}
}

func TestFetchRefusesAbsent(t *testing.T) {
	t.Parallel()

	refused := []string{"example.com.", "localhost", " example.com. "}
	for _, domain := range refused {
		t.Run(domain, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				t.Error("a refused host must never be contacted")
			}))
			defer server.Close()
			c, _, rec := clientFor(t, server, map[string][]string{})
			got := c.Fetch(context.Background(), domain)
			if got.FailedAt != StepHostname {
				t.Fatalf("FailedAt = %v (%q), want hostname", got.FailedAt, got.Detail)
			}
			if rec.count() != 0 {
				t.Fatalf("dials = %d, want none for a refused host", rec.count())
			}
		})
	}
}
