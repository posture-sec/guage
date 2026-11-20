package rules

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/sep1"
)

// Each rule passes its ID to model.NewFinding as a literal, and the registry
// names the same rule with a second literal in rules.go. A typo on either side
// gives a finding whose RuleID matches no registered rule, and since the ID is
// hashed into the fingerprint, no [[accept]] entry can ever name it: a known risk
// silently stops being accepted. This test fires every registered rule and holds
// each finding to the registry entry that produced it.
//
// The firing contexts are local on purpose. They use their own keys and builders
// so a change to another test file's helpers cannot quietly stop a rule firing.

const (
	ruleIDAccount = "GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG"
	ruleIDOther   = "GBK4NFJ7SVMUNPBIYBC4WNMCTKI6Z66MYYWZQSSLQNOSUSTMKVUKE7M3"
	ruleIDNetwork = "Test SDF Network ; September 2015"
)

func ruleIDSigner(key string, typ model.SignerType, weight int) model.Signer {
	return model.Signer{Key: key, Type: typ, Weight: weight}
}

func ruleIDContext(account policy.Account, snapshot model.AccountSnapshot) Context {
	account.ID = ruleIDAccount
	snapshot.ID = ruleIDAccount
	return Context{Account: account, Snapshot: snapshot, Network: ruleIDNetwork, Expected: ruleIDNetwork}
}

// ruleIDFiring is one context per registered rule, keyed by registry ID, with the
// exact number of findings that context must produce.
func ruleIDFiring() map[string]struct {
	ctx   Context
	count int
} {
	one := []model.Signer{ruleIDSigner(ruleIDAccount, model.SignerEd25519, 1)}
	tiers := func(low, medium, high int) model.Thresholds {
		return model.Thresholds{Low: low, Medium: medium, High: high}
	}
	declared := func(list ...policy.ExpectedSigner) *[]policy.ExpectedSigner { return &list }
	value := func(v int) *int { return &v }
	truth := func(v bool) *bool { return &v }
	domain := "example.org"

	type row = struct {
		ctx   Context
		count int
	}
	return map[string]row{
		"acct-missing": {count: 1, ctx: func() Context {
			c := ruleIDContext(policy.Account{}, model.AccountSnapshot{Endpoint: "https://horizon-testnet.stellar.org"})
			c.Missing = true
			return c
		}()},
		"acct-network-mismatch": {count: 1, ctx: func() Context {
			c := ruleIDContext(policy.Account{}, model.AccountSnapshot{})
			c.Mismatch = true
			c.Network = "Public Global Stellar Network ; September 2015"
			return c
		}()},
		"sig-no-signing-weight": {count: 1, ctx: ruleIDContext(policy.Account{}, model.AccountSnapshot{
			Signers: []model.Signer{ruleIDSigner(ruleIDAccount, model.SignerEd25519, 0)},
		})},
		"sig-threshold-unreachable": {count: 1, ctx: ruleIDContext(policy.Account{}, model.AccountSnapshot{
			Thresholds: tiers(1, 1, 5), Signers: one,
		})},
		"sig-threshold-order": {count: 1, ctx: ruleIDContext(policy.Account{}, model.AccountSnapshot{
			Thresholds: tiers(1, 1, 0), Signers: one,
		})},
		"sig-single-key-high": {count: 1, ctx: ruleIDContext(policy.Account{}, model.AccountSnapshot{
			Thresholds: tiers(1, 1, 1), Signers: one,
		})},
		// Two hash(x) entries give two findings from one rule, so the ID is checked
		// on every finding a rule returns and not only the first.
		"sig-hashx": {count: 2, ctx: ruleIDContext(policy.Account{}, model.AccountSnapshot{
			Thresholds: tiers(1, 1, 1),
			Signers: []model.Signer{
				ruleIDSigner(ruleIDAccount, model.SignerEd25519, 1),
				ruleIDSigner("XFIRSTHASHXFIRSTHASHXFIRSTHASHXFIRSTHASHXFIRSTHASHXFIRS", model.SignerHashX, 1),
				ruleIDSigner("XSECONDHASHXSECONDHASHXSECONDHASHXSECONDHASHXSECONDHASH", model.SignerHashX, 1),
			},
		})},
		"sig-unexpected-signer": {count: 1, ctx: ruleIDContext(
			policy.Account{ExpectedSigners: declared()},
			model.AccountSnapshot{Thresholds: tiers(1, 1, 1), Signers: one},
		)},
		"sig-missing-expected": {count: 1, ctx: ruleIDContext(
			policy.Account{ExpectedSigners: declared(
				policy.ExpectedSigner{Key: ruleIDAccount, Weight: 1},
				policy.ExpectedSigner{Key: ruleIDOther, Weight: 1},
			)},
			model.AccountSnapshot{Thresholds: tiers(1, 1, 1), Signers: one},
		)},
		"sig-weight-drift": {count: 1, ctx: ruleIDContext(
			policy.Account{ExpectedSigners: declared(policy.ExpectedSigner{Key: ruleIDAccount, Weight: 2})},
			model.AccountSnapshot{Thresholds: tiers(1, 1, 1), Signers: one},
		)},
		"thr-drift": {count: 1, ctx: ruleIDContext(
			policy.Account{ExpectedThresholds: &policy.ExpectedThresholds{Low: value(2)}},
			model.AccountSnapshot{Thresholds: tiers(1, 1, 1), Signers: one},
		)},
		"flag-drift": {count: 1, ctx: ruleIDContext(
			policy.Account{ExpectedFlags: &policy.ExpectedFlags{AuthRequired: truth(true)}},
			model.AccountSnapshot{Thresholds: tiers(1, 1, 1), Signers: one},
		)},
		"flag-undeclared-clawback": {count: 1, ctx: ruleIDContext(policy.Account{}, model.AccountSnapshot{
			Thresholds: tiers(1, 1, 1), Signers: one, Flags: model.Flags{ClawbackEnabled: true},
		})},
		"id-no-home-domain": {count: 1, ctx: ruleIDContext(
			policy.Account{HomeDomain: &domain},
			model.AccountSnapshot{Thresholds: tiers(1, 1, 1), Signers: one},
		)},
		"id-toml-unreachable": {count: 1, ctx: ruleIDContext(policy.Account{}, model.AccountSnapshot{
			Thresholds: tiers(1, 1, 1), Signers: one, HomeDomain: domain,
		})},
		"id-toml-account-unlisted": {count: 1, ctx: func() Context {
			c := ruleIDContext(policy.Account{}, model.AccountSnapshot{
				Thresholds: tiers(1, 1, 1), Signers: one, HomeDomain: domain,
			})
			c.Identity = &sep1.Result{
				HomeDomain: domain,
				FailedAt:   sep1.StepOK,
				Identity:   sep1.Identity{Accounts: []string{ruleIDOther}},
			}
			return c
		}()},
	}
}

