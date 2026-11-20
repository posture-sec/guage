package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// summaryReport builds a minimal single-account report so this test pins the
// summary arithmetic without touching any golden file.
func summaryReport(findings []model.Finding) model.Report {
	return model.Report{
		ToolVersion: "0.0.0-test",
		Network:     "Test SDF Network ; September 2015",
		Ledger:      5466897,
		Accounts: []model.AccountReport{{
			Snapshot: model.AccountSnapshot{ID: accountOne, Endpoint: "https://horizon-testnet.stellar.org"},
			Findings: findings,
		}},
	}
}

func TestAcceptedFindingsCountedOnce(t *testing.T) {
	t.Parallel()

	// Case 1: 1 accepted High + 1 open Warn. Accepted is a subset of the
	// severity buckets, never a fourth category, so the total stays 2.
	acceptedHigh := model.NewFinding("flag-drift", model.SevHigh, accountOne,
		"Declared flags differ from the chain.", "auth_required declared false, observed true")
	acceptedHigh.Accepted = true
	openWarn := model.NewFinding("thr-drift", model.SevWarn, accountOne,
		"Declared thresholds differ from the chain.", "low 2, medium 2, high 3")

	var buf bytes.Buffer
	if err := Text(&buf, summaryReport([]model.Finding{acceptedHigh, openWarn})); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"result: 2 findings (1 high, 1 warn, 0 info), 1 accepted",
		"findings: 2 (1 accepted)",
		"[high] flag-drift (accepted)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("case 1 output has no %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "[warn] thr-drift (accepted)") {
		t.Error("case 1: open Warn should not carry the accepted marker")
	}

	// Case 2: 2 accepted findings of one severity. Total is 2 (not 4) and
	// the accepted count is 2; each accepted High stays in the high bucket.
	first := model.NewFinding("flag-drift", model.SevHigh, accountOne, "m", "e")
	first.Accepted = true
	second := model.NewFinding("thr-drift", model.SevHigh, accountOne, "m2", "e2")
	second.Accepted = true

	buf.Reset()
	if err := Text(&buf, summaryReport([]model.Finding{first, second})); err != nil {
		t.Fatal(err)
	}
	out = buf.String()
	for _, want := range []string{
		"result: 2 findings (2 high, 0 warn, 0 info), 2 accepted",
		"findings: 2 (2 accepted)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("case 2 output has no %q:\n%s", want, out)
		}
	}
}
