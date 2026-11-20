package run

import (
	"context"
	"fmt"
	"sort"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/source"
)

// DiffOptions carries settings for comparing current chain state against a baseline.
type DiffOptions struct {
	ToolVersion string
	Concurrency int
	NewSource   SourceFactory
}

// Diff compares live account state against a saved model.Baseline.
func Diff(ctx context.Context, cfg *policy.Config, baseline model.Baseline, opts DiffOptions) (model.DiffReport, error) {
	if cfg == nil {
		return model.DiffReport{}, fmt.Errorf("%w: no policy", ErrUsage)
	}
	if len(cfg.Endpoints) == 0 {
		return model.DiffReport{}, fmt.Errorf("%w: no endpoint declared", ErrUsage)
	}
	if len(cfg.Accounts) == 0 {
		return model.DiffReport{}, fmt.Errorf("%w: no accounts declared", ErrUsage)
	}

	if baseline.Version != 1 {
		return model.DiffReport{}, fmt.Errorf("%w: unsupported baseline schema version %d", ErrUsage, baseline.Version)
	}
	if baseline.Network != cfg.ExpectedPassphrase() {
		return model.DiffReport{}, fmt.Errorf("%w: baseline network %q differs from policy network %q",
			ErrUsage, baseline.Network, cfg.ExpectedPassphrase())
	}

	factory := opts.NewSource
	if factory == nil {
		factory = horizonFactory(opts.ToolVersion)
	}
	src, err := factory(cfg.Endpoints[0])
	if err != nil {
		return model.DiffReport{}, fmt.Errorf("diff: building source: %w", err)
	}

	info, err := src.Network(ctx)
	if err != nil {
		return model.DiffReport{}, fmt.Errorf("diff: reading network: %w", err)
	}
	if err := source.CheckPassphrase(info, cfg.ExpectedPassphrase()); err != nil {
		return model.DiffReport{}, fmt.Errorf("diff: network passphrase mismatch: %w", err)
	}

	observations, err := observe(ctx, src, cfg.Endpoints[0], cfg.Accounts, opts.Concurrency, false)
	if err != nil {
		return model.DiffReport{}, fmt.Errorf("diff: fetching accounts: %w", err)
	}

	baselineAccounts := make(map[string]model.AccountSnapshot, len(baseline.Accounts))
	for _, b := range baseline.Accounts {
		baselineAccounts[b.ID] = b
	}

	report := model.DiffReport{
		Banner:         model.BaselineBanner,
		ToolVersion:    opts.ToolVersion,
		Network:        info.Passphrase,
		BaselineLedger: baseline.Ledger,
		CurrentLedger:  info.Ledger,
		Accounts:       make([]model.AccountDiff, 0, len(cfg.Accounts)),
	}

	hasDiff := false
	for i, obs := range observations {
		accountID := cfg.Accounts[i].ID
		baseSnap, inBase := baselineAccounts[accountID]

		acctDiff := model.AccountDiff{
			AccountID: accountID,
		}

		if obs.missing {
			acctDiff.Missing = true
			acctDiff.Unbaselined = !inBase
			hasDiff = true
			report.Accounts = append(report.Accounts, acctDiff)
			continue
		}

		if !inBase {
			acctDiff.Unbaselined = true
			hasDiff = true
			report.Accounts = append(report.Accounts, acctDiff)
			continue
		}

		currSnap := obs.snapshot
		currSnap.Network = info.Passphrase

		// Compare Thresholds
		if currSnap.Thresholds.Low != baseSnap.Thresholds.Low {
			acctDiff.Thresholds = append(acctDiff.Thresholds, model.ThresholdDiff{
				Tier: "low",
				Old:  baseSnap.Thresholds.Low,
				New:  currSnap.Thresholds.Low,
			})
		}
		if currSnap.Thresholds.Medium != baseSnap.Thresholds.Medium {
			acctDiff.Thresholds = append(acctDiff.Thresholds, model.ThresholdDiff{
				Tier: "medium",
				Old:  baseSnap.Thresholds.Medium,
				New:  currSnap.Thresholds.Medium,
			})
		}
		if currSnap.Thresholds.High != baseSnap.Thresholds.High {
			acctDiff.Thresholds = append(acctDiff.Thresholds, model.ThresholdDiff{
				Tier: "high",
				Old:  baseSnap.Thresholds.High,
				New:  currSnap.Thresholds.High,
			})
		}

		// Compare Signers
		baseSigners := make(map[string]model.Signer, len(baseSnap.Signers))
		for _, s := range baseSnap.Signers {
			baseSigners[s.Key] = s
		}
		currSigners := make(map[string]model.Signer, len(currSnap.Signers))
		for _, s := range currSnap.Signers {
			currSigners[s.Key] = s
		}

		for key, cSigner := range currSigners {
			bSigner, exists := baseSigners[key]
			if !exists {
				acctDiff.Signers = append(acctDiff.Signers, model.SignerDiff{
					Key:    key,
					Type:   cSigner.Type,
					Weight: cSigner.Weight,
					Kind:   "added",
				})
			} else if bSigner.Weight != cSigner.Weight || bSigner.Type != cSigner.Type {
				acctDiff.Signers = append(acctDiff.Signers, model.SignerDiff{
					Key:       key,
					Type:      cSigner.Type,
					Weight:    cSigner.Weight,
					OldWeight: bSigner.Weight,
					Kind:      "weight_changed",
				})
			}
		}
		for key, bSigner := range baseSigners {
			if _, exists := currSigners[key]; !exists {
				acctDiff.Signers = append(acctDiff.Signers, model.SignerDiff{
					Key:    key,
					Type:   bSigner.Type,
					Weight: bSigner.Weight,
					Kind:   "removed",
				})
			}
		}
		sort.Slice(acctDiff.Signers, func(i, j int) bool {
			return acctDiff.Signers[i].Key < acctDiff.Signers[j].Key
		})

		// Compare Flags
		if currSnap.Flags.AuthRequired != baseSnap.Flags.AuthRequired {
			acctDiff.Flags = append(acctDiff.Flags, model.FlagDiff{
				Flag: "auth_required",
				Old:  baseSnap.Flags.AuthRequired,
				New:  currSnap.Flags.AuthRequired,
			})
		}
		if currSnap.Flags.AuthRevocable != baseSnap.Flags.AuthRevocable {
			acctDiff.Flags = append(acctDiff.Flags, model.FlagDiff{
				Flag: "auth_revocable",
				Old:  baseSnap.Flags.AuthRevocable,
				New:  currSnap.Flags.AuthRevocable,
			})
		}
		if currSnap.Flags.AuthImmutable != baseSnap.Flags.AuthImmutable {
			acctDiff.Flags = append(acctDiff.Flags, model.FlagDiff{
				Flag: "auth_immutable",
				Old:  baseSnap.Flags.AuthImmutable,
				New:  currSnap.Flags.AuthImmutable,
			})
		}
		if currSnap.Flags.ClawbackEnabled != baseSnap.Flags.ClawbackEnabled {
			acctDiff.Flags = append(acctDiff.Flags, model.FlagDiff{
				Flag: "clawback_enabled",
				Old:  baseSnap.Flags.ClawbackEnabled,
				New:  currSnap.Flags.ClawbackEnabled,
			})
		}

		// Compare Home Domain
		if currSnap.HomeDomain != baseSnap.HomeDomain {
			acctDiff.HomeDomain = &model.DomainDiff{
				Old: baseSnap.HomeDomain,
				New: currSnap.HomeDomain,
			}
		}

		if len(acctDiff.Thresholds) > 0 || len(acctDiff.Signers) > 0 ||
			len(acctDiff.Flags) > 0 || acctDiff.HomeDomain != nil {
			hasDiff = true
		}

		report.Accounts = append(report.Accounts, acctDiff)
	}

	report.HasDiff = hasDiff
	if hasDiff {
		report.ExitCode = model.ExitFindings
	} else {
		report.ExitCode = model.ExitPass
	}
	return report, nil
}
