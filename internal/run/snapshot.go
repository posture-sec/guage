package run

import (
	"context"
	"fmt"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/source"
)

// SnapshotOptions carries the settings for recording a baseline.
type SnapshotOptions struct {
	ToolVersion string
	Concurrency int
	NewSource   SourceFactory
}

// Snapshot fetches the current state of accounts declared in cfg and constructs
// a model.Baseline containing their AccountSnapshots.
func Snapshot(ctx context.Context, cfg *policy.Config, opts SnapshotOptions) (model.Baseline, error) {
	if cfg == nil {
		return model.Baseline{}, fmt.Errorf("%w: no policy", ErrUsage)
	}
	if len(cfg.Endpoints) == 0 {
		return model.Baseline{}, fmt.Errorf("%w: no endpoint declared", ErrUsage)
	}
	if len(cfg.Accounts) == 0 {
		return model.Baseline{}, fmt.Errorf("%w: no accounts declared", ErrUsage)
	}

	factory := opts.NewSource
	if factory == nil {
		factory = horizonFactory(opts.ToolVersion)
	}
	src, err := factory(cfg.Endpoints[0])
	if err != nil {
		return model.Baseline{}, fmt.Errorf("snapshot: building source: %w", err)
	}

	info, err := src.Network(ctx)
	if err != nil {
		return model.Baseline{}, fmt.Errorf("snapshot: reading network: %w", err)
	}
	if err := source.CheckPassphrase(info, cfg.ExpectedPassphrase()); err != nil {
		return model.Baseline{}, fmt.Errorf("snapshot: network passphrase mismatch: %w", err)
	}

	observations, err := observe(ctx, src, cfg.Endpoints[0], cfg.Accounts, opts.Concurrency, false)
	if err != nil {
		return model.Baseline{}, fmt.Errorf("snapshot: fetching accounts: %w", err)
	}

	snapshots := make([]model.AccountSnapshot, len(observations))
	for i, obs := range observations {
		if obs.missing {
			return model.Baseline{}, fmt.Errorf("snapshot: account %s not found on %s", cfg.Accounts[i].ID, info.Passphrase)
		}
		s := obs.snapshot
		s.Network = info.Passphrase
		snapshots[i] = s
	}

	return model.Baseline{
		Version:     1,
		Banner:      model.BaselineBanner,
		ToolVersion: opts.ToolVersion,
		Network:     info.Passphrase,
		Ledger:      info.Ledger,
		Accounts:    snapshots,
	}, nil
}
