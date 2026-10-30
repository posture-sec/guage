package run

import (
	"context"
	"fmt"
	"time"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/rules"
)

// AuditOptions carries the run settings that a policy file does not declare.
type AuditOptions struct {
	ToolVersion string
	FailOn      model.Severity

	Concurrency int
	NewSource   SourceFactory
	Evaluate    Evaluator
	Now         func() time.Time
}

// Audit audits the accounts declared in a loaded policy, with its roles,
// expectations and accepted risks.
func Audit(ctx context.Context, cfg *policy.Config, opts AuditOptions) (model.Report, error) {
	if cfg == nil {
		return model.Report{}, fmt.Errorf("%w: no policy", ErrUsage)
	}
	evaluate := opts.Evaluate
	if evaluate == nil {
		evaluate = rules.Evaluate
	}
	return Run(ctx, Job{
		ToolVersion: opts.ToolVersion,
		Network:     cfg.ExpectedPassphrase(),
		Endpoints:   cfg.Endpoints,
		Accounts:    cfg.Accounts,
		Accepts:     cfg.Accepts,
		FailOn:      opts.FailOn,
		Concurrency: opts.Concurrency,
		NewSource:   opts.NewSource,
		Evaluate:    evaluate,
		Now:         opts.Now,
	})
}
