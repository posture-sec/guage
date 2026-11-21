package model

import (
	"encoding/json"
	"sort"
	"testing"
)

func TestFindingJSONKeys(t *testing.T) {
	t.Parallel()
	f := Finding{
		RuleID:      "sig-single-key-high",
		Severity:    SevHigh,
		Account:     "GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ",
		Message:     "One signing key controls the high threshold.",
		Evidence:    "minimum signers: 1",
		Fingerprint: "draft-fingerprint",
		Accepted:    false,
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"accepted", "account", "evidence", "fingerprint", "message", "rule_id", "severity"}
	if !equalStringSlices(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	if string(raw["severity"]) != `"high"` {
		t.Fatalf("severity = %s, want \"high\"", raw["severity"])
	}
	if _, ok := raw["accepted"]; !ok {
		t.Fatal("accepted key missing")
	}
	if _, ok := raw["fingerprint"]; !ok {
		t.Fatal("fingerprint key missing")
	}
	var accepted bool
	if err := json.Unmarshal(raw["accepted"], &accepted); err != nil || accepted {
		t.Fatalf("accepted = %v, want false", accepted)
	}
	var fingerprint string
	if err := json.Unmarshal(raw["fingerprint"], &fingerprint); err != nil || fingerprint != "draft-fingerprint" {
		t.Fatalf("fingerprint = %v, want \"draft-fingerprint\"", fingerprint)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
