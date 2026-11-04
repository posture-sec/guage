package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
)

const secondAccount = "GBK4NFJ7SVMUNPBIYBC4WNMCTKI6Z66MYYWZQSSLQNOSUSTMKVUKE7M3"

func signerEntry(key string, typ model.SignerType, weight int) model.Signer {
	return model.Signer{Key: key, Type: typ, Weight: weight}
}

func snapshotWith(thresholds model.Thresholds, signers ...model.Signer) model.AccountSnapshot {
	return model.AccountSnapshot{ID: testAccount, Thresholds: thresholds, Signers: signers}
}

func contextWith(account policy.Account, snapshot model.AccountSnapshot) Context {
	return Context{Account: account, Snapshot: snapshot, Network: testnetPassphrase, Expected: testnetPassphrase}
}

func expected(list ...policy.ExpectedSigner) *[]policy.ExpectedSigner {
	declared := list
	return &declared
}

func TestSigNoSigningWeight(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name      string
		account   policy.Account
		snapshot  model.AccountSnapshot
		wantCount int
		wantSev   model.Severity
	}{
		{
			name:      "no signers at all",
			snapshot:  snapshotWith(model.Thresholds{}),
			wantCount: 1,
			wantSev:   model.SevHigh,
		},
		{
			name:      "a weight-0 master alone",
			snapshot:  snapshotWith(model.Thresholds{}, signerEntry(testAccount, model.SignerEd25519, 0)),
			wantCount: 1,
			wantSev:   model.SevHigh,
		},
		{
			name:      "a pre-auth entry at weight is not standing control",
			snapshot:  snapshotWith(model.Thresholds{}, signerEntry(testAccount, model.SignerPreAuthTx, 255)),
			wantCount: 1,
			wantSev:   model.SevHigh,
		},
		{
			name:      "one weighted key clears it",
			snapshot:  snapshotWith(model.Thresholds{}, signerEntry(testAccount, model.SignerEd25519, 1)),
			wantCount: 0,
		},
		{
			name:      "an issuer that declares the lock reads it as intended",
			account:   policy.Account{Role: model.RoleIssuer, ExpectLocked: boolean(true)},
			snapshot:  snapshotWith(model.Thresholds{}),
			wantCount: 1,
			wantSev:   model.SevInfo,
		},
		{
			name:      "a locked declaration without the issuer role stays HIGH",
			account:   policy.Account{Role: model.RoleOther, ExpectLocked: boolean(true)},
			snapshot:  snapshotWith(model.Thresholds{}),
			wantCount: 1,
			wantSev:   model.SevHigh,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sigNoSigningWeight(contextWith(tt.account, tt.snapshot))
			if len(got) != tt.wantCount {
				t.Fatalf("got %d findings, want %d: %+v", len(got), tt.wantCount, got)
			}
			if tt.wantCount > 0 && got[0].Severity != tt.wantSev {
				t.Errorf("severity = %s, want %s", got[0].Severity, tt.wantSev)
			}
		})
	}
}

func TestSigSingleKeyHigh(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name       string
		snapshot   model.AccountSnapshot
		wantCount  int
		wantInEvid string
	}{
		{
			name:       "thresholds all zero with one key is single-key control",
			snapshot:   snapshotWith(model.Thresholds{}, signerEntry(testAccount, model.SignerEd25519, 1)),
			wantCount:  1,
			wantInEvid: "the high threshold is 0",
		},
		{
			name: "one key that alone reaches high",
			snapshot: snapshotWith(model.Thresholds{Low: 1, Medium: 1, High: 5},
				signerEntry(testAccount, model.SignerEd25519, 5),
				signerEntry(secondAccount, model.SignerEd25519, 1)),
			wantCount:  1,
			wantInEvid: "at weight 5 reaches 5",
		},
		{
			name: "two keys are needed, so it is not single-key",
			snapshot: snapshotWith(model.Thresholds{Low: 1, Medium: 1, High: 5},
				signerEntry(testAccount, model.SignerEd25519, 3),
				signerEntry(secondAccount, model.SignerEd25519, 2)),
			wantCount: 0,
		},
		{
			name: "a pre-auth entry that would reach high on its own is not standing control",
			snapshot: snapshotWith(model.Thresholds{Low: 1, Medium: 1, High: 5},
				signerEntry(testAccount, model.SignerPreAuthTx, 255)),
			wantCount: 0,
		},
		{
			name:      "a high tier nothing can reach is unreachable, not single-key",
			snapshot:  snapshotWith(model.Thresholds{High: 255}, signerEntry(testAccount, model.SignerEd25519, 1)),
			wantCount: 0,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sigSingleKeyHigh(contextWith(policy.Account{}, tt.snapshot))
			if len(got) != tt.wantCount {
				t.Fatalf("got %d findings, want %d: %+v", len(got), tt.wantCount, got)
			}
			if len(got) == 1 && got[0].Severity != model.SevInfo {
				t.Errorf("severity = %s, want info before the role map", got[0].Severity)
			}
			if tt.wantInEvid != "" && !strings.Contains(got[0].Evidence, tt.wantInEvid) {
				t.Errorf("evidence %q does not name %q", got[0].Evidence, tt.wantInEvid)
			}
		})
	}
}

