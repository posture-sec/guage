package rules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
)

func number(v int) *int { return &v }

// repeat builds count signers of one type and weight, for the cases where the
// number of entries is the point.
func repeat(typ model.SignerType, weight, count int) []model.Signer {
	signers := make([]model.Signer, count)
	for i := range signers {
		signers[i] = model.Signer{Key: keyAtPosition(i), Type: typ, Weight: weight}
	}
	return signers
}

// keyAtPosition keeps every generated signer distinct, so a rule that indexes by
// key cannot collapse an enumeration of entries into one.
func keyAtPosition(i int) string {
	return fmt.Sprintf("SIGNER-%03d", i)
}

// TestSigThresholdUnreachable pins the tier arithmetic the ledger allows but does
// not enforce: SetOptions validates ranges only, so an unreachable tier is stored
// state and Gauge is the one telling (ARCHITECTURE.md 6.3).
func TestSigThresholdUnreachable(t *testing.T) {
	t.Parallel()
	full := repeat(model.SignerEd25519, 1, 21)
	for _, tt := range []struct {
		name       string
		snapshot   model.AccountSnapshot
		wantCount  int
		wantInEvid string
	}{
		{
			name:      "every tier is covered by the weights present",
			snapshot:  snapshotWith(model.Thresholds{Low: 1, Medium: 2, High: 3}, signerEntry(testAccount, model.SignerEd25519, 3), signerEntry(secondAccount, model.SignerEd25519, 2)),
			wantCount: 0,
		},
		{
			name:       "a high tier nothing can reach",
			snapshot:   snapshotWith(model.Thresholds{Low: 1, Medium: 1, High: 10}, signerEntry(testAccount, model.SignerEd25519, 3)),
			wantCount:  1,
			wantInEvid: "high threshold 10",
		},
		{
			name:       "one unreachable tier is listed alone",
			snapshot:   snapshotWith(model.Thresholds{Low: 0, Medium: 1, High: 10}, signerEntry(testAccount, model.SignerEd25519, 5), signerEntry(secondAccount, model.SignerHashX, 4)),
			wantCount:  1,
			wantInEvid: "high threshold 10",
		},
		{
			name:       "two unreachable tiers name both",
			snapshot:   snapshotWith(model.Thresholds{Low: 1, Medium: 10, High: 11}, signerEntry(testAccount, model.SignerEd25519, 5), signerEntry(secondAccount, model.SignerHashX, 4)),
			wantCount:  1,
			wantInEvid: "medium threshold 10 and high threshold 11",
		},
		{
			name:       "no signers makes every nonzero tier unreachable",
			snapshot:   snapshotWith(model.Thresholds{Low: 1, Medium: 2, High: 3}),
			wantCount:  1,
			wantInEvid: "the account carries no signers at all",
		},
		{
			name:       "a weight-0 signer contributes nothing",
			snapshot:   snapshotWith(model.Thresholds{Low: 1, Medium: 1, High: 1}, signerEntry(testAccount, model.SignerEd25519, 0)),
			wantCount:  1,
			wantInEvid: "low threshold 1",
		},
		{
			name:       "a pre-authorized hash is not standing control",
			snapshot:   snapshotWith(model.Thresholds{Low: 1, Medium: 1, High: 1}, signerEntry(testAccount, model.SignerPreAuthTx, 255)),
			wantCount:  1,
			wantInEvid: "pre_auth_tx",
		},
		{
			// MAX_SIGNERS is 20 and the master key is a 21st weight, but an
			// envelope carries DecoratedSignature signatures<20>, so the highest
			// tier a full account can present is 20 when every weight is 1.
			name:       "a full account cannot present its 21st signature",
			snapshot:   snapshotWith(model.Thresholds{Low: 1, Medium: 1, High: 21}, full...),
			wantCount:  1,
			wantInEvid: "high threshold 21",
		},
		{
			name:      "a full account at the cap it can present",
			snapshot:  snapshotWith(model.Thresholds{Low: 1, Medium: 1, High: 20}, full...),
			wantCount: 0,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sigThresholdUnreachable(contextWith(policy.Account{ID: testAccount}, tt.snapshot))
			if len(got) != tt.wantCount {
				t.Fatalf("got %d findings, want %d: %+v", len(got), tt.wantCount, got)
			}
			if tt.wantCount == 0 {
				return
			}
			if got[0].RuleID != "sig-threshold-unreachable" || got[0].Severity != model.SevHigh {
				t.Errorf("finding = %s/%s, want sig-threshold-unreachable/high", got[0].RuleID, got[0].Severity)
			}
			if tt.wantInEvid != "" && !strings.Contains(got[0].Evidence, tt.wantInEvid) {
				t.Errorf("evidence %q does not name %q", got[0].Evidence, tt.wantInEvid)
			}
		})
	}
}

