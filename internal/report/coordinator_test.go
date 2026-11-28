package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

func TestCoordinatorFactsFormattingAndSanitization(t *testing.T) {
	t.Parallel()

	coord := "v1.multisig.satoshipay.io"
	empty := ""
	r := model.Report{
		ToolVersion: "0.0.0-test",
		Network:     "Public Global Stellar Network ; September 2015",
		Ledger:      65282653,
		Accounts: []model.AccountReport{
			{
				Snapshot: model.AccountSnapshot{
					ID:       accountOne,
					Endpoint: "https://horizon.stellar.org",
					Ledger:   65282653,
				},
				Facts: &model.AccountFacts{
					Declared: &model.DeclaredFacts{
						Role:        model.RoleTreasury,
						Coordinator: &coord,
					},
					Coordinator: &model.CoordinatorFact{
						DataKey: "config.multisig.coordinator",
						Domain:  coord,
						Valid:   true,
					},
				},
			},
			{
				Snapshot: model.AccountSnapshot{
					ID:       accountTwo,
					Endpoint: "https://horizon.stellar.org",
					Ledger:   65282653,
				},
				Facts: &model.AccountFacts{
					Declared: &model.DeclaredFacts{
						Role:        model.RoleOther,
						Coordinator: &empty,
					},
					Coordinator: &model.CoordinatorFact{
						DataKey: "evil\x1b[31m\u202e_multisig_coordinator[md](https://bad)",
						Valid:   false,
					},
				},
			},
		},
	}

	var txt bytes.Buffer
	if err := Text(&txt, r); err != nil {
		t.Fatalf("Text: %v", err)
	}
	for _, want := range []string{
		"  coordinator: v1.multisig.satoshipay.io (data entry config.multisig.coordinator)\n",
		"    coordinator: \"v1.multisig.satoshipay.io\"\n",
		"  coordinator: (unreadable data entry evil_multisig_coordinator[md](https://bad))\n",
		"    coordinator: \"\"\n",
	} {
		if !strings.Contains(txt.String(), want) {
			t.Fatalf("text output missing %q:\n%s", want, txt.String())
		}
	}

	var mdOut bytes.Buffer
	if err := Markdown(&mdOut, r); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	if !strings.Contains(mdOut.String(), "- coordinator: v1.multisig.satoshipay.io (data entry config.multisig.coordinator)\n") {
		t.Fatalf("markdown missing valid coordinator fact:\n%s", mdOut.String())
	}
	if !strings.Contains(mdOut.String(), "- coordinator: \"v1.multisig.satoshipay.io\"\n") {
		t.Fatalf("markdown missing declared coordinator:\n%s", mdOut.String())
	}
	if strings.Contains(mdOut.String(), "[md](https://bad)") {
		t.Fatalf("markdown did not escape link in coordinator DataKey:\n%s", mdOut.String())
	}
	for _, bad := range []rune{0x1b, 0x202e} {
		if strings.ContainsRune(txt.String(), bad) || strings.ContainsRune(mdOut.String(), bad) {
			t.Fatalf("output retained hostile control U+%04X", bad)
		}
	}

	var js bytes.Buffer
	if err := JSON(&js, r); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var decoded model.Report
	if err := json.Unmarshal(js.Bytes(), &decoded); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if decoded.Accounts[0].Facts.Coordinator == nil || decoded.Accounts[0].Facts.Coordinator.Domain != coord {
		t.Fatalf("decoded valid coordinator mismatch: %+v", decoded.Accounts[0].Facts.Coordinator)
	}
	if decoded.Accounts[1].Facts.Coordinator == nil || decoded.Accounts[1].Facts.Coordinator.Valid {
		t.Fatalf("decoded invalid coordinator should keep Valid=false: %+v", decoded.Accounts[1].Facts.Coordinator)
	}
}
