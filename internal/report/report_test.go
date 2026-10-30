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
					ID:         accountOne,
					Network:    "Test SDF Network ; September 2015",
					Endpoint:   "https://horizon-testnet.stellar.org",
					Ledger:     5466881,
					Sequence:   "9007199254740993",
					Thresholds: model.Thresholds{Low: 1, Medium: 2, High: 2},
					Signers: []model.Signer{
						{Key: accountOne, Type: model.SignerEd25519, Weight: 1},
					},
					Flags:      model.Flags{AuthRequired: true},
					HomeDomain: "ex\u0430mple.com\x1b[31m",
					Sponsor:    "sponsor\x1b]0;title\x07",
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
			},
		},
		ExitCode: model.ExitFindings,
	}
}

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
