package rules

import (
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
)

// hashXKeys are deliberately distinguishable so the test can tell which signer
// a finding came from, not just that two findings exist.
const (
	firstHashXKey  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	secondHashXKey = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// TestRegistryEvaluationOrder pins the pre-sort sequence Evaluate hands to the
// report layer. Findings are appended in registry order and sorted only when a
// report is prepared, so the order below is a contract nothing else names: a
// registry reorder or a rule that sorts its own output changes what the
// renderers receive. The context fires three rules, and sig-hashx fires twice,
// so both the rule sequence and the source order inside one rule are covered.
func TestRegistryEvaluationOrder(t *testing.T) {
	t.Parallel()
	ctx := Context{
		Account: policy.Account{
			ID: testAccount,
			ExpectedThresholds: &policy.ExpectedThresholds{
				Low: intPointer(2),
			},
		},
		Snapshot: model.AccountSnapshot{
			ID: testAccount,
			Thresholds: model.Thresholds{
				Low:    1,
				Medium: 2,
				High:   2,
			},
			Signers: []model.Signer{
				signerEntry(firstHashXKey, model.SignerHashX, 1),
				signerEntry(secondHashXKey, model.SignerHashX, 1),
			},
			Flags: model.Flags{ClawbackEnabled: true},
		},
	}

	findings := Evaluate(ctx)
	want := []string{
		"sig-hashx",
		"sig-hashx",
		"thr-drift",
		"flag-undeclared-clawback",
	}
	if len(findings) != len(want) {
		t.Fatalf("got %d findings %+v, want %d", len(findings), findings, len(want))
	}
	for i, id := range want {
		if findings[i].RuleID != id {
			t.Errorf("finding %d is %q, want %q", i, findings[i].RuleID, id)
		}
	}

	// sig-hashx walks the snapshot signers in order. If the rule ever sorted
	// its findings, the two keys would swap here.
	if !strings.Contains(findings[0].Evidence, firstHashXKey[:6]) {
		t.Errorf("first hash(x) finding does not name the first signer: %q", findings[0].Evidence)
	}
	if !strings.Contains(findings[1].Evidence, secondHashXKey[:6]) {
		t.Errorf("second hash(x) finding does not name the second signer: %q", findings[1].Evidence)
	}
}
