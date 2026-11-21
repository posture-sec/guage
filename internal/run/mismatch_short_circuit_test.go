package run

import (
	"context"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/rules"
	"github.com/posture-sec/guage/internal/sep1"
)

type mismatchTripwireSource struct {
	fakeSource
	t *testing.T
}

func (s *mismatchTripwireSource) Account(context.Context, string) (model.AccountSnapshot, error) {
	s.t.Fatal("account source was called for a network mismatch")
	return model.AccountSnapshot{}, nil
}

func TestMismatchSkipsIdentityFetchToo(t *testing.T) {
	t.Parallel()
	fs := &mismatchTripwireSource{
		fakeSource: fakeSource{info: model.NetworkInfo{Passphrase: "Public Global Stellar Network ; September 2015", Ledger: 1}},
		t:          t,
	}
	newIdentity := func() (sep1.Fetcher, error) {
		t.Fatal("identity factory was called for a network mismatch")
		return nil, nil
	}
	result, err := Check(context.Background(), CheckRequest{
		ToolVersion: "test",
		Network:     "testnet",
		Endpoints:   []string{"https://horizon-testnet.example"},
		Accounts:    []string{accountOne, accountTwo},
		FailOn:      model.SevHigh,
		NewSource:   stubFactory(fs),
		NewIdentity: newIdentity,
		Evaluate:    rules.Evaluate,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if calls := fs.fakeSource.called(); len(calls) != 0 {
		t.Fatalf("account reads = %v, want none on a network mismatch", calls)
	}
	if result.ExitCode != model.ExitFindings || len(result.Accounts) != 2 {
		t.Fatalf("report exit/accounts = %d/%d, want %d/2: %+v", result.ExitCode, len(result.Accounts), model.ExitFindings, result.Accounts)
	}
	for _, account := range result.Accounts {
		if account.Skipped == "" || len(account.Findings) != 1 || account.Findings[0].RuleID != "acct-network-mismatch" || account.Findings[0].Severity != model.SevHigh {
			t.Errorf("account report = %+v, want one skipped high network mismatch", account)
		}
	}
}
