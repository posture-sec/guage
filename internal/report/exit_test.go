package report

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// TestExitBoundaryMatrix extends the exit-code table across the accepted and
// fail-on boundaries. Accepted findings are skipped whatever their severity,
// and only an unaccepted finding at or above failOn fails the run.
func TestExitBoundaryMatrix(t *testing.T) {
	t.Parallel()
	acceptedHigh := model.NewFinding("flag-drift", model.SevHigh, accountOne,
		"declared flags differ from the chain", "auth_required declared false, observed true")
	acceptedHigh.Accepted = true
	openWarn := model.NewFinding("thr-drift", model.SevWarn, accountOne,
		"declared thresholds differ from the chain", "low 2, medium 2, high 3")
	openHigh := model.NewFinding("sig-weight-drift", model.SevHigh, accountTwo,
		"declared weight differs from the chain", "declared 1, observed 2")
	info := model.NewFinding("sig-single-key-high", model.SevInfo, accountOne,
		"one signing key controls the high threshold", "minimum signers: 1")

	for _, tt := range []struct {
		name     string
		accounts [][]model.Finding // each inner slice is one account's findings
		failOn   model.Severity
		want     model.ExitCode
	}{
		{"accepted high at info", [][]model.Finding{{acceptedHigh}}, model.SevInfo, model.ExitPass},
		{"accepted high at warn", [][]model.Finding{{acceptedHigh}}, model.SevWarn, model.ExitPass},
		{"accepted high at high", [][]model.Finding{{acceptedHigh}}, model.SevHigh, model.ExitPass},
		{"open warn at warn", [][]model.Finding{{openWarn}}, model.SevWarn, model.ExitFindings},
		{"open warn at high", [][]model.Finding{{openWarn}}, model.SevHigh, model.ExitPass},
		{"info at info", [][]model.Finding{{info}}, model.SevInfo, model.ExitFindings},
		{"info at warn", [][]model.Finding{{info}}, model.SevWarn, model.ExitPass},
		{"info at high", [][]model.Finding{{info}}, model.SevHigh, model.ExitPass},
		{"accepted high plus open warn at high passes",
			[][]model.Finding{{acceptedHigh}, {openWarn}}, model.SevHigh, model.ExitPass},
		{"accepted high plus open warn at warn finds",
			[][]model.Finding{{acceptedHigh}, {openWarn}}, model.SevWarn, model.ExitFindings},
		{"two accounts, any unaccepted finding meets the threshold",
			[][]model.Finding{{acceptedHigh}, {openHigh}}, model.SevHigh, model.ExitFindings},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			accounts := make([]model.AccountReport, len(tt.accounts))
			for i, findings := range tt.accounts {
				accounts[i] = model.AccountReport{Findings: findings}
			}
			r := model.Report{Accounts: accounts}
			if got := Exit(r, tt.failOn); got != tt.want {
				t.Fatalf("Exit = %d, want %d", got, tt.want)
			}
		})
	}
}
