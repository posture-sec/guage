package run

import (
	"context"
	"testing"
	"time"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/rules"
)

// oneShape is the account that raises every rule Appendix A names plus one it does
// not, so a role's row is tested against a finding that exists rather than against
// an expectation the snapshot never raises. sig-missing-expected is the control: no
// row names it, so it must read HIGH under all five roles. The clawback is observed
// and undeclared, which is the one cell that comes from the 6.1 catalog row rather
// than from Appendix A's prose, so it is tested here as a run rather than only as a
// table.
func oneShape() model.AccountSnapshot {
	return model.AccountSnapshot{
		ID:         accountOne,
		Network:    testnet,
		Sequence:   "1",
		Ledger:     10,
		Thresholds: model.Thresholds{Low: 1, Medium: 5, High: 5},
		Signers: []model.Signer{
			{Key: accountOne, Type: model.SignerEd25519, Weight: 5},
			{Key: accountTwo, Type: model.SignerEd25519, Weight: 3},
		},
		Flags: model.Flags{AuthRequired: true, ClawbackEnabled: true},
	}
}

func oneDeclarations(role model.Role) policy.Account {
	return policy.Account{
		ID:   accountOne,
		Role: role,
		ExpectedSigners: &[]policy.ExpectedSigner{
			{Key: accountOne, Weight: 4},
			{Key: accountThree, Weight: 2},
		},
		ExpectedThresholds: &policy.ExpectedThresholds{High: intPointer(3)},
		ExpectedFlags:      &policy.ExpectedFlags{AuthRequired: boolPointer(false)},
	}
}

// unreachableShape raises the non-acceptable tier rule and nothing else, so the
// claim that a role cannot move it is tested on its own.
func unreachableShape() model.AccountSnapshot {
	return model.AccountSnapshot{
		ID:         accountTwo,
		Network:    testnet,
		Sequence:   "1",
		Ledger:     11,
		Thresholds: model.Thresholds{Low: 1, Medium: 1, High: 9},
		Signers:    []model.Signer{{Key: accountTwo, Type: model.SignerEd25519, Weight: 1}},
	}
}

func intPointer(v int) *int { return &v }

func boolPointer(v bool) *bool { return &v }

// expect_locked is Appendix A's issuer row alone, and the INFO it earns has to
// survive the same adjustment that raises everything else for that role. The issuer
// row lists no floor for sig-no-signing-weight, so a declared lock is still a
// declaration that means something after 3.4; on any other role the word is unread,
// because a treasury that says it meant to lose control is not an issuer that has
// stopped issuing.
func TestExpectLockedSurvivesRoleAdjustment(t *testing.T) {
	t.Parallel()
	locked := boolPointer(true)
	unlocked := boolPointer(false)
	lockedAccount := policy.Account{ID: accountOne, Role: model.RoleIssuer, ExpectLocked: locked}
	for _, tc := range []struct {
		name    string
		account policy.Account
		want    model.Severity
	}{
		{name: "issuer declares the lock", account: lockedAccount, want: model.SevInfo},
		{name: "issuer declares no lock", account: policy.Account{ID: accountOne, Role: model.RoleIssuer, ExpectLocked: unlocked}, want: model.SevHigh},
		{name: "issuer declares nothing", account: policy.Account{ID: accountOne, Role: model.RoleIssuer}, want: model.SevHigh},
		{name: "treasury cannot borrow the word", account: policy.Account{ID: accountOne, Role: model.RoleTreasury, ExpectLocked: locked}, want: model.SevHigh},
		{name: "the other row cannot either", account: policy.Account{ID: accountOne, Role: model.RoleOther, ExpectLocked: locked}, want: model.SevHigh},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fs := &fakeSource{
				info:      model.NetworkInfo{Passphrase: testnet, Ledger: 1},
				snapshots: map[string]model.AccountSnapshot{accountOne: {ID: accountOne, Network: testnet, Sequence: "1", Ledger: 1}},
			}
			cfg := &policy.Config{
				Version:   1,
				Network:   "testnet",
				Endpoints: []string{"https://horizon-testnet.example"},
				Accounts:  []policy.Account{tc.account},
			}
			result, err := Audit(context.Background(), cfg, AuditOptions{
				ToolVersion: "test",
				FailOn:      model.SevHigh,
				NewSource:   stubFactory(fs),
				Evaluate:    rules.Evaluate,
			})
			if err != nil {
				t.Fatalf("Audit: %v", err)
			}
			var weight *model.Finding
			for i, finding := range result.Accounts[0].Findings {
				if finding.RuleID == "sig-no-signing-weight" {
					weight = &result.Accounts[0].Findings[i]
				}
			}
			if weight == nil {
				t.Fatalf("findings = %+v, want sig-no-signing-weight on an account with no signers", result.Accounts[0].Findings)
			}
			if weight.Severity != tc.want {
				t.Errorf("severity = %s, want %s", weight.Severity, tc.want)
			}
		})
	}
}

