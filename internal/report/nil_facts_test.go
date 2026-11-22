package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// Task 7.16: AccountReport.Facts is a pointer because a report can lack
// observations, as a hand-built or older saved report does. Such a report must not
// panic, must not invent fact lines, and must still print its heading and findings
// in both formats. TestReportWithoutAFactBlockPrintsNoFactLines covers text with no
// findings; these cases add the findings and the JSON path through prepareFacts.

func nilFactsAccount(facts *model.AccountFacts) model.Report {
	return model.Report{
		ToolVersion: "test",
		Accounts: []model.AccountReport{{
			Snapshot: model.AccountSnapshot{ID: accountOne},
			Facts:    facts,
			Findings: []model.Finding{
				model.NewFinding("sig-single-key", model.SevHigh, accountOne,
					"One key reaches the high threshold.", ""),
			},
		}},
		ExitCode: model.ExitFindings,
	}
}

func TestAccountWithoutFactsText(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Text(&buf, nilFactsAccount(nil)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"account " + accountOne,
		"findings: 1 (0 accepted)",
		"sig-single-key",
		"One key reaches the high threshold.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text report with nil facts has no %q:\n%s", want, out)
		}
	}
	// "declared: nothing" means a run read no policy file; an absent block means no
	// run assembled facts at all, so neither that line nor any other fact line may
	// appear.
	for _, unwanted := range []string{
		"declared:", "identity document:", "minimum signers", "expected_signers",
	} {
		if strings.Contains(out, unwanted) {
			t.Errorf("text report with nil facts printed %q:\n%s", unwanted, out)
		}
	}
}

func TestAccountWithoutFactsInnerPointersText(t *testing.T) {
	t.Parallel()
	// A present block with every inner pointer cleared reaches each facts writer
	// with a non-nil block, so each inner guard is exercised on its own. The run
	// that built it read no policy file, which is exactly what "declared: nothing"
	// says, so that line is correct here and only here.
	var buf bytes.Buffer
	if err := Text(&buf, nilFactsAccount(&model.AccountFacts{})); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"account " + accountOne,
		"declared: nothing",
		"findings: 1 (0 accepted)",
		"sig-single-key",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text report with empty facts has no %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"identity document:", "minimum signers", "expected_signers"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("text report with empty facts printed %q:\n%s", unwanted, out)
		}
	}
}

func TestAccountWithoutFactsJSON(t *testing.T) {
	t.Parallel()
	type decoded struct {
		Accounts []map[string]json.RawMessage `json:"accounts"`
	}
	render := func(t *testing.T, facts *model.AccountFacts) map[string]json.RawMessage {
		t.Helper()
		var buf bytes.Buffer
		if err := JSON(&buf, nilFactsAccount(facts)); err != nil {
			t.Fatal(err)
		}
		var doc decoded
		if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
			t.Fatalf("output is not JSON: %v\n%s", err, buf.String())
		}
		if len(doc.Accounts) != 1 {
			t.Fatalf("got %d accounts, want 1", len(doc.Accounts))
		}
		account := doc.Accounts[0]
		var findings []model.Finding
		if err := json.Unmarshal(account["findings"], &findings); err != nil {
			t.Fatalf("findings did not decode: %v", err)
		}
		if len(findings) != 1 || findings[0].RuleID != "sig-single-key" {
			t.Fatalf("findings = %+v, want the one sig-single-key finding", findings)
		}
		return account
	}

	t.Run("nil block", func(t *testing.T) {
		t.Parallel()
		account := render(t, nil)
		if raw, ok := account["facts"]; ok {
			t.Fatalf("JSON for nil facts carries facts = %s, want the key omitted", raw)
		}
	})

	t.Run("empty block", func(t *testing.T) {
		t.Parallel()
		// A present block with every inner pointer cleared is a run that observed
		// nothing; prepareFacts must copy it without dereferencing any of them, and
		// each stays null rather than gaining an invented value.
		account := render(t, &model.AccountFacts{})
		raw, ok := account["facts"]
		if !ok {
			t.Fatal("JSON dropped a present but empty facts block")
		}
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(raw, &inner); err != nil {
			t.Fatalf("facts is not an object: %v (%s)", err, raw)
		}
		for _, key := range []string{"reachability", "declared", "identity"} {
			if got := string(inner[key]); got != "null" {
				t.Errorf("facts.%s = %s, want null", key, got)
			}
		}
	})
}
