package rules

import (
	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/sep1"
	"github.com/posture-sec/guage/internal/threshold"
)

// Facts returns the observation and declaration lines a report prints ahead of
// its findings. It is as pure as a rule and reads the same Context: no I/O, no
// globals, no time and no randomness, so the same account and policy always print
// the same facts.
//
// Facts and findings are two readings of one observation, and the split is what
// lets a zero-config check say everything it saw while judging only by the
// universal rules. A rule speaks about one thing the catalog names; this speaks
// about the shapes the catalog has no row for: which tiers the account's weights
// can actually reach, what the home domain's document said, and which expectations
// the operator wrote down. Nothing returned here can become a finding, so nothing
// here can be accepted, escalated by a role or counted at exit.
//
// Declared is built from the account entry whether or not a policy file existed,
// because Context cannot tell the two apart and the run can: `check` synthesizes
// its entries with the default role, and internal/run drops this member for such a
// run rather than print a role the operator never declared.
func Facts(c Context) model.AccountFacts {
	facts := model.AccountFacts{
		Declared: declaredFacts(c.Account),
		Identity: identityFacts(c.Identity, c.Expected),
	}
	// A missing account and a mismatched endpoint have no signers and no stored
	// tiers, so the arithmetic has nothing to answer with and a nil is truer than
	// three zeros.
	if !c.Missing && !c.Mismatch {
		facts.Reachability = reachability(c.Snapshot)
	}
	return facts
}

// reachability reports the minimum participating signatures for the three tiers
// the account itself stores. It reads through threshold.MinSigners rather than
// restating the 20-signature cap, the zero-weight exclusion and the
// pre-authorization exclusion, which is the same reason every rule calls that
// package: one place for the arithmetic 0.7 verified.
func reachability(snapshot model.AccountSnapshot) *model.Reachability {
	minimum := func(tier int) *int {
		count, ok := threshold.MinSigners(snapshot.Signers, tier)
		if !ok {
			return nil
		}
		return &count
	}
	return &model.Reachability{
		Low:    minimum(snapshot.Thresholds.Low),
		Medium: minimum(snapshot.Thresholds.Medium),
		High:   minimum(snapshot.Thresholds.High),
	}
}

func declaredFacts(account policy.Account) *model.DeclaredFacts {
	facts := &model.DeclaredFacts{
		Name: account.Name,
		Role: string(account.Role),
	}
	if signers := account.ExpectedSigners; signers != nil {
		count := len(*signers)
		facts.ExpectedSigners = &count
	}
	if expected := account.ExpectedThresholds; expected != nil {
		facts.ExpectedThresholds = &model.DeclaredThresholds{
			Low: expected.Low, Medium: expected.Medium, High: expected.High,
		}
	}
	if expected := account.ExpectedFlags; expected != nil {
		facts.ExpectedFlags = &model.DeclaredFlags{
			AuthRequired: expected.AuthRequired, AuthRevocable: expected.AuthRevocable,
			AuthImmutable: expected.AuthImmutable, ClawbackEnabled: expected.ClawbackEnabled,
		}
	}
	facts.HomeDomain = account.HomeDomain
	facts.ExpectLocked = account.ExpectLocked
	// The rule that reads this gate keeps the account at HIGH; this says why the
	// declaration the operator wrote did not lower it.
	if locked := account.ExpectLocked; locked != nil && *locked {
		facts.ExpectLockedIgnored = !policy.AllowsExpectLocked(account.Role)
	}
	return facts
}

// identityFacts turns one fetch result into report lines. The fetcher's own
// HomeDomain is used rather than the snapshot's field, because it is the value the
// guard either validated or sanitized, and a name it refused is already clean text.
func identityFacts(result *sep1.Result, network string) *model.IdentityFacts {
	if result == nil {
		return nil
	}
	facts := &model.IdentityFacts{
		HomeDomain:  result.HomeDomain,
		Reachable:   result.Reachable(),
		Step:        result.FailedAt.String(),
		Detail:      result.Detail,
		Status:      result.Status,
		Hops:        result.Hops,
		Bytes:       result.Bytes,
		CORSPresent: result.CORSPresent,
		// Only a document that parsed says anything about its keys. Unparsed bytes
		// are not an absent NETWORK_PASSPHRASE, and printing the four as absent
		// would read like a configuration the issuer never served.
	}
	if !facts.Reachable {
		return facts
	}
	identity := result.Identity
	facts.NetworkPassphrase = identity.NetworkPassphrase
	if stated := identity.NetworkPassphrase; stated != nil {
		match := *stated == network
		facts.NetworkMatches = &match
	}
	facts.SigningKey = identity.SigningKey
	facts.HorizonURL = identity.HorizonURL
	facts.Version = identity.Version
	facts.Accounts = identity.Accounts
	facts.MalformedAccounts = identity.MalformedAccounts
	return facts
}
