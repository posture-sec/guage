package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
)

const testAccount = "GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG"

const (
	testnetPassphrase = "Test SDF Network ; September 2015"
	mainnetPassphrase = "Public Global Stellar Network ; September 2015"
)

func fixtureFlags(t *testing.T, name string) model.Flags {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "horizon", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	var payload struct {
		Flags struct {
			AuthRequired    bool `json:"auth_required"`
			AuthRevocable   bool `json:"auth_revocable"`
			AuthImmutable   bool `json:"auth_immutable"`
			ClawbackEnabled bool `json:"auth_clawback_enabled"`
		} `json:"flags"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decoding fixture %s: %v", name, err)
	}
	return model.Flags{
		AuthRequired:    payload.Flags.AuthRequired,
		AuthRevocable:   payload.Flags.AuthRevocable,
		AuthImmutable:   payload.Flags.AuthImmutable,
		ClawbackEnabled: payload.Flags.ClawbackEnabled,
	}
}

func boolean(v bool) *bool { return &v }

func TestAllListsEveryRuleInOrder(t *testing.T) {
	t.Parallel()
	want := []string{
		"acct-missing",
		"acct-network-mismatch",
		"flag-drift",
		"flag-undeclared-clawback",
	}
	all := All()
	if len(all) != len(want) {
		t.Fatalf("got %d rules, want %d", len(all), len(want))
	}
	for i, rule := range all {
		if rule.ID != want[i] {
			t.Errorf("rule %d is %q, want %q", i, rule.ID, want[i])
		}
		if rule.Check == nil {
			t.Errorf("rule %q has no check", rule.ID)
		}
	}
}

func TestAllReturnsACopy(t *testing.T) {
	t.Parallel()
	first := All()
	first[0].ID = "changed"
	if All()[0].ID != "acct-missing" {
		t.Fatal("All exposed the registry slice for mutation")
	}
}

func TestAccountMissing(t *testing.T) {
	t.Parallel()
	ctx := Context{
		Account:  policy.Account{ID: testAccount},
		Snapshot: model.AccountSnapshot{ID: testAccount, Endpoint: "https://horizon-testnet.stellar.org"},
		Missing:  true,
	}
	findings := accountMissing(ctx)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	f := findings[0]
	if f.RuleID != "acct-missing" || f.Severity != model.SevHigh || f.Account != testAccount {
		t.Errorf("unexpected finding: %+v", f)
	}
	if !strings.Contains(f.Evidence, "horizon-testnet.stellar.org") || !strings.Contains(f.Evidence, "not found") {
		t.Errorf("evidence does not name the endpoint and the answer: %q", f.Evidence)
	}

	ctx.Missing = false
	if got := accountMissing(ctx); len(got) != 0 {
		t.Errorf("present account raised %d findings, want 0", len(got))
	}
}

func TestAccountNetworkMismatch(t *testing.T) {
	t.Parallel()
	ctx := Context{
		Account:  policy.Account{ID: testAccount},
		Snapshot: model.AccountSnapshot{ID: testAccount, Endpoint: "https://horizon-testnet.stellar.org"},
		Mismatch: true,
		Network:  mainnetPassphrase,
		Expected: testnetPassphrase,
	}
	findings := accountNetworkMismatch(ctx)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	f := findings[0]
	if f.RuleID != "acct-network-mismatch" || f.Severity != model.SevHigh || f.Account != testAccount {
		t.Errorf("unexpected finding: %+v", f)
	}
	if !strings.Contains(f.Evidence, testnetPassphrase) || !strings.Contains(f.Evidence, mainnetPassphrase) {
		t.Errorf("evidence does not name both networks: %q", f.Evidence)
	}

	ctx.Mismatch = false
	if got := accountNetworkMismatch(ctx); len(got) != 0 {
		t.Errorf("matching network raised %d findings, want 0", len(got))
	}
}

func TestFlagDrift(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		fixture      string
		expected     *policy.ExpectedFlags
		wantEvidence string
	}{
		{
			name:     "no expectation declared",
			fixture:  "8-clawback-undeclared.json",
			expected: nil,
		},
		{
			name:     "declaration matches observed",
			fixture:  "8-clawback-undeclared.json",
			expected: &policy.ExpectedFlags{AuthRequired: boolean(true), AuthRevocable: boolean(true)},
		},
		{
			name:         "one declared flag drifts",
			fixture:      "8-clawback-undeclared.json",
			expected:     &policy.ExpectedFlags{AuthRequired: boolean(false)},
			wantEvidence: "auth_required declared false, observed true",
		},
		{
			name:    "all four drift in fixed order",
			fixture: "8-clawback-undeclared.json",
			expected: &policy.ExpectedFlags{
				AuthRequired:    boolean(false),
				AuthRevocable:   boolean(false),
				AuthImmutable:   boolean(true),
				ClawbackEnabled: boolean(false),
			},
			wantEvidence: "auth_required declared false, observed true; " +
				"auth_revocable declared false, observed true; " +
				"auth_immutable declared true, observed false; " +
				"clawback declared false, observed true",
		},
		{
			name:     "all flags match on the default fixture",
			fixture:  "1-default.json",
			expected: &policy.ExpectedFlags{AuthRequired: boolean(false), AuthRevocable: boolean(false), AuthImmutable: boolean(false), ClawbackEnabled: boolean(false)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := Context{
				Account:  policy.Account{ID: testAccount, ExpectedFlags: tc.expected},
				Snapshot: model.AccountSnapshot{ID: testAccount, Flags: fixtureFlags(t, tc.fixture)},
			}
			findings := flagDrift(ctx)
			if tc.wantEvidence == "" {
				if len(findings) != 0 {
					t.Fatalf("got %+v, want no findings", findings)
				}
				return
			}
			if len(findings) != 1 {
				t.Fatalf("got %d findings, want 1", len(findings))
			}
			f := findings[0]
			if f.RuleID != "flag-drift" || f.Severity != model.SevWarn || f.Account != testAccount {
				t.Errorf("unexpected finding: %+v", f)
			}
			if f.Evidence != tc.wantEvidence {
				t.Errorf("evidence is %q, want %q", f.Evidence, tc.wantEvidence)
			}
		})
	}
}

func TestFlagDriftSkipsUnreadAccounts(t *testing.T) {
	t.Parallel()
	expected := &policy.ExpectedFlags{AuthRequired: boolean(true)}
	for _, tc := range []struct {
		name string
		ctx  Context
	}{
		{"missing", Context{Missing: true}},
		{"mismatch", Context{Mismatch: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.ctx.Account = policy.Account{ID: testAccount, ExpectedFlags: expected}
			tc.ctx.Snapshot = model.AccountSnapshot{ID: testAccount}
			if got := flagDrift(tc.ctx); len(got) != 0 {
				t.Errorf("got %+v, want no findings", got)
			}
		})
	}
}

func TestFlagUndeclaredClawback(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		fixture  string
		expected *policy.ExpectedFlags
	}{
		{name: "clawback on and policy silent", fixture: "8-clawback-undeclared.json"},
		{name: "clawback on and policy declares other flags", fixture: "8-clawback-undeclared.json", expected: &policy.ExpectedFlags{AuthRequired: boolean(true)}},
		{name: "clawback declared true", fixture: "8-clawback-undeclared.json", expected: &policy.ExpectedFlags{ClawbackEnabled: boolean(true)}},
		{name: "clawback declared false", fixture: "8-clawback-undeclared.json", expected: &policy.ExpectedFlags{ClawbackEnabled: boolean(false)}},
		{name: "clawback off and policy silent", fixture: "1-default.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := tc.fixture == "8-clawback-undeclared.json" &&
				(tc.expected == nil || tc.expected.ClawbackEnabled == nil)
			ctx := Context{
				Account:  policy.Account{ID: testAccount, ExpectedFlags: tc.expected},
				Snapshot: model.AccountSnapshot{ID: testAccount, Flags: fixtureFlags(t, tc.fixture)},
			}
			findings := flagUndeclaredClawback(ctx)
			if want != (len(findings) == 1) {
				t.Fatalf("got %d findings, want firing=%t", len(findings), want)
			}
			if !want {
				return
			}
			f := findings[0]
			if f.RuleID != "flag-undeclared-clawback" || f.Severity != model.SevWarn || f.Account != testAccount {
				t.Errorf("unexpected finding: %+v", f)
			}
		})
	}
}

func TestEvaluateCountsOnlyWhatFires(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		ctx     Context
		wantIDs []string
	}{
		{
			name: "clawback with no policy is one warning",
			ctx: Context{
				Account:  policy.Account{ID: testAccount},
				Snapshot: model.AccountSnapshot{ID: testAccount, Flags: fixtureFlags(t, "8-clawback-undeclared.json")},
			},
			wantIDs: []string{"flag-undeclared-clawback"},
		},
		{
			name: "fully declared matching policy is silent",
			ctx: Context{
				Account: policy.Account{ID: testAccount, ExpectedFlags: &policy.ExpectedFlags{
					AuthRequired:    boolean(false),
					AuthRevocable:   boolean(false),
					AuthImmutable:   boolean(false),
					ClawbackEnabled: boolean(false),
				}},
				Snapshot: model.AccountSnapshot{ID: testAccount, Flags: fixtureFlags(t, "1-default.json")},
			},
		},
		{
			name: "a missing account only raises acct-missing",
			ctx: Context{
				Account: policy.Account{ID: testAccount, ExpectedFlags: &policy.ExpectedFlags{
					AuthRequired:    boolean(true),
					ClawbackEnabled: boolean(true),
				}},
				Snapshot: model.AccountSnapshot{ID: testAccount, Endpoint: "https://horizon-testnet.stellar.org"},
				Missing:  true,
			},
			wantIDs: []string{"acct-missing"},
		},
		{
			name: "a mismatch only raises acct-network-mismatch",
			ctx: Context{
				Account: policy.Account{ID: testAccount, ExpectedFlags: &policy.ExpectedFlags{
					AuthRequired:    boolean(true),
					ClawbackEnabled: boolean(true),
				}},
				Snapshot: model.AccountSnapshot{ID: testAccount, Endpoint: "https://horizon-testnet.stellar.org"},
				Mismatch: true,
				Network:  mainnetPassphrase,
				Expected: testnetPassphrase,
			},
			wantIDs: []string{"acct-network-mismatch"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			findings := Evaluate(tc.ctx)
			if len(findings) != len(tc.wantIDs) {
				t.Fatalf("got %+v, want rule IDs %v", findings, tc.wantIDs)
			}
			for i, id := range tc.wantIDs {
				if findings[i].RuleID != id {
					t.Errorf("finding %d is %q, want %q", i, findings[i].RuleID, id)
				}
			}
		})
	}
}