func TestSigHashX(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name      string
		snapshot  model.AccountSnapshot
		wantCount int
		wantSev   model.Severity
	}{
		{
			name: "weight alone reaches medium",
			snapshot: snapshotWith(model.Thresholds{Low: 1, Medium: 3, High: 3},
				signerEntry(testAccount, model.SignerEd25519, 1),
				signerEntry(secondAccount, model.SignerHashX, 3)),
			wantCount: 1,
			wantSev:   model.SevHigh,
		},
		{
			name:      "weight short of medium",
			snapshot:  snapshotWith(model.Thresholds{Low: 1, Medium: 3, High: 3}, signerEntry(secondAccount, model.SignerHashX, 1)),
			wantCount: 1,
			wantSev:   model.SevWarn,
		},
		{
			name:      "a medium threshold of 0 is reached by any match, so the weight is not what does the work",
			snapshot:  snapshotWith(model.Thresholds{}, signerEntry(secondAccount, model.SignerHashX, 255)),
			wantCount: 1,
			wantSev:   model.SevWarn,
		},
		{
			name:      "no hash(x) signer",
			snapshot:  snapshotWith(model.Thresholds{Medium: 1}, signerEntry(secondAccount, model.SignerEd25519, 9)),
			wantCount: 0,
		},
		{
			name: "two hash(x) signers raise two findings",
			snapshot: snapshotWith(model.Thresholds{Medium: 2},
				signerEntry(secondAccount, model.SignerHashX, 2),
				signerEntry("XA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAACU5P6", model.SignerHashX, 1)),
			wantCount: 2,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sigHashX(contextWith(policy.Account{}, tt.snapshot))
			if len(got) != tt.wantCount {
				t.Fatalf("got %d findings, want %d: %+v", len(got), tt.wantCount, got)
			}
			if tt.wantSev != 0 && got[0].Severity != tt.wantSev {
				t.Errorf("severity = %s, want %s", got[0].Severity, tt.wantSev)
			}
			for _, finding := range got {
				if !strings.Contains(finding.Evidence, "medium threshold") {
					t.Errorf("evidence does not name the tier it judged: %q", finding.Evidence)
				}
			}
		})
	}
}

