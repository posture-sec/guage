package report

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

var update = flag.Bool("update", false, "rewrite the golden files")

const (
	accountOne = "GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ"
	accountTwo = "GBK4NFJ7SVMUNPBIYBC4WNMCTKI6Z66MYYWZQSSLQNOSUSTMKVUKE7M3"
)

func sampleReport() model.Report {
	accepted := model.NewFinding("flag-drift", model.SevWarn, accountOne,
		"Declared flags differ from the chain.", "auth_required declared false, observed true")
	accepted.Accepted = true
	return model.Report{
		ToolVersion: "0.0.0-test",
		Network:     "Test SDF Network ; September 2015",
		Ledger:      5466897,
		Accounts: []model.AccountReport{
			{
				Snapshot: model.AccountSnapshot{
					ID:       accountOne,
					Network:  "Test SDF Network ; September 2015",
					Endpoint: "https://horizon-testnet.stellar.org",
					Ledger:   5466881,
					Sequence: "9007199254740993",
					// The high tier is 1 because the single-key finding below says one key
					// reaches it, and the reachability facts now printed beside that finding
					// have to agree with it. A sample whose facts contradict its findings would
					// train a reviewer to read the two blocks as unrelated.
					Thresholds: model.Thresholds{Low: 1, Medium: 2, High: 1},
					Signers: []model.Signer{
						{Key: accountOne, Type: model.SignerEd25519, Weight: 1},
					},
					Flags:      model.Flags{AuthRequired: true},
					HomeDomain: "ex\u0430mple.com\x1b[31m",
					Sponsor:    "sponsor\x1b]0;title\x07",
				},
				Facts: &model.AccountFacts{
					// One participating weight of 1: the low and high tiers are reachable with
					// it, the medium tier of 2 is out of reach for the whole account.
					Reachability: &model.Reachability{Low: value(1), High: value(1)},
					Declared: &model.DeclaredFacts{
						Name: "treasury\u001b[31m", Role: "treasury",
						ExpectedSigners:     value(1),
						ExpectedThresholds:  &model.DeclaredThresholds{Low: value(2), Medium: value(2), High: value(3)},
						ExpectLocked:        value(true),
						ExpectLockedIgnored: true,
					},
					Identity: &model.IdentityFacts{
						HomeDomain: "example.com", Reachable: true, Step: "ok",
						Status: 200, Hops: 1, Bytes: 132, CORSPresent: true,
						NetworkPassphrase: value("Test SDF Network ; September 2015"),
						NetworkMatches:    value(true),
						SigningKey:        value(accountTwo),
						// A version string is remote content, so this sample carries one with a
						// bell in it and the hostile-string test looks for it in both formats.
						Version:           value("v1.2\u0007"),
						Accounts:          []string{accountOne},
						MalformedAccounts: 1,
					},
				},
				Findings: []model.Finding{
					model.NewFinding("sig-single-key-high", model.SevHigh, accountOne,
						"One signing key controls the high threshold.", "minimum signers: 1"),
					model.NewFinding("thr-drift", model.SevWarn, accountTwo,
						"Declared thresholds differ from the chain.", "low 2, medium 2, high 3"),
					accepted,
				},
			},
			{
				Snapshot: model.AccountSnapshot{
					ID:         accountTwo,
					Network:    "Test SDF Network ; September 2015",
					Endpoint:   "https://horizon-testnet.stellar.org",
					Ledger:     5466881,
					Thresholds: model.Thresholds{Low: 2, Medium: 2, High: 2},
					Signers: []model.Signer{
						{Key: accountOne, Type: model.SignerEd25519, Weight: 1},
						{Key: accountTwo, Type: model.SignerEd25519, Weight: 1},
					},
					Flags: model.Flags{},
				},
				// Two weights of 1 against tiers of 2: every tier needs both keys. The
				// declaration is the shape an audit produces for an account whose operator
				// wrote only an ID and a role, which is what the drift rules falling silent
				// is supposed to look like in a report.
				Facts: &model.AccountFacts{
					Reachability: &model.Reachability{Low: value(2), Medium: value(2), High: value(2)},
					Declared:     &model.DeclaredFacts{Role: "other"},
				},
			},
		},
		ExitCode: model.ExitFindings,
	}
}

func value[T any](v T) *T { return &v }

func TestTextGolden(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Text(&buf, sampleReport()); err != nil {
		t.Fatalf("Text: %v", err)
	}
	checkGolden(t, "sample.text.golden", buf.Bytes())
}

