package threshold_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/threshold"
)

// crossCheckSeed fixes the random lists below so a failure reproduces.
const crossCheckSeed = 20261119

// envelopeSignatures is the protocol's DecoratedSignature limit, restated
// here so the reference does not borrow the constant it is checking.
const envelopeSignatures = 20

var (
	crossTypes   = []model.SignerType{model.SignerEd25519, model.SignerHashX, model.SignerPreAuthTx, model.SignerSignedPayload}
	crossWeights = []int{0, 1, 2, 254, 255}
)

// TestThresholdCrossCheckMixedSigners compares Reachable and MinSigners with a
// subset search over lists that mix every signer type with the edge weights 0,
// 1, 2, 254 and 255. Every list of one to three signers is enumerated (8,420
// lists), then seeded random lists of four to twelve signers follow. The
// reference applies the zero-weight filter and the pre-auth exclusion itself
// and never sees the package's own filter, so a change to either side of that
// filter disagrees here.
func TestThresholdCrossCheckMixedSigners(t *testing.T) {
	t.Parallel()
	checked := 0
	for size := 1; size <= 3; size++ {
		forEachList(size, func(list []model.Signer) {
			crossCheckSubsets(t, list)
			checked++
		})
	}
	rng := rand.New(rand.NewSource(crossCheckSeed))
	for caseIndex := 0; caseIndex < 3000; caseIndex++ {
		list := make([]model.Signer, 4+rng.Intn(9))
		for i := range list {
			list[i] = crossSigner(i, crossTypes[rng.Intn(len(crossTypes))], crossWeights[rng.Intn(len(crossWeights))])
		}
		crossCheckSubsets(t, list)
		checked++
	}
	if checked != 8420+3000 {
		t.Fatalf("checked %d lists, want %d", checked, 8420+3000)
	}
}

// TestThresholdCrossCheckEnvelopeCap compares both functions on accounts with
// more participating weights than an envelope holds. The reference is a
// dynamic program indexed by signature count: best[k] is the largest sum any
// k eligible signers reach, and k never exceeds 20. It does not sort, so it
// also catches a build that keeps the first 20 weights read instead of the
// largest 20.
func TestThresholdCrossCheckEnvelopeCap(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(crossCheckSeed))
	for caseIndex := 0; caseIndex < 400; caseIndex++ {
		list := make([]model.Signer, 24)
		for i := range list {
			weight := crossWeights[rng.Intn(len(crossWeights))]
			if rng.Intn(2) == 0 {
				weight = rng.Intn(256)
			}
			list[i] = crossSigner(i, crossTypes[rng.Intn(len(crossTypes))], weight)
		}
		best := cappedBest(list)
		tiers := []int{0, 1}
		for k := 1; k <= envelopeSignatures; k++ {
			if best[k] >= 0 {
				tiers = append(tiers, best[k], best[k]+1)
			}
		}
		for _, tier := range tiers {
			wantReach, wantMin, wantOK := fromCappedBest(best, tier)
			if disagrees(list, tier, wantReach, wantMin, wantOK) {
				t.Fatalf("case %d (seed %d): %s", caseIndex, crossCheckSeed, describe(list, tier, wantReach, wantMin, wantOK))
			}
		}
	}
}

// forEachList calls visit with every ordered list of size signers drawn from
// the four types and five edge weights.
func forEachList(size int, visit func([]model.Signer)) {
	options := len(crossTypes) * len(crossWeights)
	digits := make([]int, size)
	for {
		list := make([]model.Signer, size)
		for i, digit := range digits {
			list[i] = crossSigner(i, crossTypes[digit/len(crossWeights)], crossWeights[digit%len(crossWeights)])
		}
		visit(list)
		i := 0
		for ; i < size; i++ {
			digits[i]++
			if digits[i] < options {
				break
			}
			digits[i] = 0
		}
		if i == size {
			return
		}
	}
}

func crossSigner(index int, typ model.SignerType, weight int) model.Signer {
	return model.Signer{Key: fmt.Sprintf("key-%d-%s-%d", index, typ, weight), Type: typ, Weight: weight}
}

// crossCheckSubsets checks every interesting tier for one list of at most 12
// signers against the subset search, and reports a shrunk counterexample.
func crossCheckSubsets(t *testing.T, list []model.Signer) {
	t.Helper()
	atLeast, total := subsetMinimums(list)
	tiers := []int{0, 1, 2, 3, 4, 253, 254, 255, 256, 257, 508, 509, 510, 511, 512, total - 1, total, total + 1}
	for _, tier := range tiers {
		if tier < 0 {
			continue
		}
		wantReach, wantMin, wantOK := fromSubsetMinimums(atLeast, tier)
		if disagrees(list, tier, wantReach, wantMin, wantOK) {
			small := shrink(list, tier)
			r, m, ok := subsetAnswer(small, tier)
			t.Fatalf("minimal counterexample: %s", describe(small, tier, r, m, ok))
		}
	}
}