func TestFindingRuleID(t *testing.T) {
	t.Parallel()
	firing := ruleIDFiring()
	registered := map[string]bool{}
	for _, rule := range All() {
		registered[rule.ID] = true
		row, ok := firing[rule.ID]
		if !ok {
			t.Errorf("registered rule %q has no firing context in this table; add one", rule.ID)
			continue
		}
		findings := rule.Check(row.ctx)
		if len(findings) != row.count {
			t.Errorf("rule %q raised %d findings on its firing context, want %d: %+v", rule.ID, len(findings), row.count, findings)
		}
		for i, finding := range findings {
			if finding.RuleID != rule.ID {
				t.Errorf("rule %q finding %d carries RuleID %q, which is not the registry ID; "+
					"the literal passed to model.NewFinding and the registry entry disagree", rule.ID, i, finding.RuleID)
			}
		}
	}
	// A row naming no registered rule is a stale table, or a rule that was
	// renamed on one side only.
	for id := range firing {
		if !registered[id] {
			t.Errorf("firing table names %q, which is not a registered rule", id)
		}
	}
}

// TestFindingRuleIDThroughEvaluate checks the same promise on the path the run
// actually takes: every finding Evaluate returns names a registered rule.
func TestFindingRuleIDThroughEvaluate(t *testing.T) {
	t.Parallel()
	registered := map[string]bool{}
	for _, rule := range All() {
		registered[rule.ID] = true
	}
	for id, row := range ruleIDFiring() {
		for _, finding := range Evaluate(row.ctx) {
			if !registered[finding.RuleID] {
				t.Errorf("context for %q produced a finding with phantom RuleID %q", id, finding.RuleID)
			}
		}
	}
}
