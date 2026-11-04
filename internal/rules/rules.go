// Package rules holds the pure checks Gauge runs against an observed account.
// A rule reads only its Context: no I/O, no globals, no time and no
// randomness, so the same observation and policy always give the same
// findings.
package rules

import (
	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/sep1"
)

// Context is everything one rule may read. Missing marks an account the
// endpoint reported as not found, and Mismatch marks an endpoint whose
// passphrase differs from the one the run requested; in the second case no
// snapshot is read at all. Identity is what the guarded fetcher observed for
// the account's home domain, or nil when nothing was fetched: a rule reads a
// result, it never goes and gets one.
type Context struct {
	Account  policy.Account
	Snapshot model.AccountSnapshot
	Identity *sep1.Result
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
	{ID: "sig-no-signing-weight", Check: sigNoSigningWeight},
	{ID: "sig-threshold-unreachable", Check: sigThresholdUnreachable},
	{ID: "sig-threshold-order", Check: sigThresholdOrder},
	{ID: "sig-single-key-high", Check: sigSingleKeyHigh},
	{ID: "sig-hashx", Check: sigHashX},
	{ID: "sig-unexpected-signer", Check: sigUnexpectedSigner},
	{ID: "sig-missing-expected", Check: sigMissingExpected},
	{ID: "sig-weight-drift", Check: sigWeightDrift},
	{ID: "thr-drift", Check: thrDrift},
	{ID: "flag-drift", Check: flagDrift},
	{ID: "flag-undeclared-clawback", Check: flagUndeclaredClawback},
	{ID: "id-no-home-domain", Check: idNoHomeDomain},
	{ID: "id-toml-unreachable", Check: idTomlUnreachable},
	{ID: "id-toml-account-unlisted", Check: idTomlAccountUnlisted},
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
