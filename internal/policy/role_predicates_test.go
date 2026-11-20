package policy

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// TestRolePredicatesReadOneRow pins the table both predicates read. Membership is
// asserted explicitly because a missing role and a row that deliberately says
// false are the same thing to a map lookup: only the presence check tells a
// future role apart from one nobody remembered to write down.
func TestRolePredicatesReadOneRow(t *testing.T) {
	t.Parallel()
	for _, role := range declaredRoles {
		if _, ok := roleTraitsByRole[role]; !ok {
			t.Errorf("role %q is declared with no row in the trait table", role)
		}
	}
	if len(roleTraitsByRole) != len(declaredRoles) {
		t.Errorf("trait table holds %d rows for %d declared roles", len(roleTraitsByRole), len(declaredRoles))
	}
	for role, row := range roleTraitsByRole {
		if got := ExpectsHomeDomain(role); got != row.homeDomain {
			t.Errorf("ExpectsHomeDomain(%q) = %t, row says %t", role, got, row.homeDomain)
		}
		if got := AllowsExpectLocked(role); got != row.expectLocked {
			t.Errorf("AllowsExpectLocked(%q) = %t, row says %t", role, got, row.expectLocked)
		}
	}

	// Today's values, which the refactor must not move: the issuer row is the only
	// one Appendix A describes as expecting a domain and allowed to lock, and the
	// four other roles plus the empty string carry neither trait.
	if !ExpectsHomeDomain(model.RoleIssuer) || !AllowsExpectLocked(model.RoleIssuer) {
		t.Error("the issuer row must carry both traits")
	}
	for _, role := range []model.Role{model.RoleTreasury, model.RoleDistribution, model.RoleMerchant, model.RoleOther, ""} {
		if ExpectsHomeDomain(role) {
			t.Errorf("role %q is not described as publishing a home domain", role)
		}
		if AllowsExpectLocked(role) {
			t.Errorf("role %q is not described as allowed to lock", role)
		}
	}

	// A role the table never names falls back to the zero row.
	for _, role := range []model.Role{"custodian", ""} {
		if ExpectsHomeDomain(role) || AllowsExpectLocked(role) {
			t.Errorf("unknown role %q must be false for both traits", role)
		}
	}
}
