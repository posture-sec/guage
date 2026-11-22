package policy

import (
	"errors"
	"strings"
	"testing"
)

func TestLoadRejectsEmptyPolicy(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		body    string
		wantErr error
	}{
		{
			name:    "no account table",
			body:    "version = 1\nnetwork = \"testnet\"\n",
			wantErr: ErrNoAccounts,
		},
		{
			name: "one account table",
			body: "version = 1\nnetwork = \"testnet\"\n" +
				"endpoints = [\"https://horizon-testnet.stellar.org\"]\n" +
				"[[account]]\nid = \"" + accountOne + "\"\n",
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := Load(strings.NewReader(tt.body))
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Load: %v", err)
				}
				if len(cfg.Accounts) != 1 {
					t.Fatalf("accounts = %d, want 1", len(cfg.Accounts))
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if cfg != nil {
				t.Fatalf("a rejected policy returned a config: %+v", cfg)
			}
		})
	}
}
