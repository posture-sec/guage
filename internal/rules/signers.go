package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/threshold"
)

// sigNoSigningWeight fires when nothing on the account can authorize anything:
// every entry is either weight 0 or a pre-authorized transaction hash, and
// neither is standing control (ARCHITECTURE.md 6.3). An issuer that declares
// expect_locked means this state, so it is INFO there rather than HIGH. The
// issuer row lists no floor for this rule, which is what lets the INFO survive
// task 3.4's adjustment; Appendix A never mentions it for a role.
func sigNoSigningWeight(c Context) []model.Finding {
	if c.Missing || c.Mismatch || len(participating(c.Snapshot.Signers)) > 0 {
		return nil
	}
	severity := model.SevHigh
	if policy.AllowsExpectLocked(c.Account.Role) && c.Account.ExpectLocked != nil && *c.Account.ExpectLocked {
		severity = model.SevInfo
	}
	return []model.Finding{model.NewFinding(
		"sig-no-signing-weight",
		severity,
		c.Snapshot.ID,
		"no signer can authorize an operation, so this account cannot sign anything again",
		describeSigners(c.Snapshot.Signers),
	)}
}

// sigSingleKeyHigh fires when one signer alone reaches the high threshold, the
// tier SetOptions itself belongs to, so a single key can rewrite the account's
// control. It emits the baseline INFO, which is Appendix A's `other` row: a plain
// `check` declares no role, lands there, and thresholds all zero mean every account
// with a single weighted key is in this shape. Task 3.4 raises it once per run, to
// HIGH for issuer and treasury and to WARN for distribution and merchant, so the
// role table is read at one place rather than restated inside each rule.
func sigSingleKeyHigh(c Context) []model.Finding {
	if c.Missing || c.Mismatch {
		return nil
	}
	high := c.Snapshot.Thresholds.High
	count, ok := threshold.MinSigners(c.Snapshot.Signers, high)
	if !ok || count != 1 {
		return nil
	}
	var alone []string
	reaches := fmt.Sprintf("reaches %d", high)
	if high <= 0 {
		// A tier of 0 still needs one matching signature, because the protocol
		// drops zero-weight candidates before any comparison happens
		// (ARCHITECTURE.md 6.3), but no weight is what satisfies it.
		reaches = "is enough on its own, because the high threshold is 0 and any single participating signature satisfies it"
	}
	for _, signer := range participating(c.Snapshot.Signers) {
		if threshold.Reachable([]model.Signer{signer}, high) {
			alone = append(alone, fmt.Sprintf("%s %s at weight %d %s", shortenKey(signer.Key), signer.Type, signer.Weight, reaches))
		}
	}
	return []model.Finding{model.NewFinding(
		"sig-single-key-high",
		model.SevInfo,
		c.Snapshot.ID,
		"one signer alone reaches the high threshold, so a single key holds the account",
		strings.Join(alone, "; "),
	)}
}

// sigHashX fires for every hash(x) signer. Spending one puts the preimage in a
// published envelope, so the secret is learned by anyone who can use the signer
// (ARCHITECTURE.md 6.3). It is HIGH when the entry's own weight reaches the
// medium threshold: at a threshold of 0 any single match authorizes the tier, so
// the weight is not what does the work and the finding stays WARN.
func sigHashX(c Context) []model.Finding {
	if c.Missing || c.Mismatch {
		return nil
	}
	var findings []model.Finding
	for _, signer := range c.Snapshot.Signers {
		if signer.Type != model.SignerHashX {
			continue
		}
		medium := c.Snapshot.Thresholds.Medium
		reachesMedium := medium > 0 && signer.Weight >= medium
		severity := model.SevWarn
		if reachesMedium {
			severity = model.SevHigh
		}
		findings = append(findings, model.NewFinding(
			"sig-hashx",
			severity,
			c.Snapshot.ID,
			"a hash(x) signer is a shared secret, and using it publishes that secret",
			fmt.Sprintf("%s at weight %d; medium threshold %d, reached by this signer alone: %t",
				shortenKey(signer.Key), signer.Weight, medium, reachesMedium),
		))
	}
	return findings
}

