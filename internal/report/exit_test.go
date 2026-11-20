package report

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

func TestExitBoundaryMatrix(t *testing.T) {
	t.Parallel()
	acceptedHigh := accepted("flag-drift", model.SevHigh)
	openWarn := open("thr-drift", model.SevWarn)
	infoF := open("sig-hashx", model.SevInfo)
	openHigh := open("sig-single-key-high", model.SevHigh)

	// A two-account report: account one carries an open High, account two only
	// an accepted High. Any unaccepted finding meeting the threshold fails the
	// run regardless of which account it sits on.
	twoAccounts := model.Report{
		Accounts: []model.AccountReport{
			{Findings: []model.Finding{openHigh}},
			{Findings: []model.Finding{acceptedHigh}},
		},
	}

	for _, tt := range []struct {
		name   string
		report model.Report
		failOn model.Severity
		want   model.ExitCode
	}{
		{"accepted high at info", one(acceptedHigh), model.SevInfo, model.ExitPass},
		{"accepted high at warn", one(acceptedHigh), model.SevWarn, model.ExitPass},
		{"accepted high at high", one(acceptedHigh), model.SevHigh, model.ExitPass},
		{"open warn at warn", one(openWarn), model.SevWarn, model.ExitFindings},
		{"open warn at high", one(openWarn), model.SevHigh, model.ExitPass},
		{"info at info", one(infoF), model.SevInfo, model.ExitFindings},
		{"info at warn", one(infoF), model.SevWarn, model.ExitPass},
		{"info at high", one(infoF), model.SevHigh, model.ExitPass},
		{"accepted high plus open warn at high", reportWith(acceptedHigh, openWarn), model.SevHigh, model.ExitPass},
		{"accepted high plus open warn at warn", reportWith(acceptedHigh, openWarn), model.SevWarn, model.ExitFindings},
		{"two accounts, any unaccepted meets threshold", twoAccounts, model.SevHigh, model.ExitFindings},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Exit(tt.report, tt.failOn); got != tt.want {
				t.Fatalf("Exit = %d, want %d", got, tt.want)
			}
		})
	}
}

func one(f model.Finding) model.Report {
	return model.Report{Accounts: []model.AccountReport{{Findings: []model.Finding{f}}}}
}

func reportWith(findings ...model.Finding) model.Report {
	return model.Report{Accounts: []model.AccountReport{{Findings: findings}}}
}

func accepted(ruleID string, severity model.Severity) model.Finding {
	f := model.NewFinding(ruleID, severity, accountOne, "m", "e")
	f.Accepted = true
	return f
}

func open(ruleID string, severity model.Severity) model.Finding {
	return model.NewFinding(ruleID, severity, accountOne, "m", "e")
}
