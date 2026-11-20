package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// Task 7.14: the limits footer is the last thing a reader of the text report sees
// and a top-level key in the JSON one. The golden tests cannot hold that, because a
// renderer that drops or moves the footer gets its golden regenerated in the same
// change and the diff passes review as a diff. These assertions read the constant
// directly and depend on no golden file.

func TestLimitsFooterIsTheLastTextLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		report model.Report
	}{
		{"no accounts", model.Report{ToolVersion: "test", ExitCode: model.ExitPass}},
		{"one account", model.Report{
			ToolVersion: "test",
			Accounts: []model.AccountReport{{
				Snapshot: model.AccountSnapshot{ID: accountOne},
				Findings: []model.Finding{
					model.NewFinding("sig-single-key", model.SevHigh, accountOne, "One key reaches high.", ""),
				},
			}},
			ExitCode: model.ExitFindings,
		}},
	}
	want := "limits: " + Limits
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := Text(&buf, tc.report); err != nil {
				t.Fatal(err)
			}
			out := buf.String()
			lines := nonEmptyLines(out)
			if len(lines) == 0 {
				t.Fatal("text report is empty")
			}
			if last := lines[len(lines)-1]; last != want {
				t.Fatalf("last non-empty line is %q, want the limits footer:\n%s", last, out)
			}
			if n := strings.Count(out, "limits: "); n != 1 {
				t.Fatalf("limits footer appears %d times, want 1:\n%s", n, out)
			}
			exitAt := strings.Index(out, "exit code:")
			limitsAt := strings.Index(out, want)
			if exitAt < 0 {
				t.Fatalf("text report has no exit code line:\n%s", out)
			}
			if limitsAt < exitAt {
				t.Fatalf("limits footer at byte %d comes before the exit code line at byte %d:\n%s", limitsAt, exitAt, out)
			}
		})
	}
}

func TestLimitsFooterIsAJSONField(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	report := model.Report{
		ToolVersion: "test",
		Accounts:    []model.AccountReport{{Snapshot: model.AccountSnapshot{ID: accountOne}}},
		ExitCode:    model.ExitPass,
	}
	if err := JSON(&buf, report); err != nil {
		t.Fatal(err)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &top); err != nil {
		t.Fatalf("output is not a JSON object: %v\n%s", err, buf.String())
	}
	raw, ok := top["limits"]
	if !ok {
		t.Fatalf("JSON has no top-level limits key:\n%s", buf.String())
	}
	var limits string
	if err := json.Unmarshal(raw, &limits); err != nil {
		t.Fatalf("limits is not a string: %v", err)
	}
	if limits != Limits {
		t.Fatalf("limits = %q, want report.Limits", limits)
	}
	// A duplicated or nested copy would decode fine at the top level, so count the
	// key in the raw document too: exactly one, and it is the one checked above.
	if n := strings.Count(buf.String(), `"limits":`); n != 1 {
		t.Fatalf("JSON carries %d limits keys, want 1:\n%s", n, buf.String())
	}
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}