func TestJSONGolden(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := JSON(&buf, sampleReport()); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	checkGolden(t, "sample.json.golden", buf.Bytes())
	var document map[string]any
	if err := json.Unmarshal(buf.Bytes(), &document); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if _, ok := document["limits"]; !ok {
		t.Fatal("JSON report has no limits field")
	}
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "golden", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file (run with -update to create it): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("output differs from %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestRenderersStripHostileStrings(t *testing.T) {
	t.Parallel()
	var text, document bytes.Buffer
	if err := Text(&text, sampleReport()); err != nil {
		t.Fatal(err)
	}
	if err := JSON(&document, sampleReport()); err != nil {
		t.Fatal(err)
	}
	for name, out := range map[string]string{"text": text.String(), "json": document.String()} {
		for _, r := range []rune{0x1b, 0x07, 0x202e, 0x200b} {
			if strings.ContainsRune(out, r) {
				t.Fatalf("%s output kept U+%04X", name, r)
			}
		}
		if strings.Contains(out, "title") {
			t.Fatalf("%s output kept an OSC payload", name)
		}
	}
	if !strings.Contains(text.String(), "(IDN)") {
		t.Fatal("text output does not flag the IDN home domain")
	}
	if strings.Contains(text.String(), "secure") || strings.Contains(text.String(), "safe") {
		t.Fatal("text output uses forbidden wording")
	}
}

func TestTextIsDeterministic(t *testing.T) {
	t.Parallel()
	var first, second bytes.Buffer
	if err := Text(&first, sampleReport()); err != nil {
		t.Fatal(err)
	}
	if err := Text(&second, sampleReport()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("text output changed between identical runs")
	}
}

func TestRenderersDoNotMutate(t *testing.T) {
	t.Parallel()
	input := sampleReport()
	before, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var sink bytes.Buffer
	if err := Text(&sink, input); err != nil {
		t.Fatal(err)
	}
	if err := JSON(&sink, input); err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("a renderer mutated the report")
	}
}

