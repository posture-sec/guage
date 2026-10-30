package run

import (
	"context"
	"fmt"
	"time"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/rules"
	"github.com/posture-sec/guage/internal/strkey"
)

// CheckRequest is a policy-free audit of accounts named on the command line.
type CheckRequest struct {
	ToolVersion string
	Network     string
	Endpoints   []string
	Accounts    []string
	FailOn      model.Severity

	Concurrency int
	NewSource   SourceFactory
	Evaluate    Evaluator
	Now         func() time.Time
}

// Check audits command-line accounts with the universal rules, using the role
// "other" and no declared expectations because there is no policy file.
func Check(ctx context.Context, req CheckRequest) (model.Report, error) {
	accounts := make([]policy.Account, 0, len(req.Accounts))
	for _, id := range req.Accounts {
		if !strkey.IsAccountID(id) {
			return model.Report{}, fmt.Errorf("%w: every account must be a G address", ErrUsage)
		}
		accounts = append(accounts, policy.Account{ID: id, Role: model.RoleOther})
	}
	named := policy.Config{Network: req.Network}
	evaluate := req.Evaluate
	if evaluate == nil {
		evaluate = rules.Evaluate
	}
	return Run(ctx, Job{
		ToolVersion: req.ToolVersion,
		Network:     named.ExpectedPassphrase(),
		Endpoints:   req.Endpoints,
		Accounts:    accounts,
		FailOn:      req.FailOn,
		Concurrency: req.Concurrency,
		NewSource:   req.NewSource,
		Evaluate:    evaluate,
		Now:         req.Now,
	})
}