// eligible states the filter the way ARCHITECTURE.md 6.3 does: a weight-0
// entry is not a signer and a pre-authorized transaction is not standing
// control. It is written here rather than borrowed from the package.
func eligible(list []model.Signer) []int {
	var weights []int
	for _, entry := range list {
		if entry.Weight > 0 && entry.Type != model.SignerPreAuthTx {
			weights = append(weights, entry.Weight)
		}
	}
	return weights
}

// subsetMinimums enumerates every non-empty subset of the eligible weights and
// returns atLeast, where atLeast[s] is the fewest signers whose weights sum to
// s or more (0 when none do), plus the total of all eligible weights. Lists
// here stay at or under 12 signers, under the envelope cap, so every subset is
// one a transaction could present.
func subsetMinimums(list []model.Signer) ([]int, int) {
	weights := eligible(list)
	if len(weights) > envelopeSignatures {
		panic("subsetMinimums is only valid under the envelope cap")
	}
	total := 0
	for _, weight := range weights {
		total += weight
	}
	sums := make([]int, 1<<len(weights))
	sizes := make([]int, 1<<len(weights))
	atLeast := make([]int, total+1)
	for mask := 1; mask < len(sums); mask++ {
		low := mask & -mask
		bit := 0
		for low>>bit != 1 {
			bit++
		}
		rest := mask &^ low
		sums[mask] = sums[rest] + weights[bit]
		sizes[mask] = sizes[rest] + 1
		if current := atLeast[sums[mask]]; current == 0 || sizes[mask] < current {
			atLeast[sums[mask]] = sizes[mask]
		}
	}
	// Fold exact sums into "this sum or more", walking down from the total.
	for sum := total - 1; sum >= 0; sum-- {
		if next := atLeast[sum+1]; next != 0 && (atLeast[sum] == 0 || next < atLeast[sum]) {
			atLeast[sum] = next
		}
	}
	return atLeast, total
}

// fromSubsetMinimums answers a tier from the subset table: any subset whose
// sum reaches the tier authorizes it, and tier 0 still needs one signer, which
// the table encodes because it records only non-empty subsets.
func fromSubsetMinimums(atLeast []int, tier int) (bool, int, bool) {
	if tier >= len(atLeast) || atLeast[tier] == 0 {
		return false, 0, false
	}
	return true, atLeast[tier], true
}

func subsetAnswer(list []model.Signer, tier int) (bool, int, bool) {
	atLeast, _ := subsetMinimums(list)
	return fromSubsetMinimums(atLeast, tier)
}

// cappedBest returns best[k], the largest sum of exactly k eligible weights,
// for k from 0 to the envelope cap, with -1 where no k signers exist. It is a
// 0/1 knapsack over signature count, so it chooses weights rather than
// sorting them.
func cappedBest(list []model.Signer) []int {
	best := make([]int, envelopeSignatures+1)
	for k := 1; k < len(best); k++ {
		best[k] = -1
	}
	for _, weight := range eligible(list) {
		for k := envelopeSignatures; k >= 1; k-- {
			if best[k-1] >= 0 && best[k-1]+weight > best[k] {
				best[k] = best[k-1] + weight
			}
		}
	}
	return best
}

func fromCappedBest(best []int, tier int) (bool, int, bool) {
	for k := 1; k < len(best); k++ {
		if best[k] >= 0 && best[k] >= tier {
			return true, k, true
		}
	}
	return false, 0, false
}

func disagrees(list []model.Signer, tier int, wantReach bool, wantMin int, wantOK bool) bool {
	if threshold.Reachable(list, tier) != wantReach {
		return true
	}
	fewest, ok := threshold.MinSigners(list, tier)
	return fewest != wantMin || ok != wantOK
}

// shrink drops signers one at a time while the disagreement survives, so a
// failure names the smallest list this greedy pass can find.
func shrink(list []model.Signer, tier int) []model.Signer {
	current := append([]model.Signer(nil), list...)
	for changed := true; changed; {
		changed = false
		for i := range current {
			candidate := append(append([]model.Signer(nil), current[:i]...), current[i+1:]...)
			r, m, ok := subsetAnswer(candidate, tier)
			if disagrees(candidate, tier, r, m, ok) {
				current, changed = candidate, true
				break
			}
		}
	}
	return current
}

func describe(list []model.Signer, tier int, wantReach bool, wantMin int, wantOK bool) string {
	fewest, ok := threshold.MinSigners(list, tier)
	pairs := make([]string, len(list))
	for i, entry := range list {
		pairs[i] = fmt.Sprintf("%s:%d", entry.Type, entry.Weight)
	}
	return fmt.Sprintf("tier %d over %v: Reachable = %v, want %v; MinSigners = (%d, %v), want (%d, %v)",
		tier, pairs, threshold.Reachable(list, tier), wantReach, fewest, ok, wantMin, wantOK)
}
