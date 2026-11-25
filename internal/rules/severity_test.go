package rules

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
)

// TestRuleSeverities walks the registry and holds every finding a rule emits to
// the three legal severities and to the acceptance precondition.
//
// Severity is the rule's decision (Rule carries no separate value), and
// model.NewFinding takes it as an argument, so an out-of-range number is only
// caught here. Acceptance is not the rule's decision at all: internal/run
// applies [[accept]] once per account, after the role floors, and a rule that
// set it would accept a finding no policy names.
func TestRuleSeverities(t *testing.T) {
	t.Parallel()
	firing := ruleIDFiring()
	for _, rule := range All() {
		rule := rule
		t.Run(rule.ID, func(t *testing.T) {
			t.Parallel()
			row, ok := firing[rule.ID]
			if !ok {
				t.Fatalf("registered rule %q has no firing context in ruleIDFiring(); add one", rule.ID)
			}
			findings := rule.Check(row.ctx)
			if len(findings) == 0 {
				t.Fatalf("rule %q raised nothing on its firing context, so its severity is unpinned", rule.ID)
			}
			for i, finding := range findings {
				if !finding.Severity.Valid() {
					t.Errorf("rule %q finding %d calls %d a severity; the defined values are info, warn and high", rule.ID, i, int(finding.Severity))
					continue
				}
				switch name := finding.Severity.String(); name {
				case "info", "warn", "high":
				default:
					t.Errorf("rule %q finding %d names its severity %q, want info, warn or high", rule.ID, i, name)
				}
				if finding.Accepted {
					t.Errorf("rule %q finding %d is pre-set accepted; acceptance belongs to the run, "+
						"which applies [[accept]] after the role floors", rule.ID, i)
				}
			}
		})
	}
}

// TestRuleSeverityKnocksBeforeAcceptingTheLock pins the one severity a rule
// decides from the declaration rather than the observation: an issuer that
// declares expect_locked means the black-hole shape, so sig-no-signing-weight
// reports it at INFO. The same snapshot on a role that may not declare a lock
// stays HIGH, and the rule still never accepts.
func TestRuleSeverityKnocksBeforeAcceptingTheLock(t *testing.T) {
	t.Parallel()
	snapshot := model.AccountSnapshot{
		ID:      testAccount,
		Signers: []model.Signer{signerEntry(testAccount, model.SignerEd25519, 0)},
	}
	for _, tt := range []struct {
		name     string
		role     model.Role
		locked   *bool
		severity model.Severity
	}{
		{"issuer declares the lock", model.RoleIssuer, boolean(true), model.SevInfo},
		{"issuer declares no lock", model.RoleIssuer, boolean(false), model.SevHigh},
		{"issuer declares nothing", model.RoleIssuer, nil, model.SevHigh},
		{"treasury declares a lock it may not make", model.RoleTreasury, boolean(true), model.SevHigh},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			locks := policy.AllowsExpectLocked(tt.role) && tt.locked != nil && *tt.locked
			if locks != (tt.severity == model.SevInfo) {
				t.Fatalf("the policy table lets %q declare a lock: %t, but the case expects severity %s", tt.role, locks, tt.severity)
			}
			findings := sigNoSigningWeight(Context{
				Account:  policy.Account{ID: testAccount, Role: tt.role, ExpectLocked: tt.locked},
				Snapshot: snapshot,
			})
			if len(findings) != 1 {
				t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
			}
			if findings[0].Severity != tt.severity {
				t.Errorf("severity = %s, want %s", findings[0].Severity, tt.severity)
			}
			if findings[0].Accepted {
				t.Error("the lock declaration accepted the finding; only an [[accept]] entry may")
			}
		})
	}
}