func TestSigThresholdOrder(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name       string
		snapshot   model.AccountSnapshot
		wantCount  int
		wantInEvid string
	}{
		{
			name:      "the recommended order",
			snapshot:  snapshotWith(model.Thresholds{Low: 1, Medium: 2, High: 3}),
			wantCount: 0,
		},
		{
			name:      "equal tiers are in order",
			snapshot:  snapshotWith(model.Thresholds{Low: 2, Medium: 2, High: 2}),
			wantCount: 0,
		},
		{
			name:      "all zero is in order",
			snapshot:  snapshotWith(model.Thresholds{}),
			wantCount: 0,
		},
		{
			name:       "a medium above the high",
			snapshot:   snapshotWith(model.Thresholds{Low: 1, Medium: 5, High: 3}),
			wantCount:  1,
			wantInEvid: "low 1, medium 5, high 3",
		},
		{
			name:      "a low demanding more than the medium",
			snapshot:  snapshotWith(model.Thresholds{Low: 4, Medium: 2, High: 2}),
			wantCount: 1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sigThresholdOrder(contextWith(policy.Account{ID: testAccount}, tt.snapshot))
			if len(got) != tt.wantCount {
				t.Fatalf("got %d findings, want %d: %+v", len(got), tt.wantCount, got)
			}
			if tt.wantCount == 0 {
				return
			}
			if got[0].RuleID != "sig-threshold-order" || got[0].Severity != model.SevWarn {
				t.Errorf("finding = %s/%s, want sig-threshold-order/warn", got[0].RuleID, got[0].Severity)
			}
			if tt.wantInEvid != "" && !strings.Contains(got[0].Evidence, tt.wantInEvid) {
				t.Errorf("evidence %q does not name %q", got[0].Evidence, tt.wantInEvid)
			}
		})
	}
}

// TestThrDriftCoversDeclaredTiersOnly pins that an omitted tier is "not declared"
// rather than "expected zero", so it can never disagree with the chain.
func TestThrDriftCoversDeclaredTiersOnly(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name       string
		declared   *policy.ExpectedThresholds
		observed   model.Thresholds
		wantCount  int
		wantInEvid string
	}{
		{
			name:     "nothing declared",
			observed: model.Thresholds{Low: 1, Medium: 2, High: 3},
		},
		{
			name:     "every tier matches",
			declared: &policy.ExpectedThresholds{Low: number(1), Medium: number(2), High: number(3)},
			observed: model.Thresholds{Low: 1, Medium: 2, High: 3},
		},
		{
			name:     "an omitted tier is not an expected zero",
			declared: &policy.ExpectedThresholds{Low: number(1)},
			observed: model.Thresholds{Low: 1, Medium: 5, High: 10},
		},
		{
			name:       "one tier differs",
			declared:   &policy.ExpectedThresholds{High: number(10)},
			observed:   model.Thresholds{Low: 1, Medium: 2, High: 5},
			wantCount:  1,
			wantInEvid: "high declared 10, chain shows 5",
		},
		{
			name:       "an explicit zero disagrees with a nonzero chain tier",
			declared:   &policy.ExpectedThresholds{Medium: number(0)},
			observed:   model.Thresholds{Medium: 3},
			wantCount:  1,
			wantInEvid: "medium declared 0, chain shows 3",
		},
		{
			name:      "all three tiers differ in one finding",
			declared:  &policy.ExpectedThresholds{Low: number(1), Medium: number(2), High: number(3)},
			observed:  model.Thresholds{Low: 2, Medium: 4, High: 6},
			wantCount: 1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			account := policy.Account{ID: testAccount, ExpectedThresholds: tt.declared}
			got := thrDrift(contextWith(account, snapshotWith(tt.observed)))
			if len(got) != tt.wantCount {
				t.Fatalf("got %d findings, want %d: %+v", len(got), tt.wantCount, got)
			}
			if tt.wantCount == 0 {
				return
			}
			if got[0].RuleID != "thr-drift" || got[0].Severity != model.SevWarn {
				t.Errorf("finding = %s/%s, want thr-drift/warn", got[0].RuleID, got[0].Severity)
			}
			if tt.wantInEvid != "" && !strings.Contains(got[0].Evidence, tt.wantInEvid) {
				t.Errorf("evidence %q does not name %q", got[0].Evidence, tt.wantInEvid)
			}
		})
	}
}