func TestAuditAppliesRoleSeverities(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		role model.Role
		want map[string]model.Severity
	}{
		{
			role: model.RoleIssuer,
			want: map[string]model.Severity{
				"sig-single-key-high":      model.SevHigh,
				"flag-drift":               model.SevHigh,
				"sig-unexpected-signer":    model.SevHigh,
				"sig-weight-drift":         model.SevWarn,
				"thr-drift":                model.SevWarn,
				"sig-missing-expected":     model.SevHigh,
				"flag-undeclared-clawback": model.SevHigh,
				// The trigger Appendix A added for this role alone: the issuer row is the
				// one that expects a published home domain.
				"id-no-home-domain": model.SevWarn,
			},
		},
		{
			role: model.RoleTreasury,
			want: map[string]model.Severity{
				"sig-single-key-high":      model.SevHigh,
				"flag-drift":               model.SevWarn,
				"sig-unexpected-signer":    model.SevHigh,
				"sig-weight-drift":         model.SevHigh,
				"thr-drift":                model.SevHigh,
				"sig-missing-expected":     model.SevHigh,
				"flag-undeclared-clawback": model.SevWarn,
			},
		},
		{
			role: model.RoleDistribution,
			want: map[string]model.Severity{
				"sig-single-key-high":      model.SevWarn,
				"flag-drift":               model.SevWarn,
				"sig-unexpected-signer":    model.SevHigh,
				"sig-weight-drift":         model.SevWarn,
				"thr-drift":                model.SevWarn,
				"sig-missing-expected":     model.SevHigh,
				"flag-undeclared-clawback": model.SevWarn,
			},
		},
		{
			role: model.RoleMerchant,
			want: map[string]model.Severity{
				"sig-single-key-high":      model.SevWarn,
				"flag-drift":               model.SevWarn,
				"sig-unexpected-signer":    model.SevHigh,
				"sig-weight-drift":         model.SevWarn,
				"thr-drift":                model.SevWarn,
				"sig-missing-expected":     model.SevHigh,
				"flag-undeclared-clawback": model.SevWarn,
			},
		},
		{
			// The row an undeclared account falls to, and the one check mode declares.
			// This is the severity set task 3.1 shipped, which is the point: a role that
			// names nothing changes nothing.
			role: model.RoleOther,
			want: map[string]model.Severity{
				"sig-single-key-high":      model.SevInfo,
				"flag-drift":               model.SevWarn,
				"sig-unexpected-signer":    model.SevHigh,
				"sig-weight-drift":         model.SevWarn,
				"thr-drift":                model.SevWarn,
				"sig-missing-expected":     model.SevHigh,
				"flag-undeclared-clawback": model.SevWarn,
			},
		},
	} {
		t.Run(string(tc.role), func(t *testing.T) {
			t.Parallel()
			fs := &fakeSource{
				info: model.NetworkInfo{Passphrase: testnet, Ledger: 12},
				snapshots: map[string]model.AccountSnapshot{
					accountOne: oneShape(),
					accountTwo: unreachableShape(),
				},
			}
			cfg := &policy.Config{
				Version:   1,
				Network:   "testnet",
				Endpoints: []string{"https://horizon-testnet.example"},
				Accounts: []policy.Account{
					oneDeclarations(tc.role),
					{ID: accountTwo, Role: tc.role},
				},
			}
			result, err := Audit(context.Background(), cfg, AuditOptions{
				ToolVersion: "test",
				FailOn:      model.SevHigh,
				NewSource:   stubFactory(fs),
				Evaluate:    rules.Evaluate,
				Now:         func() time.Time { return time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC) },
			})
			if err != nil {
				t.Fatalf("Audit: %v", err)
			}
			got := map[string]model.Severity{}
			for _, finding := range result.Accounts[0].Findings {
				if finding.Account != accountOne {
					t.Errorf("a finding about another account landed on this report: %+v", finding)
				}
				got[finding.RuleID] = finding.Severity
			}
			for rule, want := range tc.want {
				if got[rule] != want {
					t.Errorf("%s came back %s, want %s", rule, got[rule], want)
				}
			}
			if len(got) != len(tc.want) {
				t.Errorf("findings = %v, want exactly %v", got, tc.want)
			}
			// The non-acceptable rule is HIGH for every role. The only other finding
			// this account may carry is the role trigger, which the issuer row alone
			// asks for.
			tier := result.Accounts[1].Findings
			var unreachable bool
			for _, finding := range tier {
				switch finding.RuleID {
				case "sig-threshold-unreachable":
					unreachable = true
					if finding.Severity != model.SevHigh {
						t.Errorf("the unreachable tier came back %s under role %s, want high", finding.Severity, tc.role)
					}
				case "id-no-home-domain":
					if tc.role != model.RoleIssuer {
						t.Errorf("the home-domain role trigger fired for role %s: %+v", tc.role, finding)
					}
				default:
					t.Errorf("unexpected finding on the unreachable account: %+v", finding)
				}
			}
			if !unreachable {
				t.Errorf("role %s: the unreachable tier did not fire: %+v", tc.role, tier)
			}
		})
	}
}

