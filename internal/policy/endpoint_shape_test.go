package policy

import (
	"errors"
	"strings"
	"testing"
)

func TestLoadEndpointShape(t *testing.T) {
	t.Parallel()
	const header = "version = 1\nnetwork = \"testnet\"\n"
	const account = "\n[[account]]\nid = \"" + accountOne + "\"\n"

	for _, tt := range []struct {
		name     string
		endpoint string
		reject   bool
	}{
		{"bare host", "https://horizon.example.org", false},
		{"trailing slash", "https://horizon.example.org/", false},
		{"explicit port", "https://horizon.example.org:443", false},
		{"loopback http", "http://127.0.0.1:8000", false},
		{"path", "https://horizon.example.org/horizon", true},
		{"nested path", "https://horizon.example.org/horizon/accounts", true},
		{"encoded slash", "https://horizon.example.org/%2F", true},
		{"port too high", "https://horizon.example.org:99999", true},
		{"port zero", "https://horizon.example.org:0", true},
		{"trailing colon", "https://horizon.example.org:", true},
		{"query", "https://horizon.example.org?cursor=1", true},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := header + "endpoints = [\"" + tt.endpoint + "\"]" + account
			cfg, err := Load(strings.NewReader(body))
			if !tt.reject {
				if err != nil {
					t.Fatalf("Load(%q): %v", tt.endpoint, err)
				}
				if len(cfg.Endpoints) != 1 || cfg.Endpoints[0] != tt.endpoint {
					t.Fatalf("endpoints = %v", cfg.Endpoints)
				}
				return
			}
			if !errors.Is(err, ErrEndpoint) {
				t.Fatalf("Load(%q) error = %v, want ErrEndpoint", tt.endpoint, err)
			}
			if !strings.Contains(err.Error(), "endpoints[0]") {
				t.Fatalf("Load(%q) error = %v, want an endpoints[0] field path", tt.endpoint, err)
			}
		})
	}
}
