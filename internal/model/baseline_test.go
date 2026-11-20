package model_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

func TestBaselineSerialization(t *testing.T) {
	b := model.Baseline{
		Version:     1,
		Banner:      model.BaselineBanner,
		ToolVersion: "v0.1.0",
		Network:     "Test SDF Network ; September 2015",
		Ledger:      1234567,
		Accounts: []model.AccountSnapshot{
			{
				ID:       "GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ",
				Network:  "Test SDF Network ; September 2015",
				Endpoint: "https://horizon-testnet.stellar.org",
				Ledger:   1234567,
				Sequence: "100",
				Thresholds: model.Thresholds{
					Low:    1,
					Medium: 2,
					High:   2,
				},
				Signers: []model.Signer{
					{
						Key:    "GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ",
						Type:   model.SignerEd25519,
						Weight: 2,
					},
				},
				Flags: model.Flags{
					AuthRequired: true,
				},
				HomeDomain: "example.com",
			},
		},
	}

	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	if !strings.Contains(string(data), model.BaselineBanner) {
		t.Fatalf("expected serialized baseline to contain review banner, got: %s", string(data))
	}

	var parsed model.Baseline
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if parsed.Version != 1 {
		t.Errorf("expected version 1, got %d", parsed.Version)
	}
	if parsed.Banner != model.BaselineBanner {
		t.Errorf("expected banner %q, got %q", model.BaselineBanner, parsed.Banner)
	}
	if len(parsed.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(parsed.Accounts))
	}
	if parsed.Accounts[0].ID != b.Accounts[0].ID {
		t.Errorf("expected account ID %q, got %q", b.Accounts[0].ID, parsed.Accounts[0].ID)
	}
}
