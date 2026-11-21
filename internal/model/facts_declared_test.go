package model

import (
	"encoding/json"
	"testing"
)

// TestRoleIsValid pins the closed set: the five names a policy may load and
// nothing else. A value outside it must be refused rather than printed with a
// real role's authority.
func TestRoleIsValid(t *testing.T) {
	t.Parallel()
	for _, role := range []Role{RoleIssuer, RoleTreasury, RoleDistribution, RoleMerchant, RoleOther} {
		if !role.IsValid() {
			t.Errorf("Role(%q).IsValid() = false, want true", role)
		}
	}
	for _, role := range []Role{"", "isuer", "Issuer", "issuer ", "unknown"} {
		if role.IsValid() {
			t.Errorf("Role(%q).IsValid() = true, want false", role)
		}
	}
}

// TestDeclaredFactsRoleJSON proves the field keeps its wire shape: the value is
// the role name and nothing else changes now that the field is a Role.
func TestDeclaredFactsRoleJSON(t *testing.T) {
	t.Parallel()
	for _, role := range []Role{RoleIssuer, RoleTreasury, RoleDistribution, RoleMerchant, RoleOther} {
		data, err := json.Marshal(DeclaredFacts{Role: role})
		if err != nil {
			t.Fatalf("marshal role %q: %v", role, err)
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("unmarshal role %q: %v", role, err)
		}
		want := `"` + string(role) + `"`
		if got := string(raw["role"]); got != want {
			t.Fatalf("role = %s, want %s", got, want)
		}
	}
}
