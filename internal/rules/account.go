package rules

import "github.com/posture-sec/guage/internal/model"

func accountMissing(c Context) []model.Finding {
	if !c.Missing {
		return nil
	}
	return []model.Finding{model.NewFinding(
		"acct-missing",
		model.SevHigh,
		c.Snapshot.ID,
		"the account was not found on this endpoint, so it may never have been created or it was merged",
		"endpoint "+c.Snapshot.Endpoint+" returned not found",
	)}
}

func accountNetworkMismatch(c Context) []model.Finding {
	if !c.Mismatch {
		return nil
	}
	return []model.Finding{model.NewFinding(
		"acct-network-mismatch",
		model.SevHigh,
		c.Snapshot.ID,
		"the endpoint is on a different network than the run requested, so the account was not read",
		"expected "+c.Expected+", endpoint reports "+c.Network,
	)}
}
