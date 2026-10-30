package report

import "github.com/posture-sec/guage/internal/model"

// Exit maps findings to an exit code. Accepted findings do not count, and a
// finding below failOn does not either. Source and usage errors are decided
// before a report exists and are not represented here.
func Exit(r model.Report, failOn model.Severity) model.ExitCode {
	for _, ar := range r.Accounts {
		for _, f := range ar.Findings {
			if !f.Accepted && f.Severity >= failOn {
				return model.ExitFindings
			}
		}
	}
	return model.ExitPass
}
