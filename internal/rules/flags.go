package rules

import (
	"fmt"
	"strings"

	"github.com/posture-sec/guage/internal/model"
)

// flagDrift compares only the flags the policy declares. An omitted flag means
// "not declared", never "false", so it cannot drift.
func flagDrift(c Context) []model.Finding {
	if c.Missing || c.Mismatch {
		return nil
	}
	want := c.Account.ExpectedFlags
	if want == nil {
		return nil
	}
	got := c.Snapshot.Flags

	var diffs []string
	compare := func(name string, expected *bool, observed bool) {
		if expected != nil && *expected != observed {
			diffs = append(diffs, fmt.Sprintf("%s declared %t, observed %t", name, *expected, observed))
		}
	}
	compare("auth_required", want.AuthRequired, got.AuthRequired)
	compare("auth_revocable", want.AuthRevocable, got.AuthRevocable)
	compare("auth_immutable", want.AuthImmutable, got.AuthImmutable)
	compare("clawback", want.ClawbackEnabled, got.ClawbackEnabled)
	if len(diffs) == 0 {
		return nil
	}
	return []model.Finding{model.NewFinding(
		"flag-drift",
		model.SevWarn,
		c.Snapshot.ID,
		"the declared flags differ from the flags the chain shows",
		strings.Join(diffs, "; "),
	)}
}

// flagUndeclaredClawback fires when clawback is on and the policy says nothing
// about it. A policy that declares clawback false is a declaration, and the
// disagreement is flag-drift instead.
func flagUndeclaredClawback(c Context) []model.Finding {
	if c.Missing || c.Mismatch || !c.Snapshot.Flags.ClawbackEnabled {
		return nil
	}
	if want := c.Account.ExpectedFlags; want != nil && want.ClawbackEnabled != nil {
		return nil
	}
	return []model.Finding{model.NewFinding(
		"flag-undeclared-clawback",
		model.SevWarn,
		c.Snapshot.ID,
		"clawback is enabled and the policy does not declare it",
		"clawback observed true, and no clawback expectation is declared",
	)}
}
