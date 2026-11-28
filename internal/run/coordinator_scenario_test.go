package run

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/report"
	"github.com/posture-sec/guage/internal/rules"
	"github.com/posture-sec/guage/internal/source"
)

const coordinatorCaptureID = "GAUA7XL5K54CC2DDGP77FJ2YBHRJLT36CPZDXWPM6MP7MANOGG77PNJU"

func coordinatorScenarios() []scenario {
	satoshi := "v1.multisig.satoshipay.io"
	return []scenario{
		{
			// Mainnet capture from posture-sec/stellar-account-fixtures
			// (coordinator-data-entry) audited with no policy: config.multisig.coordinator
			// decodes to v1.multisig.satoshipay.io and raises WARN once.
			name:    "17-coordinator-undeclared",
			network: testnet, failOn: model.SevWarn,
			accounts: []scenarioAccount{{
				file:    "17-coordinator-entry.json",
				id:      coordinatorCaptureID,
				account: policy.Account{ID: coordinatorCaptureID, Role: model.RoleOther},
			}},
			want: []string{"id-multisig-coordinator-unexpected:warn sig-single-key-high:info"},
		},
		{
			// Same account shape with a disagreeing base64 coordinator entry against a
			// treasury declaration expecting v1.multisig.satoshipay.io. Note that the
			// treasury role raises sig-single-key-high to HIGH while
			// id-multisig-coordinator-unexpected stays WARN per D-17.
			name:     "18-coordinator-mismatch",
			declared: true,
			network:  testnet, failOn: model.SevWarn,
			accounts: []scenarioAccount{{
				file: "18-coordinator-mismatch.json",
				id:   coordinatorCaptureID,
				account: policy.Account{
					ID:          coordinatorCaptureID,
					Role:        model.RoleTreasury,
					Coordinator: &satoshi,
				},
			}},
			want: []string{"sig-single-key-high:high id-multisig-coordinator-unexpected:warn"},
		},
	}
}

func TestCoordinatorScenarios(t *testing.T) {
	for _, sc := range coordinatorScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			runCoordinatorScenario(t, sc)
		})
	}
}

func runCoordinatorScenario(t *testing.T, sc scenario) {
	t.Helper()
	fs := &fakeSource{
		info:       model.NetworkInfo{Passphrase: testnet, Ledger: testnetLedger},
		snapshots:  map[string]model.AccountSnapshot{},
		accountErr: map[string]error{},
	}
	accounts := make([]policy.Account, 0, len(sc.accounts))
	for _, entry := range sc.accounts {
		accounts = append(accounts, entry.account)
		info, snapshot, err := decodeFixture(t, entry.file, entry.id, entry.status)
		if err != nil {
			t.Fatalf("fixture %s: %v", entry.file, err)
		}
		if info.Ledger != testnetLedger || info.Passphrase != testnet {
			t.Fatalf("root fixture read as %q at %d", info.Passphrase, info.Ledger)
		}
		fs.snapshots[entry.id] = snapshot
		for _, bal := range snapshot.Balances {
			if bal.AssetIssuer != "" {
				fs.accountErr[bal.AssetIssuer] = source.ErrNotFound
			}
		}
	}

	job := Job{
		ToolVersion: "test",
		Network:     sc.network,
		Endpoints:   []string{canonicalTestnet},
		Accounts:    accounts,
		FailOn:      sc.failOn,
		HasPolicy:   sc.declared,
		NewSource:   stubFactory(fs),
		Evaluate:    rules.Evaluate,
	}

	result, err := Run(context.Background(), job)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var got []string
	for _, account := range result.Accounts {
		rendered := make([]string, 0, len(account.Findings))
		for _, f := range account.Findings {
			rendered = append(rendered, fmt.Sprintf("%s:%s", f.RuleID, f.Severity))
		}
		got = append(got, strings.Join(rendered, " "))
	}
	if len(got) != len(sc.want) {
		t.Fatalf("accounts = %d, want %d", len(got), len(sc.want))
	}
	for i, want := range sc.want {
		if got[i] != want {
			t.Errorf("account %d findings = %q, want %q", i, got[i], want)
		}
	}

	var text, document, page bytes.Buffer
	if err := report.Text(&text, result); err != nil {
		t.Fatalf("report.Text: %v", err)
	}
	if err := report.JSON(&document, result); err != nil {
		t.Fatalf("report.JSON: %v", err)
	}
	if err := report.Markdown(&page, result); err != nil {
		t.Fatalf("report.Markdown: %v", err)
	}
	checkGolden(t, sc.name+".text.golden", text.String())
	checkGolden(t, sc.name+".json.golden", document.String())
	checkGolden(t, sc.name+".markdown.golden", page.String())
}

func TestCoordinatorMatchedAndEmptyPolicy(t *testing.T) {
	t.Parallel()
	_, snapshot, err := decodeFixture(t, "17-coordinator-entry.json", coordinatorCaptureID, http.StatusOK)
	if err != nil {
		t.Fatalf("decodeFixture: %v", err)
	}
	fs := &fakeSource{
		info:       model.NetworkInfo{Passphrase: testnet, Ledger: testnetLedger},
		snapshots:  map[string]model.AccountSnapshot{coordinatorCaptureID: snapshot},
		accountErr: map[string]error{},
	}
	for _, bal := range snapshot.Balances {
		if bal.AssetIssuer != "" {
			fs.accountErr[bal.AssetIssuer] = source.ErrNotFound
		}
	}

	for _, tc := range []struct {
		name string
		decl string
	}{
		{name: "matching coordinator exits clean", decl: "v1.multisig.satoshipay.io"},
		{name: "empty coordinator opts out cleanly", decl: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decl := tc.decl
			job := Job{
				ToolVersion: "test",
				Network:     testnet,
				Endpoints:   []string{canonicalTestnet},
				Accounts: []policy.Account{{
					ID:          coordinatorCaptureID,
					Role:        model.RoleOther,
					Coordinator: &decl,
				}},
				FailOn:    model.SevWarn,
				HasPolicy: true,
				NewSource: stubFactory(fs),
				Evaluate:  rules.Evaluate,
			}
			result, err := Run(context.Background(), job)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if result.ExitCode != model.ExitPass {
				t.Fatalf("exit=%d findings=%+v, want exit 0", result.ExitCode, result.Accounts[0].Findings)
			}
			for _, f := range result.Accounts[0].Findings {
				if f.RuleID == "id-multisig-coordinator-unexpected" {
					t.Fatalf("unexpected coordinator finding %+v", f)
				}
			}
			facts := result.Accounts[0].Facts
			if facts == nil || facts.Coordinator == nil || !facts.Coordinator.Valid || facts.Coordinator.Domain != "v1.multisig.satoshipay.io" {
				t.Fatalf("Coordinator fact missing or wrong: %+v", facts)
			}
			if facts.Declared == nil || facts.Declared.Coordinator == nil || *facts.Declared.Coordinator != tc.decl {
				t.Fatalf("Declared.Coordinator = %v, want %q", facts.Declared, tc.decl)
			}
		})
	}
}