func TestSigUnexpectedSigner(t *testing.T) {
	t.Parallel()
	observed := snapshotWith(model.Thresholds{Low: 1, Medium: 1, High: 1},
		signerEntry(testAccount, model.SignerEd25519, 1),
		signerEntry(secondAccount, model.SignerEd25519, 1),
		signerEntry("X"+secondAccount[1:], model.SignerHashX, 1),
	)
	for _, tt := range []struct {
		name      string
		account   policy.Account
		wantCount int
		wantNames []string
	}{
		{
			name:      "nothing declared raises nothing",
			account:   policy.Account{},
			wantCount: 0,
		},
		{
			name:      "the same list declared raises nothing",
			account:   policy.Account{ExpectedSigners: expected(policy.ExpectedSigner{Key: testAccount, Weight: 1}, policy.ExpectedSigner{Key: secondAccount, Weight: 1}, policy.ExpectedSigner{Key: "X" + secondAccount[1:], Weight: 1})},
			wantCount: 0,
		},
		{
			name:      "two keys the policy does not name",
			account:   policy.Account{ExpectedSigners: expected(policy.ExpectedSigner{Key: testAccount, Weight: 1})},
			wantCount: 1,
			wantNames: []string{secondAccount, "X" + secondAccount[1:]},
		},
		{
			name: "a declared removal that the chain still shows weighted",
			account: policy.Account{ExpectedSigners: expected(
				policy.ExpectedSigner{Key: testAccount, Weight: 1},
				policy.ExpectedSigner{Key: secondAccount, Weight: 0},
				policy.ExpectedSigner{Key: "X" + secondAccount[1:], Weight: 1},
			)},
			wantCount: 1,
			wantNames: []string{secondAccount},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sigUnexpectedSigner(contextWith(tt.account, observed))
			if len(got) != tt.wantCount {
				t.Fatalf("got %d findings, want %d: %+v", len(got), tt.wantCount, got)
			}
			if tt.wantCount == 0 {
				return
			}
			if got[0].Severity != model.SevHigh {
				t.Errorf("severity = %s, want high", got[0].Severity)
			}
			for _, key := range tt.wantNames {
				if !strings.Contains(got[0].Evidence, shortenKey(key)) {
					t.Errorf("evidence %q does not name %s", got[0].Evidence, key)
				}
			}
		})
	}
}

func TestSigMissingExpected(t *testing.T) {
	t.Parallel()
	observed := snapshotWith(model.Thresholds{Low: 1, Medium: 1, High: 1},
		signerEntry(testAccount, model.SignerEd25519, 1),
		signerEntry(secondAccount, model.SignerEd25519, 0),
	)
	for _, tt := range []struct {
		name      string
		account   policy.Account
		wantCount int
	}{
		{
			name:      "nothing declared raises nothing",
			account:   policy.Account{},
			wantCount: 0,
		},
		{
			name: "a declared signer the chain does not show",
			account: policy.Account{ExpectedSigners: expected(
				policy.ExpectedSigner{Key: testAccount, Weight: 1},
				policy.ExpectedSigner{Key: "X" + secondAccount[1:], Weight: 1},
			)},
			wantCount: 1,
		},
		{
			name: "a declared signer the chain shows at weight 0",
			account: policy.Account{ExpectedSigners: expected(
				policy.ExpectedSigner{Key: testAccount, Weight: 1},
				policy.ExpectedSigner{Key: secondAccount, Weight: 2},
			)},
			wantCount: 1,
		},
		{
			name: "a declared signer present at its declared weight",
			account: policy.Account{ExpectedSigners: expected(
				policy.ExpectedSigner{Key: testAccount, Weight: 1},
			)},
			wantCount: 0,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sigMissingExpected(contextWith(tt.account, observed))
			if len(got) != tt.wantCount {
				t.Fatalf("got %d findings, want %d: %+v", len(got), tt.wantCount, got)
			}
			if tt.wantCount == 0 {
				return
			}
			if got[0].Severity != model.SevHigh {
				t.Errorf("severity = %s, want high", got[0].Severity)
			}
			if !strings.Contains(got[0].Evidence, "chain shows 0") {
				t.Errorf("evidence does not state the observed weight: %q", got[0].Evidence)
			}
		})
	}
}

