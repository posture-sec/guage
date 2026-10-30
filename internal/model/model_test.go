package model_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

func TestSeverityOrdering(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name         string
		lower, upper model.Severity
	}{
		{"info before warn", model.SevInfo, model.SevWarn},
		{"warn before high", model.SevWarn, model.SevHigh},
		{"info before high", model.SevInfo, model.SevHigh},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.lower >= tt.upper {
				t.Fatalf("severity ordering: %v >= %v", tt.lower, tt.upper)
			}
		})
	}
}

func TestAccountSnapshotJSONRoundTrip(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		snapshot model.AccountSnapshot
	}{
		{"empty", model.AccountSnapshot{}},
		{"observed state", model.AccountSnapshot{
			ID: "account-reference", Network: "network-passphrase", Endpoint: "endpoint-reference",
			Ledger: 123, Sequence: "9007199254740993",
			Thresholds: model.Thresholds{Low: 3, Medium: 2, High: 1},
			Signers: []model.Signer{
				{Key: "master-reference", Type: model.SignerEd25519, Weight: 0},
				{Key: "hash-reference", Type: model.SignerHashX, Weight: 2, Sponsor: "sponsor-reference"},
				{Key: "preauth-reference", Type: model.SignerPreAuthTx, Weight: 1},
			},
			Flags:      model.Flags{AuthRequired: true, AuthRevocable: true, ClawbackEnabled: true},
			HomeDomain: "untrusted\u001b[31m.example", Sponsor: "sponsor-reference",
			Balances: []model.Balance{{AssetCode: "USD", AssetIssuer: "issuer-reference", IsAuthorized: true, ClawbackEnabled: true}},
		}},
		{"explicit empty collections", model.AccountSnapshot{Signers: []model.Signer{}, Balances: []model.Balance{}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := json.Marshal(tt.snapshot)
			if err != nil {
				t.Fatal(err)
			}
			var got model.AccountSnapshot
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.snapshot) {
				t.Fatal("snapshot round trip changed observed state")
			}
		})
	}
}

func TestReportJSONShape(t *testing.T) {
	t.Parallel()
	report := model.Report{
		ToolVersion: "development", Network: "network-passphrase", Ledger: 123,
		Accounts: []model.AccountReport{{
			Snapshot: model.AccountSnapshot{ID: "account-reference"},
			Findings: []model.Finding{{
				RuleID: "sig-single-key-high", Severity: model.SevHigh, Account: "account-reference",
				Message: "One signing key controls the high threshold.", Evidence: "minimum signers: 1",
				Fingerprint: "draft-fingerprint", Accepted: false,
			}},
		}},
		ExitCode: model.ExitFindings,
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tool_version", "network", "ledger", "accounts", "exit_code"} {
		if _, ok := document[key]; !ok {
			t.Errorf("missing report field %q", key)
		}
	}
	var got model.Report
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, report) {
		t.Fatal("report round trip changed facts or findings")
	}
}

func TestExitCodeValues(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		code model.ExitCode
		want int
	}{
		{"pass", model.ExitPass, 0},
		{"findings", model.ExitFindings, 1},
		{"usage", model.ExitUsage, 2},
		{"source or internal error", model.ExitSource, 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if int(tt.code) != tt.want {
				t.Fatalf("exit code = %d, want %d", tt.code, tt.want)
			}
		})
	}
}

func TestSeverityString(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		severity model.Severity
		want     string
	}{
		{model.SevInfo, "info"},
		{model.SevWarn, "warn"},
		{model.SevHigh, "high"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := tt.severity.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewFindingFingerprint(t *testing.T) {
	t.Parallel()
	f := model.NewFinding("sig-single-key-high", model.SevHigh, "account-reference", "message", "evidence")
	if len(f.Fingerprint) != 64 {
		t.Fatalf("fingerprint length = %d, want 64 hex characters", len(f.Fingerprint))
	}
	if f.Accepted {
		t.Fatal("a new finding is marked accepted")
	}
	if f.RuleID != "sig-single-key-high" || f.Severity != model.SevHigh || f.Account != "account-reference" {
		t.Fatal("constructor dropped a field")
	}

	same := model.NewFinding("sig-single-key-high", model.SevHigh, "account-reference", "message", "  evidence\t")
	if same.Fingerprint != f.Fingerprint {
		t.Fatal("whitespace around evidence changed the fingerprint")
	}
	inner := model.NewFinding("sig-single-key-high", model.SevHigh, "account-reference", "message", "evi  dence")
	if inner.Fingerprint == f.Fingerprint {
		t.Fatal("inner whitespace did not change the fingerprint")
	}

	for _, other := range []model.Finding{
		model.NewFinding("sig-other", model.SevHigh, "account-reference", "message", "evidence"),
		model.NewFinding("sig-single-key-high", model.SevHigh, "other-account", "message", "evidence"),
		model.NewFinding("sig-single-key-high", model.SevHigh, "account-reference", "message", "other evidence"),
	} {
		if other.Fingerprint == f.Fingerprint {
			t.Fatal("a different finding shares a fingerprint")
		}
	}
}

func TestSortFindings(t *testing.T) {
	t.Parallel()
	findings := []model.Finding{
		model.NewFinding("thr-b", model.SevWarn, "acct-b", "m", "e"),
		model.NewFinding("sig-z", model.SevHigh, "acct-b", "m", "e"),
		model.NewFinding("sig-a", model.SevHigh, "acct-b", "m", "e"),
		model.NewFinding("sig-a", model.SevHigh, "acct-a", "m", "e"),
		model.NewFinding("flag-a", model.SevInfo, "acct-a", "m", "e"),
		model.NewFinding("sig-c", model.SevWarn, "acct-a", "m", "e"),
	}
	model.SortFindings(findings)
	want := []string{
		"acct-a/sig-a/high",
		"acct-b/sig-a/high",
		"acct-b/sig-z/high",
		"acct-a/sig-c/warn",
		"acct-b/thr-b/warn",
		"acct-a/flag-a/info",
	}
	for i, f := range findings {
		got := f.Account + "/" + f.RuleID + "/" + f.Severity.String()
		if got != want[i] {
			t.Fatalf("position %d = %s, want %s", i, got, want[i])
		}
	}
}

func TestSortFindingsIsStable(t *testing.T) {
	t.Parallel()
	first := model.NewFinding("sig-a", model.SevHigh, "acct-a", "first", "evidence one")
	second := model.NewFinding("sig-a", model.SevHigh, "acct-a", "second", "evidence two")
	findings := []model.Finding{first, second}
	model.SortFindings(findings)
	if findings[0].Message != "first" || findings[1].Message != "second" {
		t.Fatal("finding order among equal keys changed")
	}
}
