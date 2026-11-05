package model

// AccountFacts is what a run read and what its policy declared, kept apart from
// the findings so a report can be read as observations first and judgements
// second. Nothing in it can fail a run: report.Exit reads findings only, so a
// fact carries no severity, joins no fingerprint and never becomes a risk an
// operator can accept.
//
// Each member is a pointer because each has a real "this run has no answer"
// state, and the two kinds of absence are different facts. Reachability is nil
// when no account state was read at all, as on a missing account or an endpoint
// on the wrong network. Identity is nil when no stellar.toml was fetched, which
// covers an account publishing no home domain and an observed domain the run
// never reached. Declared is nil when the run read no policy file, which is what
// separates a zero-config check, whose role is the default `other` rather than
// something an operator wrote, from an audit whose operator declared nothing.
//
// Every account a run reports carries one. The pointer on AccountReport is nil only
// for a report assembled by hand, and a renderer prints no fact lines for such an
// account rather than reading a missing block as an answer about the world.
type AccountFacts struct {
	Reachability *Reachability  `json:"reachability"`
	Declared     *DeclaredFacts `json:"declared"`
	Identity     *IdentityFacts `json:"identity"`
}

// Reachability is the smallest number of signatures that can authorize each
// threshold, taken from threshold.MinSigners over the participating weights the
// snapshot carries. A nil tier is a tier no combination of the account's signers
// can reach, which is the same computation sig-threshold-unreachable reports as a
// HIGH finding; it is printed here anyway because a report that named only the
// reachable tiers would describe an account as working.
type Reachability struct {
	Low    *int `json:"low"`
	Medium *int `json:"medium"`
	High   *int `json:"high"`
}

// DeclaredFacts is what a policy file said about one account. The counts and
// presence flags are the point: a reader who sees no drift finding needs to know
// whether the rules compared against a declaration or had nothing to compare
// with. An expected signer is counted rather than listed because a declaration
// can hold 21 keys and the drift findings name the ones that disagree.
type DeclaredFacts struct {
	Name               string              `json:"name,omitempty"`
	Role               string              `json:"role"`
	ExpectedSigners    *int                `json:"expected_signers"`
	ExpectedThresholds *DeclaredThresholds `json:"expected_thresholds"`
	ExpectedFlags      *DeclaredFlags      `json:"expected_flags"`
	HomeDomain         *string             `json:"home_domain"`
	ExpectLocked       *bool               `json:"expect_locked"`
	// ExpectLockedIgnored marks a true declaration on a role Appendix A does not
	// allow to make it. It is not refused at load, so the only place an operator
	// can learn that the line did nothing is this report.
	ExpectLockedIgnored bool `json:"expect_locked_ignored,omitempty"`
}

// DeclaredThresholds and DeclaredFlags keep the policy's pointers, so an omitted
// entry reads as null and an explicit zero or false reads as a value. The
// distinction is what the loader preserves and what the drift rules key on.
type DeclaredThresholds struct {
	Low    *int `json:"low"`
	Medium *int `json:"medium"`
	High   *int `json:"high"`
}

type DeclaredFlags struct {
	AuthRequired    *bool `json:"auth_required"`
	AuthRevocable   *bool `json:"auth_revocable"`
	AuthImmutable   *bool `json:"auth_immutable"`
	ClawbackEnabled *bool `json:"clawback"`
}

// IdentityFacts is what the guarded fetcher observed for one home domain. The
// document's body is never here: only the keys SEP-1 defines and the fixed phrase
// this repository wrote about the step that decided the outcome.
//
// CORSPresent is printed although no browser reads this report, because SEP-1
// mandates the header and its absence is a fact about the issuer's configuration
// rather than a finding Gauge would invent a rule for.
type IdentityFacts struct {
	HomeDomain  string `json:"home_domain"`
	Reachable   bool   `json:"reachable"`
	Step        string `json:"step"`
	Detail      string `json:"detail,omitempty"`
	Status      int    `json:"status"`
	Hops        int    `json:"hops"`
	Bytes       int    `json:"bytes"`
	CORSPresent bool   `json:"cors_present"`

	// NetworkPassphrase is what the document named, and NetworkMatches is whether
	// it is the network this run was told to audit. Matches is nil when the
	// document stated no passphrase, because silence and agreement are different.
	// No rule covers a mismatch: an account address is valid on both networks and
	// inventing a rule ID for it is an owner decision, so the disagreement is a
	// fact and the report is where an operator finds it.
	NetworkPassphrase *string  `json:"network_passphrase"`
	NetworkMatches    *bool    `json:"network_matches"`
	SigningKey        *string  `json:"signing_key"`
	HorizonURL        *string  `json:"horizon_url"`
	Version           *string  `json:"version"`
	Accounts          []string `json:"accounts"`
	MalformedAccounts int      `json:"malformed_accounts"`
}
