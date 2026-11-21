package run

import (
	"context"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/rules"
	"github.com/posture-sec/guage/internal/sep1"
)

type countingIdentity struct {
	calls   int
	fetcher sep1.Fetcher
}

func (c *countingIdentity) factory() (sep1.Fetcher, error) {
	c.calls++
	return c.fetcher, nil
}

func TestIdentityFetcherBuiltOnlyWhenNeeded(t *testing.T) {
	t.Parallel()
	accounts := []policy.Account{
		{ID: accountOne, Role: model.RoleOther},
		{ID: accountTwo, Role: model.RoleOther},
	}
	cases := []struct {
		name         string
		snapshots    map[string]model.AccountSnapshot
		fetches      map[string]sep1.Result
		wantBuilds   int
		wantFetched  []string
		wantFindings [][]string
	}{
		{
			name:       "all missing",
			snapshots:  nil,
			wantBuilds: 0,
			wantFindings: [][]string{
				{"acct-missing"},
				{"acct-missing"},
			},
		},
		{
			name: "all domainless",
			snapshots: map[string]model.AccountSnapshot{
				accountOne: snapshot(accountOne, 1),
				accountTwo: snapshot(accountTwo, 2),
			},
			wantBuilds:   0,
			wantFindings: [][]string{{}, {}},
		},
		{
			name: "one with a domain",
			snapshots: map[string]model.AccountSnapshot{
				accountOne: withDomain(snapshot(accountOne, 1), "example.org"),
				accountTwo: snapshot(accountTwo, 2),
			},
			fetches:      map[string]sep1.Result{"example.org": reachable("example.org")},
			wantBuilds:   1,
			wantFetched:  []string{"example.org"},
			wantFindings: [][]string{{"id-toml-account-unlisted"}, {}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fs := &fakeSource{
				info:      model.NetworkInfo{Passphrase: testnet, Ledger: 1},
				snapshots: tc.snapshots,
			}
			fetches := &fakeIdentity{results: tc.fetches}
			counter := &countingIdentity{fetcher: fetches}
			job := baseJob(fs)
			job.Accounts = accounts
			job.NewIdentity = counter.factory
			job.Evaluate = rules.Evaluate

			result, err := Run(context.Background(), job)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if counter.calls != tc.wantBuilds {
				t.Fatalf("identity factory calls = %d, want %d", counter.calls, tc.wantBuilds)
			}
			gotFetched := fetches.fetched()
			if len(gotFetched) != len(tc.wantFetched) {
				t.Fatalf("identity fetches = %v, want %v", gotFetched, tc.wantFetched)
			}
			for i := range tc.wantFetched {
				if gotFetched[i] != tc.wantFetched[i] {
					t.Fatalf("identity fetches = %v, want %v", gotFetched, tc.wantFetched)
				}
			}
			if len(result.Accounts) != len(tc.wantFindings) {
				t.Fatalf("accounts = %d, want %d", len(result.Accounts), len(tc.wantFindings))
			}
			for i, want := range tc.wantFindings {
				got := make([]string, len(result.Accounts[i].Findings))
				for j, finding := range result.Accounts[i].Findings {
					got[j] = finding.RuleID
				}
				if len(got) != len(want) {
					t.Fatalf("account %d findings = %v, want %v", i, got, want)
				}
				for j := range want {
					if got[j] != want[j] {
						t.Fatalf("account %d findings = %v, want %v", i, got, want)
					}
				}
			}
		})
	}
}
