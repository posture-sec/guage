package threshold_test

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/threshold"
)

// TestMinSignersBoundaries pins the order of MinSigners' two early returns: a
// threshold at or below zero needs one signer, but only after the participating
// list has been shown to be non-empty. Moving the zero check above the empty
// check would claim a signature for an account with no signing weight.
func TestMinSignersBoundaries(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name      string
		signers   []model.Signer
		threshold int
		want      int
		wantOK    bool
	}{
		{
			name:      "an empty list at threshold 0 needs nothing",
			signers:   nil,
			threshold: 0,
			want:      0,
			wantOK:    false,
		},
		{
			name:      "an empty list at threshold 1 needs nothing",
			signers:   nil,
			threshold: 1,
			want:      0,
			wantOK:    false,
		},
		{
			name:      "an empty list at the top tier needs nothing",
			signers:   nil,
			threshold: 255,
			want:      0,
			wantOK:    false,
		},
		{
			name:      "a lone zero-weight master at threshold 0 is not a signer",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 0)},
			threshold: 0,
			want:      0,
			wantOK:    false,
		},
		{
			name:      "a lone zero-weight master at threshold 1 is not a signer",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 0)},
			threshold: 1,
			want:      0,
			wantOK:    false,
		},
		{
			name:      "a lone zero-weight master at the top tier is not a signer",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 0)},
			threshold: 255,
			want:      0,
			wantOK:    false,
		},
		{
			name:      "one weighted signer at threshold 0 needs one match",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 1)},
			threshold: 0,
			want:      1,
			wantOK:    true,
		},
		{
			name:      "one weighted signer at threshold 1 needs one match",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 1)},
			threshold: 1,
			want:      1,
			wantOK:    true,
		},
		{
			name:      "a weight-255 signer at threshold 1 needs one match",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 255)},
			threshold: 1,
			want:      1,
			wantOK:    true,
		},
		{
			name:      "a weight-255 signer at threshold 254 needs one match",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 255)},
			threshold: 254,
			want:      1,
			wantOK:    true,
		},
		{
			name:      "a weight-255 signer at the top tier needs one match",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 255)},
			threshold: 255,
			want:      1,
			wantOK:    true,
		},
		{
			name:      "a weight-255 signer cannot reach threshold 256",
			signers:   []model.Signer{signerOf(model.SignerEd25519, 255)},
			threshold: 256,
			want:      0,
			wantOK:    false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fewest, ok := threshold.MinSigners(tt.signers, tt.threshold)
			if fewest != tt.want || ok != tt.wantOK {
				t.Errorf("MinSigners = (%d, %v), want (%d, %v)", fewest, ok, tt.want, tt.wantOK)
			}
		})
	}
}
