package report

import (
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// TestAcceptedFindingsCountedOnce pins the summary arithmetic: an accepted
// finding is a subset of its severity bucket, never a fourth category, so it
// inflates neither the total nor a bucket beyond once.
func TestAcceptedFindingsCountedOnce(t *testing.T) {
	t.Parallel()

	// Case 1: one accepted High and one open Warn in the same account.
	accepted := model.NewFinding("flag-drift", model.SevHigh, accountOne,
		"declared flags differ from the chain", "auth_required declared false, observed true")
	accepted.Accepted = true
	openWarn := model.NewFinding("thr-drift", model.SevWarn, accountOne,
		"declared thresholds differ from the chain", "low 2, medium 2, high 3")
	case1 := model.Report{
		ToolVersion: "0.0.0-test",
		Network:     "Test SDF Network ; September 2015",
		Accounts: []model.AccountReport{{
			Snapshot: model.AccountSnapshot{ID: accountOne},
			Findings: []model.Finding{accepted, openWarn},
		}},
	}
	var text1 strings.Builder
	if err := Text(&text1, case1); err != nil {
		t.Fatalf("Text: %v", err)
	}
	out1 := text1.String()
	for _, want := range []string{
		// The summary: accepted is a subset of the severities, total is 2.
		"result: 2 findings (1 high, 1 warn, 0 info), 1 accepted",
		// The per-account line: two findings, one of them accepted.
		"findings: 2 (1 accepted)",
		// Only the accepted High carries the accepted marker.
		"[high] flag-drift (accepted)",
	} {
		if !strings.Contains(out1, want) {
			t.Errorf("case 1 output has no %q:\n%s", want, out1)
		}
	}
	// The open Warn must not carry the accepted marker.
	if strings.Contains(out1, "[warn] thr-drift (accepted)") {
		t.Errorf("case 1 marked the open Warn as accepted:\n%s", out1)
	}

	// Case 2: two accepted findings of one severity must total 2, not 4, and
	// the accepted count is 2.
	two := model.NewFinding("flag-drift", model.SevWarn, accountOne,
		"declared flags differ from the chain", "auth_required declared false, observed true")
	two.Accepted = true
	three := model.NewFinding("thr-drift", model.SevWarn, accountOne,
		"declared thresholds differ from the chain", "low 2, medium 2, high 3")
	three.Accepted = true
	case2 := model.Report{
		ToolVersion: "0.0.0-test",
		Network:     "Test SDF Network ; September 2015",
		Accounts: []model.AccountReport{{
			Snapshot: model.AccountSnapshot{ID: accountOne},
			Findings: []model.Finding{two, three},
		}},
	}
	var text2 strings.Builder
	if err := Text(&text2, case2); err != nil {
		t.Fatalf("Text: %v", err)
	}
	out2 := text2.String()
	for _, want := range []string{
		"result: 2 findings (0 high, 2 warn, 0 info), 2 accepted",
		"findings: 2 (2 accepted)",
	} {
		if !strings.Contains(out2, want) {
			t.Errorf("case 2 output has no %q:\n%s", want, out2)
		}
	}
	if strings.Contains(out2, "result: 4 findings") {
		t.Errorf("case 2 counted accepted findings twice in the total:\n%s", out2)
	}
}