// A role changes the weight of a fact, not the fact. The fingerprint is what an
// accepted-risk entry keys on, so if adjustment moved it, an operator who declared
// a role after accepting a risk would silently un-accept it.
func TestRoleAdjustmentLeavesTheFingerprintAlone(t *testing.T) {
	t.Parallel()
	var fingerprints []string
	for _, role := range []model.Role{model.RoleOther, model.RoleIssuer} {
		fs := &fakeSource{
			info:      model.NetworkInfo{Passphrase: testnet, Ledger: 1},
			snapshots: map[string]model.AccountSnapshot{accountOne: oneShape()},
		}
		cfg := &policy.Config{
			Version:   1,
			Network:   "testnet",
			Endpoints: []string{"https://horizon-testnet.example"},
			Accounts:  []policy.Account{oneDeclarations(role)},
		}
		result, err := Audit(context.Background(), cfg, AuditOptions{
			ToolVersion: "test",
			FailOn:      model.SevHigh,
			NewSource:   stubFactory(fs),
			Evaluate:    rules.Evaluate,
		})
		if err != nil {
			t.Fatalf("Audit: %v", err)
		}
		for _, finding := range result.Accounts[0].Findings {
			if finding.RuleID != "sig-single-key-high" {
				continue
			}
			fingerprints = append(fingerprints, finding.Fingerprint)
		}
	}
	if len(fingerprints) != 2 {
		t.Fatalf("found %d single-key findings, want one per role", len(fingerprints))
	}
	if fingerprints[0] != fingerprints[1] {
		t.Errorf("the fingerprint moved with the role: %s then %s", fingerprints[0], fingerprints[1])
	}
}

// An acceptance covers a finding whatever its role-raised severity, because
// adjustment runs before matching. The rule has to be acceptable for the entry to
// load at all, so the case that matters is that a raise does not defeat an
// acceptance an operator already recorded.
func TestRoleAdjustmentRunsBeforeAcceptMatching(t *testing.T) {
	t.Parallel()
	fs := &fakeSource{
		info:      model.NetworkInfo{Passphrase: testnet, Ledger: 1},
		snapshots: map[string]model.AccountSnapshot{accountOne: oneShape()},
	}
	account := oneDeclarations(model.RoleIssuer)
	cfg := &policy.Config{
		Version:   1,
		Network:   "testnet",
		Endpoints: []string{"https://horizon-testnet.example"},
		Accounts:  []policy.Account{account},
		Accepts: []policy.Acceptance{{
			Rule:    "flag-drift",
			Account: accountOne,
			Reason:  "the issuer flipped auth deliberately and the policy is behind",
		}},
	}
	result, err := Audit(context.Background(), cfg, AuditOptions{
		ToolVersion: "test",
		FailOn:      model.SevHigh,
		NewSource:   stubFactory(fs),
		Evaluate:    rules.Evaluate,
	})
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	var flagged *model.Finding
	for i, finding := range result.Accounts[0].Findings {
		if finding.RuleID == "flag-drift" {
			flagged = &result.Accounts[0].Findings[i]
		}
	}
	if flagged == nil {
		t.Fatal("flag-drift did not fire on a shape that raises it")
	}
	if flagged.Severity != model.SevHigh {
		t.Errorf("severity = %s, want high from the issuer row", flagged.Severity)
	}
	if !flagged.Accepted {
		t.Error("the recorded acceptance did not cover the finding after the role raised it")
	}
}
