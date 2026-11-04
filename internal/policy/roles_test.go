package policy

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// TestSeverityForIsAppendixA is task 3.4's Done-when in one table: one rule, one
// severity the rule emitted, and five declared roles that do not agree with each
// other. The emitted column is what the registry ships at today, so every row is
// the real question rather than a hypothetical one.
func TestSeverityForIsAppendixA(t *testing.T) {
	t.Parallel()
	roles := []model.Role{model.RoleIssuer, model.RoleTreasury, model.RoleDistribution, model.RoleMerchant, model.RoleOther}
	for _, tt := range []struct {
		rule    string
		emitted model.Severity
		want    map[model.Role]model.Severity
	}{
		{
			// The row every role names, at three different values.
			rule:    "sig-single-key-high",
			emitted: model.SevInfo,
			want: map[model.Role]model.Severity{
				model.RoleIssuer:       model.SevHigh,
				model.RoleTreasury:     model.SevHigh,
				model.RoleDistribution: model.SevWarn,
				model.RoleMerchant:     model.SevWarn,
				model.RoleOther:        model.SevInfo,
			},
		},
		{
			rule:    "flag-drift",
			emitted: model.SevWarn,
			want: map[model.Role]model.Severity{
				model.RoleIssuer:       model.SevHigh,
				model.RoleTreasury:     model.SevWarn,
				model.RoleDistribution: model.SevWarn,
				model.RoleMerchant:     model.SevWarn,
				model.RoleOther:        model.SevWarn,
			},
		},
		{
			// "signer and threshold drift are HIGH" for a treasury, read as the three
			// rules that compare a declaration with the chain.
			rule:    "sig-weight-drift",
			emitted: model.SevWarn,
			want: map[model.Role]model.Severity{
				model.RoleIssuer:       model.SevWarn,
				model.RoleTreasury:     model.SevHigh,
				model.RoleDistribution: model.SevWarn,
				model.RoleMerchant:     model.SevWarn,
				model.RoleOther:        model.SevWarn,
			},
		},
		{
			rule:    "thr-drift",
			emitted: model.SevWarn,
			want: map[model.Role]model.Severity{
				model.RoleIssuer:       model.SevWarn,
				model.RoleTreasury:     model.SevHigh,
				model.RoleDistribution: model.SevWarn,
				model.RoleMerchant:     model.SevWarn,
				model.RoleOther:        model.SevWarn,
			},
		},
		{
			// The issuer row's "unexpected signers are HIGH" is the baseline this rule
			// already emits, and no other row names it, so it does not move anywhere.
			rule:    "sig-unexpected-signer",
			emitted: model.SevHigh,
			want: map[model.Role]model.Severity{
				model.RoleIssuer:       model.SevHigh,
				model.RoleTreasury:     model.SevHigh,
				model.RoleDistribution: model.SevHigh,
				model.RoleMerchant:     model.SevHigh,
				model.RoleOther:        model.SevHigh,
			},
		},
		{
			// A declared signer the chain no longer shows is not "drift", so neither
			// the distribution nor the merchant row softens it.
			rule:    "sig-missing-expected",
			emitted: model.SevHigh,
			want: map[model.Role]model.Severity{
				model.RoleIssuer:       model.SevHigh,
				model.RoleTreasury:     model.SevHigh,
				model.RoleDistribution: model.SevHigh,
				model.RoleMerchant:     model.SevHigh,
				model.RoleOther:        model.SevHigh,
			},
		},
		{
			// The catalog's own by-role cell, not Appendix A's prose: `flag-drift` is
			// what the issuer row names, and this rule is the sibling the 6.1 table
			// gives "WARN (HIGH for issuer)".
			rule:    "flag-undeclared-clawback",
			emitted: model.SevWarn,
			want: map[model.Role]model.Severity{
				model.RoleIssuer:       model.SevHigh,
				model.RoleTreasury:     model.SevWarn,
				model.RoleDistribution: model.SevWarn,
				model.RoleMerchant:     model.SevWarn,
				model.RoleOther:        model.SevWarn,
			},
		},
		{
			// Nothing in the table moves these. Two are on the non-acceptable list,
			// which a role cannot reach either, and two are rules no row names at all.
			rule:    "sig-threshold-unreachable",
			emitted: model.SevHigh,
			want: map[model.Role]model.Severity{
				model.RoleIssuer:       model.SevHigh,
				model.RoleTreasury:     model.SevHigh,
				model.RoleDistribution: model.SevHigh,
				model.RoleMerchant:     model.SevHigh,
				model.RoleOther:        model.SevHigh,
			},
		},
		{
			rule:    "acct-missing",
			emitted: model.SevHigh,
			want: map[model.Role]model.Severity{
				model.RoleIssuer:       model.SevHigh,
				model.RoleTreasury:     model.SevHigh,
				model.RoleDistribution: model.SevHigh,
				model.RoleMerchant:     model.SevHigh,
				model.RoleOther:        model.SevHigh,
			},
		},
		{
			rule:    "sig-hashx",
			emitted: model.SevWarn,
			want: map[model.Role]model.Severity{
				model.RoleIssuer:       model.SevWarn,
				model.RoleTreasury:     model.SevWarn,
				model.RoleDistribution: model.SevWarn,
				model.RoleMerchant:     model.SevWarn,
				model.RoleOther:        model.SevWarn,
			},
		},
		{
			rule:    "id-toml-account-unlisted",
			emitted: model.SevWarn,
			want: map[model.Role]model.Severity{
				model.RoleIssuer:       model.SevWarn,
				model.RoleTreasury:     model.SevWarn,
				model.RoleDistribution: model.SevWarn,
				model.RoleMerchant:     model.SevWarn,
				model.RoleOther:        model.SevWarn,
			},
		},
	} {
		for _, role := range roles {
			want, listed := tt.want[role]
			if !listed {
				t.Fatalf("table for %s has no row for role %s", tt.rule, role)
			}
			if got := SeverityFor(role, tt.rule, tt.emitted); got != want {
				t.Errorf("%s for role %s: got %s, want %s", tt.rule, role, got, want)
			}
		}
	}
}

