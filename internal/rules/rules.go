// Package rules holds the pure checks Gauge runs against an observed account.
// A rule reads only its Context: no I/O, no globals, no time and no
// randomness, so the same observation and policy always give the same
// findings.
package rules

import (
	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
)

// Context is everything one rule may read. Missing marks an account the
// endpoint reported as not found, and Mismatch marks an endpoint whose
// passphrase differs from the one the run requested; in the second case no
// snapshot is read at all.
type Context struct {
	Account  policy.Account
	Snapshot model.AccountSnapshot
	Missing  bool
	Mismatch bool
	Network  string
	Expected string
}

// Rule is one check. Its findings carry their own severity because a rule can
// raise severity on what it finds.
type Rule struct {
	ID    string
	Check func(Context) []model.Finding
}

// registry is the whole rule set. It is written once at package init and
// never mutated.
var registry = []Rule{
	{ID: "acct-missing", Check: accountMissing},
	{ID: "acct-network-mismatch", Check: accountNetworkMismatch},
	{ID: "flag-drift", Check: flagDrift},
	{ID: "flag-undeclared-clawback", Check: flagUndeclaredClawback},
}

// All returns the registered rules in evaluation order.
func All() []Rule {
	return append([]Rule(nil), registry...)
}

// Evaluate runs every rule against one observation.
func Evaluate(c Context) []model.Finding {
	var findings []model.Finding
	for _, rule := range registry {
		findings = append(findings, rule.Check(c)...)
	}
	return findings
}
