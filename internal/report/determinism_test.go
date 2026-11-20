package report

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// determinismReport builds a report with enough findings to make shuffle
// stability meaningful. Two findings share severity and account but differ by
// rule ID (flag-drift and thr-drift are both [warn] on accountOne). Two keys
// are fully equal: the two flag-drift findings below carry the same severity,
// account and rule ID; only their evidence differs, so a stable sort keeps
// their input order.
func determinismReport() model.Report {
	// Two findings share severity and account but differ by rule ID:
	// flag-drift and thr-drift are both [warn] on accountOne.
	// Two keys are fully equal: the two flag-drift findings below carry the
	// same severity, account and rule ID (an equal sort key). They render
	// identically, so they are interchangeable under any sort, which is what
	// lets the shuffle assertion hold byte-for-byte.
	drift := model.NewFinding("flag-drift", model.SevWarn, accountOne,
		"Declared flags differ from the chain.", "auth_required declared false, observed true")
	return model.Report{
		ToolVersion: "0.0.0-test",
		Network:     "Test SDF Network ; September 2015",
		Ledger:      5466897,
		Accounts: []model.AccountReport{
			{
				Snapshot: model.AccountSnapshot{
					ID:       accountOne,
					Endpoint: "https://horizon-testnet.stellar.org",
				},
				Findings: []model.Finding{
					model.NewFinding("sig-single-key-high", model.SevHigh, accountOne,
						"One signing key controls the high threshold.", "minimum signers: 1"),
					drift,
					drift,
					model.NewFinding("thr-drift", model.SevWarn, accountOne,
						"Declared thresholds differ from the chain.", "low 2, medium 2, high 3"),
					model.NewFinding("sig-hashx", model.SevInfo, accountOne,
						"A hash(x) signer is present.", "weight 1"),
				},
			},
			{
				Snapshot: model.AccountSnapshot{
					ID:       accountTwo,
					Endpoint: "https://horizon-testnet.stellar.org",
				},
				Findings: []model.Finding{
					model.NewFinding("flag-drift", model.SevWarn, accountTwo,
						"Declared flags differ from the chain.", "clawback declared false, observed true"),
					model.NewFinding("sig-unexpected-signer", model.SevHigh, accountTwo,
						"A signer the policy does not declare.", accountTwo),
				},
			},
		},
	}
}

func copyReport(r model.Report) model.Report {
	out := model.Report{
		ToolVersion: r.ToolVersion,
		Network:     r.Network,
		Ledger:      r.Ledger,
		ExitCode:    r.ExitCode,
		Accounts:    make([]model.AccountReport, len(r.Accounts)),
	}
	for i, ar := range r.Accounts {
		dup := ar
		dup.Findings = make([]model.Finding, len(ar.Findings))
		copy(dup.Findings, ar.Findings)
		out.Accounts[i] = dup
	}
	return out
}

func TestRenderingIsByteIdentical(t *testing.T) {
	t.Parallel()
	ordered := determinismReport()

	var textFirst, jsonFirst bytes.Buffer
	if err := Text(&textFirst, ordered); err != nil {
		t.Fatalf("text: %v", err)
	}
	if err := JSON(&jsonFirst, ordered); err != nil {
		t.Fatalf("json: %v", err)
	}
	textWant := textFirst.Bytes()
	jsonWant := jsonFirst.Bytes()

	for i := 0; i < 20; i++ {
		var tb, jb bytes.Buffer
		if err := Text(&tb, ordered); err != nil {
			t.Fatalf("text run %d: %v", i, err)
		}
		if err := JSON(&jb, ordered); err != nil {
			t.Fatalf("json run %d: %v", i, err)
		}
		if !bytes.Equal(tb.Bytes(), textWant) {
			t.Fatalf("text run %d differs from the first", i)
		}
		if !bytes.Equal(jb.Bytes(), jsonWant) {
			t.Fatalf("json run %d differs from the first", i)
		}
	}

	// A copy shuffled with a fixed seed renders to the same bytes as ordered input.
	rng := rand.New(rand.NewSource(1))
	shuffled := copyReport(ordered)
	for _, ar := range shuffled.Accounts {
		rng.Shuffle(len(ar.Findings), func(i, j int) {
			ar.Findings[i], ar.Findings[j] = ar.Findings[j], ar.Findings[i]
		})
	}

	var textShuffled, jsonShuffled bytes.Buffer
	if err := Text(&textShuffled, shuffled); err != nil {
		t.Fatalf("shuffled text: %v", err)
	}
	if err := JSON(&jsonShuffled, shuffled); err != nil {
		t.Fatalf("shuffled json: %v", err)
	}
	if !bytes.Equal(textShuffled.Bytes(), textWant) {
		t.Fatal("shuffled text output differs from ordered input")
	}
	if !bytes.Equal(jsonShuffled.Bytes(), jsonWant) {
		t.Fatal("shuffled JSON output differs from ordered input")
	}
}
