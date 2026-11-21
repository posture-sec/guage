package rules

import (
	"runtime/debug"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// TestZeroContext runs every registered check against the three contexts a
// fail-closed run can build without reading an account: the zero value, an
// endpoint that reported not found, and an endpoint on the wrong network. The
// guards are per rule and were only exercised one rule at a time, so a missing
// guard in any one of the sixteen would panic inside Evaluate before a report
// existed. A recovered panic fails the named case with its stack rather than
// aborting the whole binary on the first one.
func TestZeroContext(t *testing.T) {
	t.Parallel()

	const (
		accountMissingID         = "acct-missing"
		accountNetworkMismatchID = "acct-network-mismatch"
	)
	scenarios := []struct {
		name        string
		ctx         Context
		accountRule string
		evidence    string
	}{
		{name: "zero value"},
		{
			name:        "missing only",
			ctx:         Context{Missing: true},
			accountRule: accountMissingID,
			evidence:    "not found",
		},
		{
			name:        "mismatch only",
			ctx:         Context{Mismatch: true},
			accountRule: accountNetworkMismatchID,
			evidence:    "endpoint reports",
		},
	}
	for _, scenario := range scenarios {
		for _, rule := range All() {
			t.Run(scenario.name+"/"+rule.ID, func(t *testing.T) {
				t.Parallel()
				findings, panicValue, stack := checkRule(rule, scenario.ctx)
				if panicValue != nil {
					t.Fatalf("rule %s panicked with %v\n%s", rule.ID, panicValue, stack)
				}
				for _, finding := range findings {
					switch finding.RuleID {
					case accountMissingID, accountNetworkMismatchID:
						if finding.RuleID != scenario.accountRule {
							t.Errorf("rule %s returned %q when Missing=%t and Mismatch=%t: %+v",
								rule.ID, finding.RuleID, scenario.ctx.Missing, scenario.ctx.Mismatch, finding)
							continue
						}
						if !strings.Contains(finding.Evidence, scenario.evidence) {
							t.Errorf("%s evidence %q does not contain %q", finding.RuleID, finding.Evidence, scenario.evidence)
						}
					default:
						if scenario.ctx.Missing || scenario.ctx.Mismatch {
							t.Errorf("rule %s returned %q for an account that was not read: %+v",
								rule.ID, finding.RuleID, finding)
						}
					}
				}
			})
		}
	}
}

// checkRule isolates a panic so the test can name the rule that caused it. The
// recover lives here, in the test, because production has to fail closed rather
// than hide the same panic.
func checkRule(rule Rule, c Context) (findings []model.Finding, panicValue any, stack []byte) {
	defer func() {
		if recovered := recover(); recovered != nil {
			panicValue = recovered
			stack = debug.Stack()
		}
	}()
	return rule.Check(c), nil, nil
}
