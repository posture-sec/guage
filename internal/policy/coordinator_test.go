package policy

import (
	"errors"
	"strings"
	"testing"
)

func TestLoadCoordinatorField(t *testing.T) {
	t.Parallel()

	baseHeader := "version = 1\nnetwork = \"testnet\"\nendpoints = [\"https://horizon-testnet.stellar.org\"]\n\n[[account]]\nid = \"GAUA7XL5K54CC2DDGP77FJ2YBHRJLT36CPZDXWPM6MP7MANOGG77PNJU\"\nrole = \"treasury\"\n"

	t.Run("omitted stays nil", func(t *testing.T) {
		t.Parallel()
		cfg, err := Load(strings.NewReader(baseHeader))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Accounts[0].Coordinator != nil {
			t.Fatalf("Coordinator = %v, want nil", *cfg.Accounts[0].Coordinator)
		}
	})

	t.Run("empty string declares no coordinator", func(t *testing.T) {
		t.Parallel()
		cfg, err := Load(strings.NewReader(baseHeader + "coordinator = \"\"\n"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Accounts[0].Coordinator == nil || *cfg.Accounts[0].Coordinator != "" {
			t.Fatalf("Coordinator = %v, want non-nil empty string", cfg.Accounts[0].Coordinator)
		}
	})

	t.Run("valid domain is normalized to lowercase bare host", func(t *testing.T) {
		t.Parallel()
		cfg, err := Load(strings.NewReader(baseHeader + "coordinator = \"V1.Multisig.SatoshiPay.IO.\"\n"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Accounts[0].Coordinator == nil || *cfg.Accounts[0].Coordinator != "v1.multisig.satoshipay.io" {
			t.Fatalf("Coordinator = %v, want v1.multisig.satoshipay.io", cfg.Accounts[0].Coordinator)
		}
	})

	invalidCases := []struct {
		name  string
		value string
	}{
		{name: "https url", value: "https://v1.multisig.satoshipay.io"},
		{name: "path suffix", value: "v1.multisig.satoshipay.io/api"},
		{name: "port", value: "v1.multisig.satoshipay.io:443"},
		{name: "ipv4", value: "127.0.0.1"},
		{name: "single label", value: "localhost"},
		{name: "leading space", value: " v1.multisig.satoshipay.io"},
		{name: "empty label", value: "v1..satoshipay.io"},
		{name: "non-ascii", value: "v1.ex\u00e4mple.io"},
	}

	for _, tc := range invalidCases {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			t.Parallel()
			cfgText := baseHeader + "coordinator = \"" + tc.value + "\"\n"
			_, err := Load(strings.NewReader(cfgText))
			if !errors.Is(err, ErrCoordinator) {
				t.Fatalf("error = %v, want ErrCoordinator", err)
			}
			var perr *Error
			if !errors.As(err, &perr) || perr.Detail != "account[0].coordinator" {
				t.Fatalf("Detail = %q, want account[0].coordinator", err)
			}
			if strings.Contains(err.Error(), tc.value) {
				t.Fatalf("error leaked raw policy value %q: %v", tc.value, err)
			}
		})
	}
}
