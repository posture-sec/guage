package threshold_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/threshold"
)

func signerOf(typ model.SignerType, weight int) model.Signer {
	return model.Signer{Key: fmt.Sprintf("key-%s-%d", typ, weight), Type: typ, Weight: weight}
}

func repeat(typ model.SignerType, weight, count int) []model.Signer {
	out := make([]model.Signer, count)
	for i := range out {
		out[i] = signerOf(typ, weight)
		out[i].Key = fmt.Sprintf("key-%d", i)
	}
	return out
}

func TestReachableAndMinSigners(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name      string
		signers   []model.Signer
		threshold int
		want      bool
		wantMin   int
		wantOK    bool
	}{
		{
			name:      "no signers at threshold 0 authorizes nothing",
			signers:   nil,
			threshold: 0,
			want:      false,
		},
		{
			name:      "a weight-0 master alone is not a signer",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 0)},
			threshold: 0,
			want:      false,
		},
		{
			name:      "one weighted signer at threshold 0 needs one match",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 1)},
			threshold: 0,
			want:      true,
			wantMin:   1,
			wantOK:    true,
		},
		{
			name:      "a pre-auth signer alone is not standing control",
			signers:   []model.Signer{signerOf(model.SignerPreAuthTx, 255)},
			threshold: 0,
			want:      false,
		},
		{
			name:      "exact single weight reaches",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 5)},
			threshold: 5,
			want:      true,
			wantMin:   1,
			wantOK:    true,
		},
		{
			name:      "two weights sum to reach",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 3), signerOf(model.SignerEd25519, 2)},
			threshold: 5,
			want:      true,
			wantMin:   2,
			wantOK:    true,
		},
		{
			name:      "one short is unreachable",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 2), signerOf(model.SignerEd25519, 2)},
			threshold: 5,
			want:      false,
		},
		{
			name:      "a stored tier above every weight is unreachable",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 1)},
			threshold: 255,
			want:      false,
		},
		{
			name:      "hash(x) participates at its weight",
			signers:   []model.Signer{signerOf(model.SignerHashX, 4)},
			threshold: 4,
			want:      true,
			wantMin:   1,
			wantOK:    true,
		},
		{
			name:      "a signed payload participates beside the others",
			signers:   []model.Signer{signerOf(model.SignerHashX, 1), signerOf(model.SignerSignedPayload, 2)},
			threshold: 3,
			want:      true,
			wantMin:   2,
			wantOK:    true,
		},
		{
			name: "a pre-auth signer adds nothing to the sum",
			signers: []model.Signer{
				signerOf(model.SignerPreAuthTx, 255),
				signerOf(model.SignerEd25519, 1),
			},
			threshold: 2,
			want:      false,
		},
		{
			name: "a pre-auth signer does not join the minimum count",
			signers: []model.Signer{
				signerOf(model.SignerPreAuthTx, 255),
				signerOf(model.SignerEd25519, 1),
				signerOf(model.SignerHashX, 1),
			},
			threshold: 2,
			want:      true,
			wantMin:   2,
			wantOK:    true,
		},
		{
			name:      "zero-weight entries are skipped",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 0), signerOf(model.SignerEd25519, 5)},
			threshold: 5,
			want:      true,
			wantMin:   1,
			wantOK:    true,
		},
		{
			name:      "twenty signatures fill the envelope",
			signers:   repeat(model.SignerEd25519, 12, 20),
			threshold: 240,
			want:      true,
			wantMin:   20,
			wantOK:    true,
		},
		{
			name:      "a full account is one weight past the envelope",
			signers:   append(repeat(model.SignerEd25519, 12, 20), signerOf(model.SignerHashX, 12)),
			threshold: 252,
			want:      false,
		},
		{
			name:      "the same account one tier lower",
			signers:   append(repeat(model.SignerEd25519, 12, 20), signerOf(model.SignerHashX, 12)),
			threshold: 240,
			want:      true,
			wantMin:   20,
			wantOK:    true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := threshold.Reachable(tt.signers, tt.threshold); got != tt.want {
				t.Errorf("Reachable = %v, want %v", got, tt.want)
			}
			fewest, ok := threshold.MinSigners(tt.signers, tt.threshold)
			if fewest != tt.wantMin || ok != tt.wantOK {
				t.Errorf("MinSigners = (%d, %v), want (%d, %v)", fewest, ok, tt.wantMin, tt.wantOK)
			}
		})
	}
}

// TestThresholdAgainstBruteForce checks the greedy claim ARCHITECTURE.md 6.3
// makes for Reachable and MinSigners: taking the largest weights first reaches
// a tier when any set can, and does so with the fewest signers. The comparison
// enumerates subsets rather than sorting, so it disagrees with the
// implementation by construction if the ordering rule is wrong. Signer sets
// stay under the 20-signature cap on purpose, because the cap is what the
// greedy rule is truncated by and the table above names those cases.
func TestThresholdAgainstBruteForce(t *testing.T) {
	t.Parallel()
	types := []model.SignerType{model.SignerEd25519, model.SignerHashX, model.SignerPreAuthTx, model.SignerSignedPayload}
	rng := rand.New(rand.NewSource(20261104))
	for caseIndex := 0; caseIndex < 5000; caseIndex++ {
		list := make([]model.Signer, 1+rng.Intn(10))
		for i := range list {
			list[i] = signerOf(types[rng.Intn(len(types))], rng.Intn(256))
		}
		tier := rng.Intn(300)
		want, wantMin, wantOK := bruteForce(list, tier)
		if got := threshold.Reachable(list, tier); got != want {
			t.Fatalf("case %d: Reachable(%v, %d) = %v, want %v", caseIndex, list, tier, got, want)
		}
		fewest, ok := threshold.MinSigners(list, tier)
		if fewest != wantMin || ok != wantOK {
			t.Fatalf("case %d: MinSigners(%v, %d) = (%d, %v), want (%d, %v)", caseIndex, list, tier, fewest, ok, wantMin, wantOK)
		}
	}
}

// bruteForce states the rule the way the protocol does, over subsets: a set of
// signers authorizes a tier when the sum of its weights reaches it, a tier of 0
// still needs one signer in the set, and a pre-authorized transaction entry is
// not standing control.
func bruteForce(signers []model.Signer, tier int) (reachable bool, fewest int, ok bool) {
	var weights []int
	for _, entry := range signers {
		if entry.Weight > 0 && entry.Type != model.SignerPreAuthTx {
			weights = append(weights, entry.Weight)
		}
	}
	for subset := 1; subset < 1<<len(weights); subset++ {
		total, size := 0, 0
		for i, weight := range weights {
			if subset&(1<<i) != 0 {
				total += weight
				size++
			}
		}
		if total >= tier {
			reachable = true
			if !ok || size < fewest {
				fewest, ok = size, true
			}
		}
	}
	return reachable, fewest, ok
}
