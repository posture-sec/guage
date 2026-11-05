package rules

import (
	"fmt"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
)

// idNoHomeDomain has two triggers, and PRD 6.1 phrases them as one sentence: an
// account whose role expects a home domain, or one a policy declared a domain for.
// The declaration is the expectation an operator wrote; the role is the expectation
// Appendix A wrote, which is why this rule could not fire in check mode before task
// 3.4 and still cannot: check declares no role, which lands on `other`, and the
// other row says nothing about publishing a domain.
func idNoHomeDomain(c Context) []model.Finding {
	if c.Missing || c.Mismatch || c.Snapshot.HasHomeDomain() {
		return nil
	}
	declared := c.Account.HomeDomain
	if declared != nil && *declared != "" {
		return []model.Finding{model.NewFinding(
			"id-no-home-domain",
			model.SevWarn,
			c.Snapshot.ID,
			"the policy declares a home domain and the account publishes none",
			fmt.Sprintf("home_domain declared %q, observed none", *declared),
		)}
	}
	// The second trigger is the one PRD.md 6.1 phrases by role and Appendix A's
	// issuer row alone asserts: an account whose own role is expected to publish a
	// domain and shows none is a finding whether or not a policy said so. `check`
	// declares no role and lands on `other`, which the appendix does not describe as
	// publishing anything, so this cannot fire outside `audit`.
	if policy.ExpectsHomeDomain(c.Account.Role) {
		return []model.Finding{model.NewFinding(
			"id-no-home-domain",
			model.SevWarn,
			c.Snapshot.ID,
			"the account's role expects a home domain and the account publishes none",
			fmt.Sprintf("role %q expects a home_domain, observed none", c.Account.Role),
		)}
	}
	return nil
}

// idTomlUnreachable fires when a stellar.toml could not be fetched or parsed under
// the guard, and equally when one was observed on the account and never fetched: an
// unverified identity claim is not a pass. A home domain that is blank after trimming
// is no domain at all, so the rule stays silent about it rather than reporting an
// unverified claim, and a document that is present but refuses validation is the
// opposite case and is reported. The evidence names the step, so a dead domain is
// distinguishable from a domain that serves a document Gauge cannot read. Nothing from
// the document is ever quoted; Detail is a phrase this repository wrote.
func idTomlUnreachable(c Context) []model.Finding {
	if c.Missing || c.Mismatch || !c.Snapshot.HasHomeDomain() {
		return nil
	}
	result := c.Identity
	if result == nil {
		return []model.Finding{model.NewFinding(
			"id-toml-unreachable",
			model.SevWarn,
			c.Snapshot.ID,
			fmt.Sprintf("the home domain %s was never fetched, so its identity claim is unverified", c.Snapshot.HomeDomain),
			fmt.Sprintf("step not-attempted; home domain %s", c.Snapshot.HomeDomain),
		)}
	}
	if result.Reachable() {
		return nil
	}
	return []model.Finding{model.NewFinding(
		"id-toml-unreachable",
		model.SevWarn,
		c.Snapshot.ID,
		fmt.Sprintf("the home domain %s does not serve a stellar.toml Gauge can read", result.HomeDomain),
		fmt.Sprintf("step %s; detail %s; status %d", result.FailedAt, result.Detail, result.Status),
	)}
}

// idTomlAccountUnlisted fires when a readable stellar.toml does not name this
// account. It only speaks about a document it could read, so an unreachable domain
// raises one finding rather than two.
//
// The document's own NETWORK_PASSPHRASE is not compared here. PRD 6.1 asks the
// identity rules to use it, but no rule ID covers a mismatch, an account address is
// valid on both networks, and inventing an ID or borrowing another rule's would be
// an owner decision. The value travels in the result so task 3.7 can print it.
func idTomlAccountUnlisted(c Context) []model.Finding {
	if c.Missing || c.Mismatch || !c.Snapshot.HasHomeDomain() {
		return nil
	}
	result := c.Identity
	if result == nil || !result.Reachable() {
		return nil
	}
	for _, account := range result.Identity.Accounts {
		if account == c.Snapshot.ID {
			return nil
		}
	}
	listed := len(result.Identity.Accounts)
	evidence := fmt.Sprintf("home domain %s lists %d %s",
		result.HomeDomain, listed, plural(listed, "account", "accounts"))
	if malformed := result.Identity.MalformedAccounts; malformed > 0 {
		evidence += fmt.Sprintf(" and %d %s that are not account addresses",
			malformed, plural(malformed, "entry", "entries"))
	}
	evidence += "; this account is not among them"
	return []model.Finding{model.NewFinding(
		"id-toml-account-unlisted",
		model.SevWarn,
		c.Snapshot.ID,
		"the account is not listed in its home domain's stellar.toml",
		evidence,
	)}
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
}