// An account Gauge could not read, or read on the wrong network, carries no facts
// about signers or thresholds, so none of the nine may speak about it. Each row is
// first shown to fire on a readable account, so a guard that was never reachable
// cannot pass as tested here.
func TestSignerAndThresholdRulesSkipUnreadAccounts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		check    func(Context) []model.Finding
		account  policy.Account
		snapshot model.AccountSnapshot
	}{
		{
			name:     "sig-no-signing-weight",
			check:    sigNoSigningWeight,
			snapshot: snapshotWith(model.Thresholds{}),
		},
		{
			name:     "sig-single-key-high",
			check:    sigSingleKeyHigh,
			snapshot: snapshotWith(model.Thresholds{High: 5}, signerEntry(testAccount, model.SignerEd25519, 5)),
		},
		{
			name:     "sig-hashx",
			check:    sigHashX,
			snapshot: snapshotWith(model.Thresholds{}, signerEntry(testAccount, model.SignerHashX, 0)),
		},
		{
			name:     "sig-unexpected-signer",
			check:    sigUnexpectedSigner,
			account:  policy.Account{ExpectedSigners: expected(policy.ExpectedSigner{Key: secondAccount, Weight: 5})},
			snapshot: snapshotWith(model.Thresholds{High: 1}, signerEntry(testAccount, model.SignerEd25519, 1)),
		},
		{
			name:     "sig-missing-expected",
			check:    sigMissingExpected,
			account:  policy.Account{ExpectedSigners: expected(policy.ExpectedSigner{Key: secondAccount, Weight: 5})},
			snapshot: snapshotWith(model.Thresholds{High: 1}, signerEntry(testAccount, model.SignerEd25519, 1)),
		},
		{
			name:     "sig-weight-drift",
			check:    sigWeightDrift,
			account:  policy.Account{ExpectedSigners: expected(policy.ExpectedSigner{Key: secondAccount, Weight: 5})},
			snapshot: snapshotWith(model.Thresholds{High: 1}, signerEntry(secondAccount, model.SignerEd25519, 1)),
		},
		{
			name:     "sig-threshold-unreachable",
			check:    sigThresholdUnreachable,
			snapshot: snapshotWith(model.Thresholds{High: 10}, signerEntry(testAccount, model.SignerEd25519, 1)),
		},
		{
			name:     "sig-threshold-order",
			check:    sigThresholdOrder,
			snapshot: snapshotWith(model.Thresholds{Low: 3, Medium: 2, High: 4}, signerEntry(testAccount, model.SignerEd25519, 4)),
		},
		{
			name:     "thr-drift",
			check:    thrDrift,
			account:  policy.Account{ExpectedThresholds: &policy.ExpectedThresholds{High: number(10)}},
			snapshot: snapshotWith(model.Thresholds{High: 3}, signerEntry(testAccount, model.SignerEd25519, 3)),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			account := tc.account
			account.ID = testAccount
			readable := tc.check(contextWith(account, tc.snapshot))
			if len(readable) == 0 {
				t.Fatalf("%s does not fire on a readable account, so the guards below prove nothing", tc.name)
			}
			for _, unread := range []struct {
				name   string
				mutate func(*Context)
			}{
				{"missing", func(c *Context) { c.Missing = true }},
				{"network mismatch", func(c *Context) { c.Mismatch = true }},
			} {
				ctx := contextWith(account, tc.snapshot)
				unread.mutate(&ctx)
				if got := tc.check(ctx); len(got) != 0 {
					t.Errorf("%s spoke about an account with a %s: %+v", tc.name, unread.name, got)
				}
			}
		})
	}
}

// sig-threshold-unreachable is on the non-acceptable list, so no [[accept]] entry
// can silence it: an unreachable tier is a fact about control, not a preference.
func TestUnreachableTierCannotBeAccepted(t *testing.T) {
	t.Parallel()
	if policy.Acceptable("sig-threshold-unreachable") {
		t.Error("an unreachable tier must not be acceptable, or a policy can hide it")
	}
	for _, id := range []string{"sig-threshold-order", "thr-drift", "sig-hashx", "sig-single-key-high", "sig-no-signing-weight"} {
		if !policy.Acceptable(id) {
			t.Errorf("%s should be acceptable: a role or an operator may reasonably disagree with it", id)
		}
	}
}
