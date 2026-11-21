package run

import (
	"context"
	"testing"
	"time"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/rules"
)

// A G address is the same key on mainnet and testnet, and a threshold reading can
// be identical on both. An acceptance written for one network therefore used to
// silence the other network's finding, with nothing in the report saying why. A
// named network pins the entry; an empty one keeps the meaning it had before the
// key existed and covers either network.
func TestAcceptanceNetworkScope(t *testing.T) {
	t.Parallel()
	const mainnet = "Public Global Stellar Network ; September 2015"
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		entry   string
		running string
		want    bool
	}{
		{"a testnet entry does not cover a mainnet finding", "testnet", mainnet, false},
		{"a mainnet entry does not cover a testnet finding", "mainnet", testnet, false},
		{"a pinned entry covers the network it names", "mainnet", mainnet, true},
		{"the passphrase names the same network as its alias", mainnet, mainnet, true},
		{"an unpinned entry still covers a mainnet run", "", mainnet, true},
		{"an unpinned entry still covers a testnet run", "", testnet, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fs := &fakeSource{
				info:      model.NetworkInfo{Passphrase: tc.running, Ledger: 1},
				snapshots: map[string]model.AccountSnapshot{accountOne: snapshot(accountOne, 1)},
			}
			job := baseJob(fs)
			job.Network = tc.running
			job.Accounts = job.Accounts[:1]
			job.Now = func() time.Time { return now }
			job.Accepts = []policy.Acceptance{{
				Rule:    "flag-drift",
				Account: accountOne,
				Network: tc.entry,
				Reason:  "the flag is what the integrator asked for",
			}}
			job.Evaluate = func(c rules.Context) []model.Finding {
				return []model.Finding{model.NewFinding(
					"flag-drift", model.SevHigh, c.Snapshot.ID,
					"a declared flag is not what the chain shows",
					"auth_required declared false, observed true",
				)}
			}

			result, err := Run(context.Background(), job)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if len(result.Accounts) != 1 || len(result.Accounts[0].Findings) != 1 {
				t.Fatalf("findings = %+v, want the one drift finding", result.Accounts)
			}
			if got := result.Accounts[0].Findings[0].Accepted; got != tc.want {
				t.Errorf("accepted = %v, want %v", got, tc.want)
			}
			wantExit := model.ExitFindings
			if tc.want {
				wantExit = model.ExitPass
			}
			if result.ExitCode != wantExit {
				t.Errorf("exit = %d, want %d", result.ExitCode, wantExit)
			}
		})
	}
}
