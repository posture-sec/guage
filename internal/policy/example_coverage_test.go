package policy

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// declaredRoles is every role internal/model declares. An example policy role is
// checked against this list rather than against any string, so a typo or a role
// the loader would reject cannot pass as coverage.
var declaredRoles = []model.Role{
	model.RoleIssuer,
	model.RoleTreasury,
	model.RoleDistribution,
	model.RoleMerchant,
	model.RoleOther,
}

// TestExamplePolicyCoversEveryExpectationKind keeps gauge.example.toml level with
// the schema. The example is the file the documentation tells a new user to copy,
// so a sixth expectation field added to Account must arrive in the example in the
// same change, and dropping one field inside expected_thresholds or
// expected_flags is the same drift one level down. TestExamplePolicyLoads stays
// the loader's own check; this one is about the declaration surface, and it reads
// the shipped file through LoadFile rather than the unexported declaredFacts so it
// pins what a copied policy actually declares.
func TestExamplePolicyCoversEveryExpectationKind(t *testing.T) {
	t.Parallel()
	cfg, err := LoadFile("../../gauge.example.toml")
	if err != nil {
		t.Fatalf("gauge.example.toml: %v", err)
	}
	if len(cfg.Accounts) != 1 {
		t.Fatalf("the example must declare exactly one account to be a coverage check, got %d", len(cfg.Accounts))
	}
	account := cfg.Accounts[0]

	allowed := make(map[model.Role]bool, len(declaredRoles))
	for _, role := range declaredRoles {
		allowed[role] = true
	}
	if !allowed[account.Role] {
		t.Errorf("example role %q is not one of the five declared roles", account.Role)
	}

	if account.ExpectedSigners == nil {
		t.Error("expected_signers is not declared in the example")
	}
	if account.ExpectedThresholds == nil {
		t.Error("expected_thresholds is not declared in the example")
	} else if account.ExpectedThresholds.Low == nil || account.ExpectedThresholds.Medium == nil || account.ExpectedThresholds.High == nil {
		t.Errorf("expected_thresholds must declare low, medium and high; got %+v", account.ExpectedThresholds)
	}
	if account.ExpectedFlags == nil {
		t.Error("expected_flags is not declared in the example")
	} else if account.ExpectedFlags.AuthRequired == nil || account.ExpectedFlags.AuthRevocable == nil ||
		account.ExpectedFlags.AuthImmutable == nil || account.ExpectedFlags.ClawbackEnabled == nil {
		t.Errorf("expected_flags must declare all four flags; got %+v", account.ExpectedFlags)
	}
	if account.HomeDomain == nil {
		t.Error("home_domain is not declared in the example")
	}
	if account.ExpectLocked == nil {
		t.Error("expect_locked is not declared in the example")
	}
}
