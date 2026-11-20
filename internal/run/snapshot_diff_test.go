package run

import (
	"context"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/source"
)

func TestSnapshotAndDiff(t *testing.T) {
	fake := &fakeSource{
		info: model.NetworkInfo{
			Passphrase: testnet,
			Ledger:     5000000,
		},
		snapshots: map[string]model.AccountSnapshot{
			accountOne: {
				ID:       accountOne,
				Network:  testnet,
				Ledger:   5000000,
				Sequence: "1",
				Thresholds: model.Thresholds{
					Low:    1,
					Medium: 2,
					High:   2,
				},
				Signers: []model.Signer{
					{Key: accountOne, Type: model.SignerEd25519, Weight: 2},
				},
				Flags:      model.Flags{AuthRequired: true},
				HomeDomain: "example.com",
			},
		},
	}

	cfg := &policy.Config{
		Network:   "testnet",
		Endpoints: []string{"https://horizon-testnet.stellar.org"},
		Accounts: []policy.Account{
			{ID: accountOne, Role: model.RoleTreasury},
		},
	}

	// 1. Take snapshot
	baseline, err := Snapshot(context.Background(), cfg, SnapshotOptions{
		ToolVersion: "v0.1.0",
		NewSource: func(string) (source.Source, error) {
			return fake, nil
		},
	})
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	if baseline.Banner != model.BaselineBanner {
		t.Errorf("expected banner %q, got %q", model.BaselineBanner, baseline.Banner)
	}
	if baseline.Ledger != 5000000 {
		t.Errorf("expected ledger 5000000, got %d", baseline.Ledger)
	}
	if len(baseline.Accounts) != 1 {
		t.Fatalf("expected 1 account in baseline, got %d", len(baseline.Accounts))
	}

	// 2. Diff against identical state -> no diff, ExitPass
	diffReport, err := Diff(context.Background(), cfg, baseline, DiffOptions{
		ToolVersion: "v0.1.0",
		NewSource: func(string) (source.Source, error) {
			return fake, nil
		},
	})
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diffReport.HasDiff {
		t.Errorf("expected no diff on identical state")
	}
	if diffReport.ExitCode != model.ExitPass {
		t.Errorf("expected ExitPass (0), got %v", diffReport.ExitCode)
	}

	// 3. Diff after chain modification: threshold change, signer added, signer weight changed
	fakeDrift := &fakeSource{
		info: model.NetworkInfo{
			Passphrase: testnet,
			Ledger:     5000100,
		},
		snapshots: map[string]model.AccountSnapshot{
			accountOne: {
				ID:       accountOne,
				Network:  testnet,
				Ledger:   5000100,
				Sequence: "2",
				Thresholds: model.Thresholds{
					Low:    1,
					Medium: 3, // Changed from 2 to 3
					High:   3, // Changed from 2 to 3
				},
				Signers: []model.Signer{
					{Key: accountOne, Type: model.SignerEd25519, Weight: 1}, // Changed from 2 to 1
					{Key: accountTwo, Type: model.SignerEd25519, Weight: 2}, // Added
				},
				Flags:      model.Flags{AuthRequired: false}, // Changed from true to false
				HomeDomain: "new-domain.com",                 // Changed from example.com
			},
		},
	}

	driftReport, err := Diff(context.Background(), cfg, baseline, DiffOptions{
		ToolVersion: "v0.1.0",
		NewSource: func(string) (source.Source, error) {
			return fakeDrift, nil
		},
	})
	if err != nil {
		t.Fatalf("Diff with drift failed: %v", err)
	}
	if !driftReport.HasDiff {
		t.Fatalf("expected HasDiff true, got false")
	}
	if driftReport.ExitCode != model.ExitFindings {
		t.Errorf("expected ExitFindings (1), got %v", driftReport.ExitCode)
	}
	if len(driftReport.Accounts) != 1 {
		t.Fatalf("expected 1 account diff, got %d", len(driftReport.Accounts))
	}
	ad := driftReport.Accounts[0]
	if len(ad.Thresholds) != 2 {
		t.Errorf("expected 2 threshold changes, got %d", len(ad.Thresholds))
	}
	if len(ad.Signers) != 2 {
		t.Errorf("expected 2 signer changes (1 added, 1 weight_changed), got %d", len(ad.Signers))
	}
	if len(ad.Flags) != 1 {
		t.Errorf("expected 1 flag change, got %d", len(ad.Flags))
	}
	if ad.HomeDomain == nil || ad.HomeDomain.New != "new-domain.com" {
		t.Errorf("expected home domain change to new-domain.com, got %v", ad.HomeDomain)
	}
}