// sigUnexpectedSigner compares the observed list with a declared one. An
// undeclared account is not this rule's problem: nothing was said, so nothing
// can disagree. A declared weight of 0 states an intent to remove, and a signer
// the chain still shows at 0 is drift rather than an extra signer.
func sigUnexpectedSigner(c Context) []model.Finding {
	if c.Missing || c.Mismatch || c.Account.ExpectedSigners == nil {
		return nil
	}
	declared := map[string]int{}
	for _, expected := range *c.Account.ExpectedSigners {
		declared[expected.Key] = expected.Weight
	}
	var extra []string
	for _, signer := range c.Snapshot.Signers {
		want, known := declared[signer.Key]
		if !known || (want == 0 && signer.Weight > 0) {
			extra = append(extra, fmt.Sprintf("%s %s at weight %d is not declared", shortenKey(signer.Key), signer.Type, signer.Weight))
		}
	}
	if len(extra) == 0 {
		return nil
	}
	return []model.Finding{model.NewFinding(
		"sig-unexpected-signer",
		model.SevHigh,
		c.Snapshot.ID,
		"the account carries signers the policy does not declare",
		strings.Join(extra, "; "),
	)}
}

// sigMissingExpected reports a declared signer the chain no longer shows, or one
// it shows at weight 0 while the policy declares weight above 0: either way the
// control that was promised is not there.
func sigMissingExpected(c Context) []model.Finding {
	if c.Missing || c.Mismatch || c.Account.ExpectedSigners == nil {
		return nil
	}
	observed := map[string]int{}
	for _, signer := range c.Snapshot.Signers {
		observed[signer.Key] = signer.Weight
	}
	var missing []string
	for _, expected := range *c.Account.ExpectedSigners {
		weight, present := observed[expected.Key]
		if expected.Weight > 0 && (!present || weight == 0) {
			missing = append(missing, fmt.Sprintf("%s declared at weight %d, chain shows %d", shortenKey(expected.Key), expected.Weight, weight))
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return []model.Finding{model.NewFinding(
		"sig-missing-expected",
		model.SevHigh,
		c.Snapshot.ID,
		"a declared signer cannot authorize on this account",
		strings.Join(missing, "; "),
	)}
}

// sigWeightDrift compares the declared weight with the observed one for the
// signers both sides name. Additions and removals are the other two signer
// rules, so a key appears in one finding only.
func sigWeightDrift(c Context) []model.Finding {
	if c.Missing || c.Mismatch || c.Account.ExpectedSigners == nil {
		return nil
	}
	observed := map[string]model.Signer{}
	for _, signer := range c.Snapshot.Signers {
		observed[signer.Key] = signer
	}
	var diffs []string
	for _, expected := range *c.Account.ExpectedSigners {
		if signer, present := observed[expected.Key]; present && signer.Weight != expected.Weight {
			diffs = append(diffs, fmt.Sprintf("%s declared %d, chain shows %d", shortenKey(expected.Key), expected.Weight, signer.Weight))
		}
	}
	if len(diffs) == 0 {
		return nil
	}
	sort.Strings(diffs)
	return []model.Finding{model.NewFinding(
		"sig-weight-drift",
		model.SevWarn,
		c.Snapshot.ID,
		"a signer holds a different weight than the policy declares",
		strings.Join(diffs, "; "),
	)}
}

// participating returns the signers whose weight the protocol counts toward
// standing control: above zero, and not a pre-authorized transaction hash.
func participating(signers []model.Signer) []model.Signer {
	out := make([]model.Signer, 0, len(signers))
	for _, signer := range signers {
		if signer.Weight > 0 && signer.Type != model.SignerPreAuthTx {
			out = append(out, signer)
		}
	}
	return out
}

// describeSigners renders the observed list for evidence, sorted so the same
// account always produces the same string.
func describeSigners(signers []model.Signer) string {
	if len(signers) == 0 {
		return "the account carries no signers at all"
	}
	rendered := make([]string, 0, len(signers))
	for _, signer := range signers {
		rendered = append(rendered, fmt.Sprintf("%s %s at weight %d", shortenKey(signer.Key), signer.Type, signer.Weight))
	}
	sort.Strings(rendered)
	return strings.Join(rendered, "; ")
}

func shortenKey(key string) string {
	if len(key) <= 12 {
		return key
	}
	return key[:6] + "..." + key[len(key)-4:]
}