func TestExit(t *testing.T) {
	t.Parallel()
	reportWith := func(findings ...model.Finding) model.Report {
		return model.Report{Accounts: []model.AccountReport{{Findings: findings}}}
	}
	accepted := model.NewFinding("flag-drift", model.SevHigh, accountOne, "m", "e")
	accepted.Accepted = true
	for _, tt := range []struct {
		name   string
		report model.Report
		failOn model.Severity
		want   model.ExitCode
	}{
		{"no findings", reportWith(), model.SevHigh, model.ExitPass},
		{"no accounts", model.Report{}, model.SevHigh, model.ExitPass},
		{"high at high", reportWith(model.NewFinding("a-b", model.SevHigh, accountOne, "m", "e")), model.SevHigh, model.ExitFindings},
		{"warn below high", reportWith(model.NewFinding("a-b", model.SevWarn, accountOne, "m", "e")), model.SevHigh, model.ExitPass},
		{"warn at warn", reportWith(model.NewFinding("a-b", model.SevWarn, accountOne, "m", "e")), model.SevWarn, model.ExitFindings},
		{"info at info", reportWith(model.NewFinding("a-b", model.SevInfo, accountOne, "m", "e")), model.SevInfo, model.ExitFindings},
		{"accepted high", reportWith(accepted), model.SevHigh, model.ExitPass},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Exit(tt.report, tt.failOn); got != tt.want {
				t.Fatalf("Exit = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMissingAndSkippedAccountsRender(t *testing.T) {
	t.Parallel()
	r := model.Report{
		ToolVersion: "development",
		Network:     "Test SDF Network ; September 2015",
		Accounts: []model.AccountReport{
			{
				Snapshot: model.AccountSnapshot{ID: accountOne, Endpoint: "https://horizon-testnet.stellar.org"},
				Missing:  true,
				Findings: []model.Finding{model.NewFinding("acct-missing", model.SevHigh, accountOne,
					"the account was not found on this endpoint", "endpoint https://horizon-testnet.stellar.org returned not found")},
			},
			{
				Snapshot: model.AccountSnapshot{ID: accountTwo, Endpoint: "https://horizon-testnet.stellar.org"},
				Skipped:  "the endpoint is on a different network\x1b[31m",
			},
		},
		ExitCode: model.ExitFindings,
	}

	var text bytes.Buffer
	if err := Text(&text, r); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"not found on this endpoint (never created, or merged)",
		"no state read: the endpoint is on a different network",
		"[high] acct-missing",
		"exit code: 1",
	} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("text output does not contain %q:\n%s", want, text.String())
		}
	}
	if strings.ContainsRune(text.String(), 0x1b) {
		t.Fatal("text output kept the escape from the skipped reason")
	}

	var document bytes.Buffer
	if err := JSON(&document, r); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Accounts []struct {
			Missing bool   `json:"missing"`
			Skipped string `json:"skipped"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(document.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, document.String())
	}
	if len(payload.Accounts) != 2 {
		t.Fatalf("got %d accounts, want 2", len(payload.Accounts))
	}
	if !payload.Accounts[0].Missing {
		t.Error("the missing account did not carry missing=true")
	}
	if payload.Accounts[1].Skipped != "the endpoint is on a different network" {
		t.Errorf("skipped reason is %q", payload.Accounts[1].Skipped)
	}
	if strings.ContainsRune(document.String(), 0x1b) {
		t.Fatal("JSON output kept the escape from the skipped reason")
	}
}

func TestEmptyReportRenders(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Text(&buf, model.Report{ToolVersion: "development"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no accounts") {
		t.Fatalf("empty report text = %q", buf.String())
	}
}

// Task 3.7: the fact block is what a zero-config run prints, and it is what keeps a
// report readable when the rules had nothing to compare against. These cases hold the
// three properties the design promises: a fact cannot fail a run, a report with no
// fact block prints no fact lines, and the two kinds of "nothing declared" say which
// one the reader is looking at.

func TestFactsCannotFailARun(t *testing.T) {
	t.Parallel()
	// Every fact in this account is one an operator would worry about: no tier the
	// weights can reach, a document naming a different network, and a lock the role
	// may not declare. There are no findings, so the run passes and the report says
	// why, which is the division the catalog keeps.
	report := model.Report{
		ToolVersion: "0.0.0-test",
		Network:     "Test SDF Network ; September 2015",
		Accounts: []model.AccountReport{{
			Snapshot: model.AccountSnapshot{
				ID: accountOne, Thresholds: model.Thresholds{Low: 10, Medium: 10, High: 10},
				Signers: []model.Signer{{Key: accountOne, Type: model.SignerEd25519, Weight: 1}},
			},
			Facts: &model.AccountFacts{
				// A tier present only when its own signers can reach it, so the three
				// values stay nil and the line says so.
				Reachability: &model.Reachability{},
				Declared: &model.DeclaredFacts{
					Role: "treasury", ExpectLocked: value(true), ExpectLockedIgnored: true,
				},
				Identity: &model.IdentityFacts{
					HomeDomain: "example.com", Reachable: true, Step: "ok", Status: 200,
					NetworkPassphrase: value("Public Global Stellar Network ; September 2015"),
					NetworkMatches:    value(false),
				},
			},
		}},
		ExitCode: model.ExitPass,
	}
	if got := Exit(report, model.SevInfo); got != model.ExitPass {
		t.Fatalf("a report of facts alone exited %d, want 0", got)
	}

	var text bytes.Buffer
	if err := Text(&text, report); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"minimum signers to authorize: low (unreachable), medium (unreachable), high (unreachable)",
		`network_passphrase: "Public Global Stellar Network ; September 2015" (not this network)`,
		`expect_locked: true, which role "treasury" is not allowed to declare`,
		"accounts: 0 listed",
		"exit code: 0",
	} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("text output has no %q:\n%s", want, text.String())
		}
	}
}

// TestReportWithoutAFactBlockPrintsNoFactLines is the other direction: an absence of
// facts is not a fact about absence. A report assembled without a block, as no run
// does, says nothing in the places a run would have answered.
func TestReportWithoutAFactBlockPrintsNoFactLines(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	report := model.Report{Accounts: []model.AccountReport{{
		Snapshot: model.AccountSnapshot{ID: accountOne, HomeDomain: "example.com"},
	}}}
	if err := Text(&buf, report); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, unwanted := range []string{
		"declared:", "identity document:", "minimum signers", "expect_locked",
	} {
		if strings.Contains(out, unwanted) {
			t.Errorf("a report with no fact block printed %q:\n%s", unwanted, out)
		}
	}
}

func TestTheTwoKindsofUndeclaredAccountSayWhich(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	report := model.Report{Accounts: []model.AccountReport{
		{Snapshot: model.AccountSnapshot{ID: accountOne},
			Facts: &model.AccountFacts{Declared: nil}},
		{Snapshot: model.AccountSnapshot{ID: accountTwo},
			Facts: &model.AccountFacts{Declared: &model.DeclaredFacts{Role: "other"}}},
		{Snapshot: model.AccountSnapshot{ID: accountOne, HomeDomain: "example.com"},
			Facts: &model.AccountFacts{}},
	}}
	if err := Text(&buf, report); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"declared: nothing; this run read no policy file",
		"declared: role other",
		"expected_signers: (not declared)",
		"identity document: never fetched, so the published home domain is unverified",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output has no %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "identity document:") != 1 {
		t.Errorf("an account publishing no domain still spoke about identity:\n%s", out)
	}
}

func TestAFailingDocumentPrintsTheStepAndNoParsedValues(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	report := model.Report{Accounts: []model.AccountReport{{
		Snapshot: model.AccountSnapshot{ID: accountOne, HomeDomain: "example.com"},
		Facts: &model.AccountFacts{Identity: &model.IdentityFacts{
			HomeDomain: "example.com", Step: "guard",
			Detail: "the host name resolves to a loopback address, which Gauge will not dial",
		}},
	}}}
	if err := Text(&buf, report); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"identity document: example.com did not serve a readable file",
		"step: guard",
		"fetch: no response received",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output has no %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"network_passphrase:", "signing_key:", "accounts:"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("a document that never parsed printed %q:\n%s", unwanted, out)
		}
	}
}
