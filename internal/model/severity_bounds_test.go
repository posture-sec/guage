package model_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

func TestSeverityBounds(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		for _, tt := range []struct {
			severity model.Severity
			want     bool
		}{
			{model.SevInfo, true},
			{model.SevWarn, true},
			{model.SevHigh, true},
			{model.Severity(-1), false},
			{model.Severity(3), false},
			{model.Severity(9), false},
		} {
			if got := tt.severity.Valid(); got != tt.want {
				t.Errorf("Severity(%d).Valid() = %v, want %v", int(tt.severity), got, tt.want)
			}
		}
	})

	for _, invalid := range []model.Severity{-1, 3} {
		severity := invalid
		t.Run("marshal rejects "+strconv.Itoa(int(severity)), func(t *testing.T) {
			t.Parallel()
			data, err := json.Marshal(severity)
			if err == nil {
				t.Fatalf("Severity(%d) marshalled as %s", int(severity), data)
			}
			if !strings.Contains(err.Error(), strconv.Itoa(int(severity))) {
				t.Fatalf("marshal error %q does not name %d", err, int(severity))
			}
			if strings.Contains(err.Error(), "unknown") {
				t.Fatalf("marshal error %q used the document name", err)
			}
			if len(data) != 0 {
				t.Fatalf("marshal returned document %s with an error", data)
			}
		})
	}

	t.Run("finding marshals only a valid severity", func(t *testing.T) {
		t.Parallel()
		if _, err := json.Marshal(model.Finding{RuleID: "test", Severity: model.Severity(3)}); err == nil {
			t.Fatal("a finding with Severity(3) marshalled")
		}
		if _, err := json.Marshal(model.Finding{RuleID: "test", Severity: model.SevHigh}); err != nil {
			t.Fatalf("a finding with a valid severity failed: %v", err)
		}
	})

	t.Run("invalid sorts below every valid severity", func(t *testing.T) {
		t.Parallel()
		findings := []model.Finding{
			model.NewFinding("above-high", model.Severity(3), "acct", "m", "e"),
			model.NewFinding("below-info", model.Severity(-1), "acct", "m", "e"),
			model.NewFinding("info", model.SevInfo, "acct", "m", "e"),
			model.NewFinding("warn", model.SevWarn, "acct", "m", "e"),
			model.NewFinding("high", model.SevHigh, "acct", "m", "e"),
		}
		model.SortFindings(findings)
		want := []string{"high", "warn", "info", "above-high", "below-info"}
		for i, ruleID := range want {
			if findings[i].RuleID != ruleID {
				t.Fatalf("position %d = %s, want %s", i, findings[i].RuleID, ruleID)
			}
		}
	})
}