// TestSeverityForNeverLowers pins the reading the map is built on: a role is a
// floor, not an override. It is what makes the plan's invariant true without a
// special case for the non-acceptable list, and it is the property that keeps a
// declared role from talking a HIGH down into something a run passes on.
func TestSeverityForNeverLowers(t *testing.T) {
	t.Parallel()
	for role, rows := range roleSeverities {
		for rule, floor := range rows {
			for _, emitted := range []model.Severity{model.SevInfo, model.SevWarn, model.SevHigh} {
				got := SeverityFor(role, rule, emitted)
				if got < emitted {
					t.Errorf("role %s, rule %s: %s emitted came back %s, which is a lowering", role, rule, emitted, got)
				}
				if emitted > floor && got != emitted {
					t.Errorf("role %s, rule %s: a severity above the row's floor came back %s, want %s", role, rule, got, emitted)
				}
				if emitted < floor && got != floor {
					t.Errorf("role %s, rule %s: a severity below the row's floor came back %s, want %s", role, rule, got, floor)
				}
			}
		}
	}
}

func TestSeverityForOnInputItDoesNotKnow(t *testing.T) {
	t.Parallel()
	for _, role := range []model.Role{"", "custodian", model.RoleIssuer} {
		if got := SeverityFor(role, "rule-the-map-never-names", model.SevInfo); got != model.SevInfo {
			t.Errorf("role %q: an unlisted rule came back %s", role, got)
		}
	}
	// The issuer row is the only one that expects a published home domain, and the
	// only role whose expect_locked declaration is read as intent rather than as a
	// loss of control.
	for _, role := range []model.Role{model.RoleTreasury, model.RoleDistribution, model.RoleMerchant, model.RoleOther, ""} {
		if ExpectsHomeDomain(role) {
			t.Errorf("role %q does not have a home domain in its Appendix A row", role)
		}
		if AllowsExpectLocked(role) {
			t.Errorf("role %q may not declare expect_locked", role)
		}
	}
	if !ExpectsHomeDomain(model.RoleIssuer) || !AllowsExpectLocked(model.RoleIssuer) {
		t.Error("the issuer row is the one Appendix A describes as expecting a domain and allowed to lock")
	}
}
