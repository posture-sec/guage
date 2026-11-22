package policy

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// catalogRules is PRD.md 6.1's rule catalog (v0.1) written out by hand: all
// sixteen rule IDs, in the order the table lists them. It is deliberately not
// derived from roleFloors, because a test that reads its expectations from the
// code under test cannot see a misspelled severity key: the typo would define its
// own row and pass.
func catalogRules() []string {
	return []string{
		"acct-missing",
		"acct-network-mismatch",
		"sig-no-signing-weight",
		"sig-threshold-unreachable",
		"sig-threshold-order",
		"sig-single-key-high",
		"sig-hashx",
		"sig-unexpected-signer",
		"sig-missing-expected",
		"sig-weight-drift",
		"thr-drift",
		"flag-drift",
		"flag-undeclared-clawback",
		"id-no-home-domain",
		"id-toml-unreachable",
		"id-toml-account-unlisted",
	}
}

// catalogRoles is Appendix A's five rows in the order it prints them.
func catalogRoles() []model.Role {
	return []model.Role{
		model.RoleIssuer,
		model.RoleTreasury,
		model.RoleDistribution,
		model.RoleMerchant,
		model.RoleOther,
	}
}

// TestSeverityForEveryRoleAndCatalogRule is issue #142 as one matrix: every one of
// the sixteen catalog rules against every one of the five roles, each asking
// SeverityFor what a finding emitted at INFO becomes.
//
// INFO is the base on purpose. A role is a floor, so the answer here is the cell
// Appendix A names for that rule or INFO where the row is silent, which is what
// makes this pass a test of the severity keys themselves rather than of the
// catalog's default severities. That a role never lowers a finding is the property
// TestSeverityForNeverLowers pins over the same maps.
func TestSeverityForEveryRoleAndCatalogRule(t *testing.T) {
	t.Parallel()
	want := map[string]map[model.Role]model.Severity{
		// Appendix A does not name these at all: two are on the non-acceptable list,
		// which a role cannot reach either, and the rest appear in no row.
		"acct-missing": {
			model.RoleIssuer:       model.SevInfo,
			model.RoleTreasury:     model.SevInfo,
			model.RoleDistribution: model.SevInfo,
			model.RoleMerchant:     model.SevInfo,
			model.RoleOther:        model.SevInfo,
		},
		"acct-network-mismatch": {
			model.RoleIssuer:       model.SevInfo,
			model.RoleTreasury:     model.SevInfo,
			model.RoleDistribution: model.SevInfo,
			model.RoleMerchant:     model.SevInfo,
			model.RoleOther:        model.SevInfo,
		},
		"sig-no-signing-weight": {
			model.RoleIssuer:       model.SevInfo,
			model.RoleTreasury:     model.SevInfo,
			model.RoleDistribution: model.SevInfo,
			model.RoleMerchant:     model.SevInfo,
			model.RoleOther:        model.SevInfo,
		},
		"sig-threshold-unreachable": {
			model.RoleIssuer:       model.SevInfo,
			model.RoleTreasury:     model.SevInfo,
			model.RoleDistribution: model.SevInfo,
			model.RoleMerchant:     model.SevInfo,
			model.RoleOther:        model.SevInfo,
		},
		"sig-threshold-order": {
			model.RoleIssuer:       model.SevInfo,
			model.RoleTreasury:     model.SevInfo,
			model.RoleDistribution: model.SevInfo,
			model.RoleMerchant:     model.SevInfo,
			model.RoleOther:        model.SevInfo,
		},
		// The one row every role names, at three different values.
		"sig-single-key-high": {
			model.RoleIssuer:       model.SevHigh,
			model.RoleTreasury:     model.SevHigh,
			model.RoleDistribution: model.SevWarn,
			model.RoleMerchant:     model.SevWarn,
			model.RoleOther:        model.SevInfo,
		},
		"sig-hashx": {
			model.RoleIssuer:       model.SevInfo,
			model.RoleTreasury:     model.SevInfo,
			model.RoleDistribution: model.SevInfo,
			model.RoleMerchant:     model.SevInfo,
			model.RoleOther:        model.SevInfo,
		},
		// The issuer row names unexpected signers; no other row does.
		"sig-unexpected-signer": {
			model.RoleIssuer:       model.SevHigh,
			model.RoleTreasury:     model.SevHigh,
			model.RoleDistribution: model.SevInfo,
			model.RoleMerchant:     model.SevInfo,
			model.RoleOther:        model.SevInfo,
		},
		// The treasury row reads "signer and threshold drift are HIGH", and a
		// declared signer the chain no longer shows is not drift for the other rows.
		"sig-missing-expected": {
			model.RoleIssuer:       model.SevInfo,
			model.RoleTreasury:     model.SevHigh,
			model.RoleDistribution: model.SevInfo,
			model.RoleMerchant:     model.SevInfo,
			model.RoleOther:        model.SevInfo,
		},
		"sig-weight-drift": {
			model.RoleIssuer:       model.SevInfo,
			model.RoleTreasury:     model.SevHigh,
			model.RoleDistribution: model.SevWarn,
			model.RoleMerchant:     model.SevWarn,
			model.RoleOther:        model.SevInfo,
		},
		"thr-drift": {
			model.RoleIssuer:       model.SevInfo,
			model.RoleTreasury:     model.SevHigh,
			model.RoleDistribution: model.SevWarn,
			model.RoleMerchant:     model.SevWarn,
			model.RoleOther:        model.SevInfo,
		},
		"flag-drift": {
			model.RoleIssuer:       model.SevHigh,
			model.RoleTreasury:     model.SevInfo,
			model.RoleDistribution: model.SevWarn,
			model.RoleMerchant:     model.SevWarn,
			model.RoleOther:        model.SevInfo,
		},
		// The catalog's own by-role cell, not Appendix A's prose: this rule is the
		// sibling the 6.1 table gives "WARN (HIGH for issuer)".
		"flag-undeclared-clawback": {
			model.RoleIssuer:       model.SevHigh,
			model.RoleTreasury:     model.SevInfo,
			model.RoleDistribution: model.SevInfo,
			model.RoleMerchant:     model.SevInfo,
			model.RoleOther:        model.SevInfo,
		},
		"id-no-home-domain": {
			model.RoleIssuer:       model.SevInfo,
			model.RoleTreasury:     model.SevInfo,
			model.RoleDistribution: model.SevInfo,
			model.RoleMerchant:     model.SevInfo,
			model.RoleOther:        model.SevInfo,
		},
		"id-toml-unreachable": {
			model.RoleIssuer:       model.SevInfo,
			model.RoleTreasury:     model.SevInfo,
			model.RoleDistribution: model.SevInfo,
			model.RoleMerchant:     model.SevInfo,
			model.RoleOther:        model.SevInfo,
		},
		"id-toml-account-unlisted": {
			model.RoleIssuer:       model.SevInfo,
			model.RoleTreasury:     model.SevInfo,
			model.RoleDistribution: model.SevInfo,
			model.RoleMerchant:     model.SevInfo,
			model.RoleOther:        model.SevInfo,
		},
	}
	// The matrix has to cover the catalog exactly: a rule added to the catalog and
	// forgotten here would otherwise be silently untested.
	if len(want) != len(catalogRules()) {
		t.Fatalf("expectations cover %d rules, the catalog has %d", len(want), len(catalogRules()))
	}
	for _, rule := range catalogRules() {
		cells, listed := want[rule]
		if !listed {
			t.Errorf("no expectation for catalog rule %s", rule)
			continue
		}
		for _, role := range catalogRoles() {
			expected, known := cells[role]
			if !known {
				t.Errorf("%s has no expectation for role %s", rule, role)
				continue
			}
			if got := SeverityFor(role, rule, model.SevInfo); got != expected {
				t.Errorf("SeverityFor(%s, %s, info) = %s, want %s", role, rule, got, expected)
			}
		}
	}
}

