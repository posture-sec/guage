package rules

import (
	"fmt"
	"strings"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/threshold"
)

// sigThresholdUnreachable reports a tier the account can never authorize.
// SetOptions validates ranges only, so an unreachable tier is a state the
// ledger stores rather than a mutation the protocol refused (ARCHITECTURE.md
// 6.3). It is on the non-acceptable list in policy, so no [[accept]] entry can
// silence it.
func sigThresholdUnreachable(c Context) []model.Finding {
	if c.Missing || c.Mismatch {
		return nil
	}
	tiers := []struct {
		name  string
		value int
	}{
		{"low", c.Snapshot.Thresholds.Low},
		{"medium", c.Snapshot.Thresholds.Medium},
		{"high", c.Snapshot.Thresholds.High},
	}
	var unreachable []string
	for _, tier := range tiers {
		if !threshold.Reachable(c.Snapshot.Signers, tier.value) {
			unreachable = append(unreachable, fmt.Sprintf("%s threshold %d", tier.name, tier.value))
		}
	}
	if len(unreachable) == 0 {
		return nil
	}
	return []model.Finding{model.NewFinding(
		"sig-threshold-unreachable",
		model.SevHigh,
		c.Snapshot.ID,
		"a threshold tier cannot be authorized by the signers this account has",
		strings.Join(unreachable, " and ")+" cannot be reached; signers: "+describeSigners(c.Snapshot.Signers),
	)}
}

// sigThresholdOrder reports thresholds that do not satisfy low <= medium <= high.
// The protocol does not enforce that ordering and SetOptions accepts the state,
// which is why the recommendation lives here as a WARN rather than as a claim
// about what the ledger rejected.
func sigThresholdOrder(c Context) []model.Finding {
	if c.Missing || c.Mismatch {
		return nil
	}
	thresholds := c.Snapshot.Thresholds
	if thresholds.Low <= thresholds.Medium && thresholds.Medium <= thresholds.High {
		return nil
	}
	return []model.Finding{model.NewFinding(
		"sig-threshold-order",
		model.SevWarn,
		c.Snapshot.ID,
		"the thresholds are out of the recommended order, so a lower tier can demand more than a higher one",
		fmt.Sprintf("low %d, medium %d, high %d", thresholds.Low, thresholds.Medium, thresholds.High),
	)}
}

// thrDrift compares only the tiers the policy declares. An omitted tier means
// "not declared", never "zero", so it cannot drift.
func thrDrift(c Context) []model.Finding {
	if c.Missing || c.Mismatch || c.Account.ExpectedThresholds == nil {
		return nil
	}
	want := c.Account.ExpectedThresholds
	got := c.Snapshot.Thresholds
	var diffs []string
	compare := func(name string, expected *int, observed int) {
		if expected != nil && *expected != observed {
			diffs = append(diffs, fmt.Sprintf("%s declared %d, chain shows %d", name, *expected, observed))
		}
	}
	compare("low", want.Low, got.Low)
	compare("medium", want.Medium, got.Medium)
	compare("high", want.High, got.High)
	if len(diffs) == 0 {
		return nil
	}
	return []model.Finding{model.NewFinding(
		"thr-drift",
		model.SevWarn,
		c.Snapshot.ID,
		"the declared thresholds differ from the thresholds the chain shows",
		strings.Join(diffs, "; "),
	)}
}
