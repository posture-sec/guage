package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// Task 7.15: an empty report is a real outcome (a filtered run, a policy with no
// accounts) and a CI job parses its shape. This pins every line the text renderer
// owes it and the JSON envelope, where accounts must be a present empty array and
// never null (ARCHITECTURE.md 6.8).
func TestEmptyReportRendersBothFormats(t *testing.T) {
	t.Parallel()
	report := model.Report{ToolVersion: "test", ExitCode: model.ExitPass}

	t.Run("text", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		if err := Text(&buf, report); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		for _, want := range []string{
			"no accounts were checked",
			"result: 0 findings (0 high, 0 warn, 0 info), 0 accepted",
			"exit code: 0",
			"limits: " + Limits,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("empty text report has no %q:\n%s", want, out)
			}
		}
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		if err := JSON(&buf, report); err != nil {
			t.Fatal(err)
		}
		var top map[string]json.RawMessage
		if err := json.Unmarshal(buf.Bytes(), &top); err != nil {
			t.Fatalf("output is not a JSON object: %v\n%s", err, buf.String())
		}

		// A nil slice marshals as null, which decodes into a nil Go slice just as
		// an absent key does, so the raw token is checked rather than the decoded
		// length.
		accounts, ok := top["accounts"]
		if !ok {
			t.Fatalf("JSON has no accounts key:\n%s", buf.String())
		}
		if got := string(bytes.TrimSpace(accounts)); got != "[]" {
			t.Fatalf("accounts = %s, want a present empty array []", got)
		}

		var limits string
		if err := json.Unmarshal(top["limits"], &limits); err != nil {
			t.Fatalf("limits is missing or not a string: %v\n%s", err, buf.String())
		}
		if limits != Limits {
			t.Fatalf("limits = %q, want report.Limits", limits)
		}

		var exitCode int
		if err := json.Unmarshal(top["exit_code"], &exitCode); err != nil || exitCode != 0 {
			t.Fatalf("exit_code = %s (err %v), want 0", top["exit_code"], err)
		}
	})
}