func TestSigWeightDrift(t *testing.T) {
	t.Parallel()
	observed := snapshotWith(model.Thresholds{Low: 1, Medium: 1, High: 1},
		signerEntry(testAccount, model.SignerEd25519, 5),
		signerEntry(secondAccount, model.SignerEd25519, 1),
	)
	for _, tt := range []struct {
		name      string
		account   policy.Account
		wantCount int
	}{
		{
			name:      "nothing declared raises nothing",
			account:   policy.Account{},
			wantCount: 0,
		},
		{
			name: "a weight that moved",
			account: policy.Account{ExpectedSigners: expected(
				policy.ExpectedSigner{Key: testAccount, Weight: 3},
				policy.ExpectedSigner{Key: secondAccount, Weight: 1},
			)},
			wantCount: 1,
		},
		{
			name: "weights as declared",
			account: policy.Account{ExpectedSigners: expected(
				policy.ExpectedSigner{Key: testAccount, Weight: 5},
				policy.ExpectedSigner{Key: secondAccount, Weight: 1},
			)},
			wantCount: 0,
		},
		{
			name: "a signer the chain does not show is missing-expected, not drift",
			account: policy.Account{ExpectedSigners: expected(
				policy.ExpectedSigner{Key: testAccount, Weight: 5},
				policy.ExpectedSigner{Key: "X" + secondAccount[1:], Weight: 1},
			)},
			wantCount: 0,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sigWeightDrift(contextWith(tt.account, observed))
			if len(got) != tt.wantCount {
				t.Fatalf("got %d findings, want %d: %+v", len(got), tt.wantCount, got)
			}
			if tt.wantCount == 0 {
				return
			}
			if got[0].Severity != model.SevWarn {
				t.Errorf("severity = %s, want warn before the role map", got[0].Severity)
			}
			if strings.Contains(got[0].Evidence, "X"+shortenKey(secondAccount[1:])) {
				t.Errorf("drift named a signer the chain does not show: %q", got[0].Evidence)
			}
		})
	}
}

// TestSignerRulesOnACapturedAccount runs the declaration rules against the bytes
// task 0.6 saved, rather than against a hand-built list, so the signer shapes a
// real multisig account carries are what these rules are judged on.
func TestSignerRulesOnACapturedAccount(t *testing.T) {
	t.Parallel()
	signers, thresholds := capturedAccount(t, "7-extra-signer.json")
	snapshot := model.AccountSnapshot{ID: testAccount, Thresholds: thresholds, Signers: signers}

	if got := sigNoSigningWeight(contextWith(policy.Account{}, snapshot)); len(got) != 0 {
		t.Fatalf("a six-signer account raised %+v", got)
	}
	if got := sigThresholdUnreachable(contextWith(policy.Account{}, snapshot)); len(got) != 0 {
		t.Fatalf("thresholds 3/3/3 over six weights raised %+v", got)
	}
	if got := sigSingleKeyHigh(contextWith(policy.Account{}, snapshot)); len(got) != 0 {
		t.Fatalf("no single key reaches 3, so %+v is wrong", got)
	}

	declared := policy.Account{ExpectedSigners: expected(
		policy.ExpectedSigner{Key: signers[0].Key, Weight: signers[0].Weight},
		policy.ExpectedSigner{Key: signers[1].Key, Weight: signers[1].Weight},
		policy.ExpectedSigner{Key: signers[2].Key, Weight: signers[2].Weight},
	)}
	if got := sigUnexpectedSigner(contextWith(declared, snapshot)); len(got) != 1 {
		t.Fatalf("three of six keys declared, want one finding naming the rest: %+v", got)
	} else if !strings.Contains(got[0].Evidence, shortenKey(signers[3].Key)) {
		t.Errorf("evidence omits the fourth signer: %q", got[0].Evidence)
	}

	if got := Evaluate(contextWith(declared, snapshot)); len(got) != 1 {
		t.Errorf("the whole registry on this account raised %d findings, want the one declaration disagreement: %+v", len(got), got)
	}
}

func capturedAccount(t *testing.T, name string) ([]model.Signer, model.Thresholds) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "horizon", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	var payload struct {
		Thresholds struct {
			Low    int `json:"low_threshold"`
			Medium int `json:"med_threshold"`
			High   int `json:"high_threshold"`
		} `json:"thresholds"`
		Signers []struct {
			Key    string `json:"key"`
			Type   string `json:"type"`
			Weight int    `json:"weight"`
		} `json:"signers"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decoding fixture %s: %v", name, err)
	}
	signers := make([]model.Signer, 0, len(payload.Signers))
	for _, entry := range payload.Signers {
		typ := model.SignerEd25519
		if entry.Type == "sha256_hash" {
			typ = model.SignerHashX
		}
		signers = append(signers, signerEntry(entry.Key, typ, entry.Weight))
	}
	return signers, model.Thresholds{Low: payload.Thresholds.Low, Medium: payload.Thresholds.Medium, High: payload.Thresholds.High}
}
