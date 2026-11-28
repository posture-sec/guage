package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMultiSigCoordinatorEntryPrecedenceAndAliases(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]string
		wantKey     string
		wantRaw     string
		wantPresent bool
	}{
		{
			name:        "nil data",
			data:        nil,
			wantPresent: false,
		},
		{
			name:        "unrelated keys only",
			data:        map[string]string{"config.memo_required": "MQ==", "other": "val"},
			wantPresent: false,
		},
		{
			name:        "leading separator without prefix rejected",
			data:        map[string]string{".multisig.coordinator": "djE="},
			wantPresent: false,
		},
		{
			name:        "bare multisig.coordinator rejected",
			data:        map[string]string{"multisig.coordinator": "djE="},
			wantPresent: false,
		},
		{
			name: "canonical key wins over aliases",
			data: map[string]string{
				"app.multisig.coordinator":    "YWxpYXM=",
				"config.multisig.coordinator": "djEubXVsdGlzaWcuc2F0b3NoaXBheS5pbw==",
			},
			wantKey:     "config.multisig.coordinator",
			wantRaw:     "djEubXVsdGlzaWcuc2F0b3NoaXBheS5pbw==",
			wantPresent: true,
		},
		{
			name: "aliases pick lexicographically first deterministic key",
			data: map[string]string{
				"z.multisig.coordinator":      "eg==",
				"app:multisig:coordinator":    "YQ==",
				"lobstr.multisig.coordinator": "bA==",
			},
			wantKey:     "app:multisig:coordinator",
			wantRaw:     "YQ==",
			wantPresent: true,
		},
		{
			name:        "underscore separator accepted",
			data:        map[string]string{"vendor_multisig_coordinator": "dmVuZG9y"},
			wantKey:     "vendor_multisig_coordinator",
			wantRaw:     "dmVuZG9y",
			wantPresent: true,
		},
		{
			name:        "slash separator accepted",
			data:        map[string]string{"vendor/multisig/coordinator": "dmVuZG9y"},
			wantKey:     "vendor/multisig/coordinator",
			wantRaw:     "dmVuZG9y",
			wantPresent: true,
		},
		{
			name:        "empty value still counts as present",
			data:        map[string]string{"config.multisig.coordinator": ""},
			wantKey:     "config.multisig.coordinator",
			wantRaw:     "",
			wantPresent: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snap := AccountSnapshot{Data: tc.data}
			gotKey, gotRaw, gotPresent := snap.MultiSigCoordinatorEntry()
			if gotKey != tc.wantKey || gotRaw != tc.wantRaw || gotPresent != tc.wantPresent {
				t.Fatalf("MultiSigCoordinatorEntry() = (%q, %q, %v), want (%q, %q, %v)",
					gotKey, gotRaw, gotPresent, tc.wantKey, tc.wantRaw, tc.wantPresent)
			}
			raw, present := snap.MultiSigCoordinator()
			if raw != tc.wantRaw || present != tc.wantPresent {
				t.Fatalf("MultiSigCoordinator() = (%q, %v), want (%q, %v)",
					raw, present, tc.wantRaw, tc.wantPresent)
			}
		})
	}
}

func TestCoordinatorFactsJSON(t *testing.T) {
	nilFacts := AccountFacts{}
	raw, err := json.Marshal(nilFacts)
	if err != nil {
		t.Fatalf("marshal nilFacts: %v", err)
	}
	if strings.Contains(string(raw), "coordinator") {
		t.Fatalf("nil Coordinator omitted expected, got %s", raw)
	}

	emptyCoord := ""
	decl := DeclaredFacts{
		Role:        RoleTreasury,
		Coordinator: &emptyCoord,
	}
	rawDecl, err := json.Marshal(decl)
	if err != nil {
		t.Fatalf("marshal DeclaredFacts: %v", err)
	}
	if !strings.Contains(string(rawDecl), `"coordinator":""`) {
		t.Fatalf("non-nil empty DeclaredFacts.Coordinator should marshal as empty string, got %s", rawDecl)
	}

	obs := AccountFacts{
		Declared: &decl,
		Coordinator: &CoordinatorFact{
			DataKey: "config.multisig.coordinator",
			Domain:  "v1.multisig.satoshipay.io",
			Valid:   true,
		},
	}
	rawObs, err := json.Marshal(obs)
	if err != nil {
		t.Fatalf("marshal obs: %v", err)
	}
	wantSub := `"coordinator":{"data_key":"config.multisig.coordinator","domain":"v1.multisig.satoshipay.io","valid":true}`
	if !strings.Contains(string(rawObs), wantSub) {
		t.Fatalf("marshaled AccountFacts missing %s in %s", wantSub, rawObs)
	}

	invalidObs := CoordinatorFact{
		DataKey: "config.multisig.coordinator",
		Valid:   false,
	}
	rawInvalid, err := json.Marshal(invalidObs)
	if err != nil {
		t.Fatalf("marshal invalidObs: %v", err)
	}
	if string(rawInvalid) != `{"data_key":"config.multisig.coordinator","valid":false}` {
		t.Fatalf("invalid CoordinatorFact should omit empty domain, got %s", rawInvalid)
	}
}
