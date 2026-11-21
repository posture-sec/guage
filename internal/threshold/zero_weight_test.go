package threshold_test

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/threshold"
)

// TestZeroWeightIsAbsent pins the weight filter at every tier: a master at
// weight zero is absent rather than weight zero, so it neither authorizes a
// zero tier nor adds to a tier another signer has to reach. A negative weight
// is skipped the same way.
func TestZeroWeightIsAbsent(t *testing.T) {
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
			name:      "a lone zero-weight master at threshold 0 authorizes nothing",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 0)},
			threshold: 0,
			want:      false,
		},
		{
			name:      "a lone zero-weight master at threshold 1 authorizes nothing",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 0)},
			threshold: 1,
			want:      false,
		},
		{
			name:      "a lone zero-weight master at the top tier authorizes nothing",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 0)},
			threshold: 255,
			want:      false,
		},
		{
			name: "a zero-weight master leaves a weight-5 signer alone at threshold 0",
			signers: []model.Signer{
				signerOf(model.SignerEd25519, 0),
				signerOf(model.SignerEd25519, 5),
			},
			threshold: 0,
			want:      true,
			wantMin:   1,
			wantOK:    true,
		},
		{
			name: "a zero-weight master leaves a weight-5 signer alone at threshold 5",
			signers: []model.Signer{
				signerOf(model.SignerEd25519, 0),
				signerOf(model.SignerEd25519, 5),
			},
			threshold: 5,
			want:      true,
			wantMin:   1,
			wantOK:    true,
		},
		{
			name: "a zero-weight master does not lift a weight-5 signer to threshold 6",
			signers: []model.Signer{
				signerOf(model.SignerEd25519, 0),
				signerOf(model.SignerEd25519, 5),
			},
			threshold: 6,
			want:      false,
		},
		{
			name: "a zero-weight master does not lift a weight-5 signer to the top tier",
			signers: []model.Signer{
				signerOf(model.SignerEd25519, 0),
				signerOf(model.SignerEd25519, 5),
			},
			threshold: 255,
			want:      false,
		},
		{
			name:      "a lone negative-weight signer at threshold 0 authorizes nothing",
			signers:   []model.Signer{signerOf(model.SignerEd25519, -5)},
			threshold: 0,
			want:      false,
		},
		{
			name: "a negative weight is skipped beside a weight-5 signer at threshold 5",
			signers: []model.Signer{
				signerOf(model.SignerEd25519, -5),
				signerOf(model.SignerEd25519, 5),
			},
			threshold: 5,
			want:      true,
			wantMin:   1,
			wantOK:    true,
		},
		{
			name: "a negative weight does not lift a weight-5 signer to threshold 6",
			signers: []model.Signer{
				signerOf(model.SignerEd25519, -5),
				signerOf(model.SignerEd25519, 5),
			},
			threshold: 6,
			want:      false,
		},
		{
			name: "zero and negative weights together leave the weighted signer alone",
			signers: []model.Signer{
				signerOf(model.SignerEd25519, 0),
				signerOf(model.SignerEd25519, -5),
				signerOf(model.SignerEd25519, 5),
			},
			threshold: 5,
			want:      true,
			wantMin:   1,
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

// TestZeroAndNegativeWeightsDoNotChangeTheAnswer states the mixed-list half of
// the same rule directly: adding entries at weight zero or below to a list must
// answer exactly as the list without them.
func TestZeroAndNegativeWeightsDoNotChangeTheAnswer(t *testing.T) {
	t.Parallel()
	base := []model.Signer{signerOf(model.SignerEd25519, 5)}
	mixed := []model.Signer{
		signerOf(model.SignerEd25519, 0),
		signerOf(model.SignerEd25519, -5),
		signerOf(model.SignerEd25519, 5),
	}
	for _, tier := range []int{0, 1, 5, 6, 255} {
		wantReachable := threshold.Reachable(base, tier)
		wantMin, wantOK := threshold.MinSigners(base, tier)
		if got := threshold.Reachable(mixed, tier); got != wantReachable {
			t.Errorf("Reachable at %d = %v, want %v from the base list", tier, got, wantReachable)
		}
		gotMin, gotOK := threshold.MinSigners(mixed, tier)
		if gotMin != wantMin || gotOK != wantOK {
			t.Errorf("MinSigners at %d = (%d, %v), want (%d, %v) from the base list", tier, gotMin, gotOK, wantMin, wantOK)
		}
	}
}
