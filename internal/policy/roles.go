package policy

import "github.com/posture-sec/guage/internal/model"

// roleSeverities is PRD Appendix A as data. The rows are read as floors: a role
// raises a finding to the severity its row names and never lowers one, so the
// three non-acceptable rules cannot be softened by a declaration of purpose, and
// a cell that equals the rule's own severity is written down anyway because it is
// what the appendix says rather than an inference from silence.
//
// Each row names the rules its prose names, and nothing more. Where a cell is
// missing, the appendix did not speak about that rule for that role, and raising
// it here would be an edit to Appendix A rather than a code decision.
var roleSeverities = map[model.Role]map[string]model.Severity{
	// "Single-key control is HIGH; flag drift is HIGH; unexpected signers are
	// HIGH; home domain expected; expect_locked allowed." The row names no
	// threshold or weight drift, so those stay at their baseline.
	// flag-undeclared-clawback is not named here in words, but PRD.md 6.1 names it
	// for this role in the catalog itself: "WARN (HIGH for issuer)". The catalog
	// speaks about this rule by ID and Appendix A speaks about flags generally, so
	// the row that could be wrong about one rule loses to the row that cannot.
	model.RoleIssuer: {
		"sig-single-key-high":      model.SevHigh,
		"flag-drift":               model.SevHigh,
		"flag-undeclared-clawback": model.SevHigh,
		"sig-unexpected-signer":    model.SevHigh,
	},
	// "Single-key control is HIGH; signer and threshold drift are HIGH." Signer
	// drift is read as the family that compares a declaration with the chain,
	// which is the three rules the words cover; sig-missing-expected already
	// emits HIGH, and the floor reading means naming it changes nothing today.
	model.RoleTreasury: {
		"sig-single-key-high":   model.SevHigh,
		"sig-missing-expected":  model.SevHigh,
		"sig-unexpected-signer": model.SevHigh,
		"sig-weight-drift":      model.SevHigh,
		"thr-drift":             model.SevHigh,
	},
	// "Single-key control is WARN; drift is WARN." A declared signer the chain no
	// longer shows is not drift, so this row leaves sig-missing-expected at HIGH.
	model.RoleDistribution: {
		"sig-single-key-high": model.SevWarn,
		"sig-weight-drift":    model.SevWarn,
		"thr-drift":           model.SevWarn,
		"flag-drift":          model.SevWarn,
	},
	// The merchant row is the distribution row for severity. Its memo-required
	// check is task 5.6, which is a new rule rather than a severity.
	model.RoleMerchant: {
		"sig-single-key-high": model.SevWarn,
		"sig-weight-drift":    model.SevWarn,
		"thr-drift":           model.SevWarn,
		"flag-drift":          model.SevWarn,
	},
	// "Facts and universal rules only; single-key control is INFO." This is the
	// row an undeclared account falls to, and `check` declares it.
	model.RoleOther: {
		"sig-single-key-high": model.SevInfo,
	},
}

// SeverityFor returns the severity a role demands for a rule, which is never
// below the severity the rule emitted. An unknown role or an unlisted rule comes
// back unchanged, so a rule that has no row is not silenced by one.
func SeverityFor(role model.Role, ruleID string, emitted model.Severity) model.Severity {
	if floor, listed := roleSeverities[role][ruleID]; listed && floor > emitted {
		return floor
	}
	return emitted
}

// ExpectsHomeDomain reports whether the role's own row says the account should
// publish a home domain. Only the issuer row does: a treasury, merchant or
// distribution account is not described as publishing one, and the other row is
// explicitly facts and universal rules.
func ExpectsHomeDomain(role model.Role) bool {
	return role == model.RoleIssuer
}

// AllowsExpectLocked reports whether the role may declare expect_locked, which is
// the intended black-hole shape only for an issuer that has stopped issuing. A
// declaration on another role is not refused at load, because refusing would make
// a typo in a declaration cost more than the declaration is worth: the rule reads
// it only here, so an ineffective declaration still leaves the account at HIGH and
// 3.7 can print what was declared.
func AllowsExpectLocked(role model.Role) bool {
	return role == model.RoleIssuer
}

// ExpectedPassphrase returns the network passphrase a run must see from its
// endpoint. The two named networks map to their published passphrases; any
// other value is the passphrase itself.
func (c *Config) ExpectedPassphrase() string {
	switch c.Network {
	case "mainnet":
		return "Public Global Stellar Network ; September 2015"
	case "testnet":
		return "Test SDF Network ; September 2015"
	default:
		return c.Network
	}
}
