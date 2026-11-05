package threshold

import (
	"sort"

	"github.com/posture-sec/guage/internal/model"
)

// maxSignatures is the cap an envelope carries: DecoratedSignature signatures<20>.
// An account can hold MAX_SIGNERS entries plus a nonzero master, so a full
// account has one participating weight more than a transaction can present.
const maxSignatures = 20

// Reachable reports whether any set of signers can authorize at this threshold.
func Reachable(signers []model.Signer, threshold int) bool {
	weights := participating(signers)
	if len(weights) == 0 {
		return false
	}
	if threshold <= 0 {
		return true
	}
	total := 0
	for _, weight := range weights {
		total += weight
	}
	return total >= threshold
}

// MinSigners returns the smallest number of signers whose weights reach the threshold.
func MinSigners(signers []model.Signer, threshold int) (int, bool) {
	weights := participating(signers)
	if len(weights) == 0 {
		return 0, false
	}
	if threshold <= 0 {
		return 1, true
	}
	total := 0
	for i, weight := range weights {
		total += weight
		if total >= threshold {
			return i + 1, true
		}
	}
	return 0, false
}

func participating(signers []model.Signer) []int {
	weights := make([]int, 0, len(signers))
	for _, signer := range signers {
		if signer.Weight <= 0 || signer.Type == model.SignerPreAuthTx {
			continue
		}
		weights = append(weights, signer.Weight)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(weights)))
	if len(weights) > maxSignatures {
		weights = weights[:maxSignatures]
	}
	return weights
}
