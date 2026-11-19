package report

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

const detAccount = "GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ"

// determinismReport builds one account with four findings:
//   - index 0 and 1: equal sort keys (severity High, account detAccount, rule
//     ID flag-drift) but different messages, so a stable sort must keep their
//     discovery order through a shuffle.
//   - index 2 and 3: same severity Warn and account, differing rule IDs
//     (sig-weight-drift before thr-drift), so rule ID is the final tiebreaker.
func determinismReport() model.Report {
	equalOne := model.NewFinding("flag-drift", model.SevHigh, detAccount,
		"auth required flag differs", "declared false, observed true")
	equalTwo := model.NewFinding("flag-drift", model.SevHigh, detAccount,
		"auth revocable flag differs", "declared false, observed true")
	byRule := model.NewFinding("sig-weight-drift", model.SevWarn, detAccount,
		"declared weight differs from the chain", "declared 1, observed 2")
	byRuleTwo := model.NewFinding("thr-drift", model.SevWarn, detAccount,
		"declared thresholds differ from the chain", "low 2, medium 2, high 3")
	return model.Report{
		ToolVersion: "0.0.0-test",
		Network:     "Test SDF Network ; September 2015",
		Ledger:      5466897,
		Accounts: []model.AccountReport{{
			Snapshot: model.AccountSnapshot{
				ID:         detAccount,
				Network:    "Test SDF Network ; September 2015",
				Endpoint:   "https://horizon-testnet.stellar.org",
				Ledger:     5466881,
				Thresholds: model.Thresholds{Low: 1, Medium: 2, High: 1},
				Signers: []model.Signer{
					{Key: detAccount, Type: model.SignerEd25519, Weight: 1},
				},
				Flags: model.Flags{AuthRequired: true},
			},
			Findings: []model.Finding{equalOne, equalTwo, byRule, byRuleTwo},
		}},
		ExitCode: model.ExitFindings,
	}
}

func renderTextT(t *testing.T, r model.Report) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := Text(&buf, r); err != nil {
		t.Fatalf("Text: %v", err)
	}
	return buf.Bytes()
}

func renderJSONT(t *testing.T, r model.Report) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := JSON(&buf, r); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	return buf.Bytes()
}

func copyReport(r model.Report) model.Report {
	out := r
	out.Accounts = make([]model.AccountReport, len(r.Accounts))
	for i, ar := range r.Accounts {
		dup := ar
		dup.Findings = make([]model.Finding, len(ar.Findings))
		copy(dup.Findings, ar.Findings)
		out.Accounts[i] = dup
	}
	return out
}

// TestRenderingIsByteIdentical proves the renderers and the sort decide the
// bytes, not discovery order or map iteration. Every run must equal the first
// byte for byte, and a fixed-seed shuffle must render identically to the
// ordered input.
func TestRenderingIsByteIdentical(t *testing.T) {
	t.Parallel()
	ordered := determinismReport()

	var textFirst, jsonFirst []byte
	for i := 0; i < 20; i++ {
		text := renderTextT(t, ordered)
		json := renderJSONT(t, ordered)
		if i == 0 {
			textFirst = text
			jsonFirst = json
			continue
		}
		if !bytes.Equal(text, textFirst) {
			t.Fatalf("text run %d differs from the first run", i+1)
		}
		if !bytes.Equal(json, jsonFirst) {
			t.Fatalf("json run %d differs from the first run", i+1)
		}
	}

	// Shuffle a copy with a fixed local seed (not the global rand source) and
	// assert it renders to the same bytes as the ordered input.
	rng := rand.New(rand.NewSource(1))
	shuffled := copyReport(ordered)
	for _, ar := range shuffled.Accounts {
		rng.Shuffle(len(ar.Findings), func(i, j int) {
			ar.Findings[i], ar.Findings[j] = ar.Findings[j], ar.Findings[i]
		})
	}
	if got := renderTextT(t, shuffled); !bytes.Equal(got, textFirst) {
		t.Fatalf("shuffled text output differs from ordered output\n--- ordered ---\n%s--- shuffled ---\n%s", textFirst, got)
	}
	if got := renderJSONT(t, shuffled); !bytes.Equal(got, jsonFirst) {
		t.Fatalf("shuffled JSON output differs from ordered output\n--- ordered ---\n%s--- shuffled ---\n%s", jsonFirst, got)
	}
}
