package run

import (
	"context"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/rules"
)

func TestOnePanicOneFinding(t *testing.T) {
	t.Parallel()
	ids := []string{accountOne, accountTwo, accountThree}
	fs := &fakeSource{
		info: model.NetworkInfo{Passphrase: testnet, Ledger: 1},
		snapshots: map[string]model.AccountSnapshot{
			accountOne:   snapshot(accountOne, 1),
			accountTwo:   snapshot(accountTwo, 2),
			accountThree: snapshot(accountThree, 3),
		},
	}
	accounts := make([]policy.Account, len(ids))
	for i, id := range ids {
		accounts[i] = policy.Account{ID: id, Role: model.RoleOther}
	}
	realFinding := func(c rules.Context) []model.Finding {
		return []model.Finding{model.NewFinding(
			"sig-unexpected-signer",
			model.SevWarn,
			c.Snapshot.ID,
			"a real finding the run must keep",
			"evidence for the real finding",
		)}
	}
	job := baseJob(fs)
	job.Accounts = accounts
	job.FailOn = model.SevWarn
	job.Evaluate = func(c rules.Context) []model.Finding {
		switch c.Snapshot.ID {
		case accountOne, accountThree:
			panic("a rule broke")
		case accountTwo:
			return realFinding(c)
		}
		return nil
	}

	result, err := Run(context.Background(), job)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != model.ExitSource {
		t.Fatalf("exit = %d, want %d", result.ExitCode, model.ExitSource)
	}
	if len(result.Accounts) != len(ids) {
		t.Fatalf("accounts = %d, want %d", len(result.Accounts), len(ids))
	}
	for _, i := range []int{0, 2} {
		findings := result.Accounts[i].Findings
		if len(findings) != 1 {
			t.Fatalf("account %d findings = %+v, want exactly one", i, findings)
		}
		if findings[0].RuleID != "src-rule-panic" || findings[0].Severity != model.SevHigh {
			t.Fatalf("account %d findings = %+v, want one HIGH src-rule-panic", i, findings)
		}
	}
	kept := result.Accounts[1].Findings
	if len(kept) != 1 {
		t.Fatalf("account two findings = %+v, want exactly its real finding", kept)
	}
	if kept[0].RuleID != "sig-unexpected-signer" || kept[0].Severity != model.SevWarn {
		t.Fatalf("account two findings = %+v, want its real finding unchanged", kept)
	}
	total := 0
	for _, account := range result.Accounts {
		total += len(account.Findings)
	}
	if total != 3 {
		t.Fatalf("total findings = %d, want 3", total)
	}

	job.Evaluate = func(c rules.Context) []model.Finding {
		if c.Snapshot.ID == accountTwo {
			return realFinding(c)
		}
		return nil
	}
	quiet, err := Run(context.Background(), job)
	if err != nil {
		t.Fatalf("Run without panic: %v", err)
	}
	if quiet.ExitCode != model.ExitFindings {
		t.Fatalf("exit without panic = %d, want %d", quiet.ExitCode, model.ExitFindings)
	}
}
