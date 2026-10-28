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