// TestRoleRaisedCounts pins how many of the sixteen rules each role lifts above
// INFO: 4 for an issuer, 5 for a treasury, 4 for a distribution and a merchant
// account, 0 for other. A misspelled severity key drops its role's count even if
// the matrix above were edited to agree with the typo, so the two tests fail for
// different reasons and the count is what names the mistake.
func TestRoleRaisedCounts(t *testing.T) {
	t.Parallel()
	want := map[model.Role]int{
		model.RoleIssuer:       4,
		model.RoleTreasury:     5,
		model.RoleDistribution: 4,
		model.RoleMerchant:     4,
		model.RoleOther:        0,
	}
	for _, role := range catalogRoles() {
		raised := 0
		for _, rule := range catalogRules() {
			if SeverityFor(role, rule, model.SevInfo) > model.SevInfo {
				raised++
			}
		}
		if got := want[role]; got != raised {
			t.Errorf("role %s raises %d rules above info, want %d", role, raised, got)
		}
	}
}

// TestRoleFloorsNameOnlyCatalogRules is the direct answer to a misspelled key: a
// severity key that names no catalog rule is a typo, not a new rule, and it must
// fail here rather than sit in the map raising nothing.
func TestRoleFloorsNameOnlyCatalogRules(t *testing.T) {
	t.Parallel()
	known := make(map[string]bool, len(catalogRules()))
	for _, rule := range catalogRules() {
		known[rule] = true
	}
	for _, role := range catalogRoles() {
		for rule := range roleFloors(role) {
			if !known[rule] {
				t.Errorf("role %s has a severity key %q that names no rule in PRD.md 6.1", role, rule)
			}
		}
	}
}
