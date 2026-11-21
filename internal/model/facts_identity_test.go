package model

import (
	"encoding/json"
	"errors"
	"testing"
)

// TestIdentityFactsValidate pins the three states a real read can produce and
// the two contradictions it cannot: a verdict on whether the document named
// this network when the document named no network. Values are never printed.
func TestIdentityFactsValidate(t *testing.T) {
	t.Parallel()
	passphrase := "Public Global Stellar Network ; September 2015"
	matches := true
	misses := false
	tests := []struct {
		name  string
		facts IdentityFacts
		want  error
	}{
		{name: "no document read", facts: IdentityFacts{}},
		{name: "passphrase read but not compared", facts: IdentityFacts{NetworkPassphrase: &passphrase}},
		{name: "passphrase and agreement", facts: IdentityFacts{NetworkPassphrase: &passphrase, NetworkMatches: &matches}},
		{name: "passphrase and disagreement", facts: IdentityFacts{NetworkPassphrase: &passphrase, NetworkMatches: &misses}},
		{name: "agreement without a passphrase", facts: IdentityFacts{NetworkMatches: &matches}, want: ErrIdentityNetworkPair},
		{name: "disagreement without a passphrase", facts: IdentityFacts{NetworkMatches: &misses}, want: ErrIdentityNetworkPair},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.facts.Validate(); !errors.Is(err, tt.want) {
				t.Fatalf("Validate() = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestAccountFactsValidate proves the block guard covers an absent identity
// block as nothing to refuse and a present one as the identity guard does.
func TestAccountFactsValidate(t *testing.T) {
	t.Parallel()
	matches := true
	tests := []struct {
		name  string
		facts *AccountFacts
		want  error
	}{
		{name: "absent block", facts: nil},
		{name: "no identity read", facts: &AccountFacts{}},
		{name: "identity consistent", facts: &AccountFacts{Identity: &IdentityFacts{}}},
		{name: "identity contradicts", facts: &AccountFacts{Identity: &IdentityFacts{NetworkMatches: &matches}}, want: ErrIdentityNetworkPair},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.facts.Validate(); !errors.Is(err, tt.want) {
				t.Fatalf("Validate() = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestIdentityFactsJSONRoundTrip proves the typed pair survives the report's
// wire format and still validates, so the guard reads the same state a consumer
// does.
func TestIdentityFactsJSONRoundTrip(t *testing.T) {
	t.Parallel()
	passphrase := "Public Global Stellar Network ; September 2015"
	matches := false
	data, err := json.Marshal(IdentityFacts{NetworkPassphrase: &passphrase, NetworkMatches: &matches})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded IdentityFacts
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("Validate() after round trip = %v, want nil", err)
	}
	if decoded.NetworkPassphrase == nil || *decoded.NetworkPassphrase != passphrase {
		t.Fatal("passphrase did not survive the round trip")
	}
	if decoded.NetworkMatches == nil || *decoded.NetworkMatches {
		t.Fatal("match verdict did not survive the round trip")
	}
}
