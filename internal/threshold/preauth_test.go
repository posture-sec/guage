package threshold_test

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/threshold"
)

// TestPreAuthTxIsNotStandingControl pins the SignerPreAuthTx exclusion at the
// edges the shared table in threshold_test.go does not cover: a lone T... entry
// at the top tier, and a T... of equal weight beside a P... entry, where a
// regression that let the T... through would invent reachability.
func TestPreAuthTxIsNotStandingControl(t *testing.T) {
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
			name:      "a lone pre-auth signer at threshold 0 authorizes nothing",
			signers:   []model.Signer{signerOf(model.SignerPreAuthTx, 255)},
			threshold: 0,
			want:      false,
		},
		{
			name:      "a lone pre-auth signer at threshold 1 authorizes nothing",
			signers:   []model.Signer{signerOf(model.SignerPreAuthTx, 255)},
			threshold: 1,
			want:      false,
		},
		{
			name:      "a lone pre-auth signer at the top tier authorizes nothing",
			signers:   []model.Signer{signerOf(model.SignerPreAuthTx, 255)},
			threshold: 255,
			want:      false,
		},
		{
			name: "a signed payload carries the tier beside a pre-auth signer",
			signers: []model.Signer{
				signerOf(model.SignerPreAuthTx, 255),
				signerOf(model.SignerSignedPayload, 1),
			},
			threshold: 1,
			want:      true,
			wantMin:   1,
			wantOK:    true,
		},
		{
			name: "a pre-auth signer adds nothing beside a weight-10 key",
			signers: []model.Signer{
				signerOf(model.SignerPreAuthTx, 255),
				signerOf(model.SignerEd25519, 10),
			},
			threshold: 10,
			want:      true,
			wantMin:   1,
			wantOK:    true,
		},
		{
			name: "a pre-auth signer cannot lift a weight-10 key to 11",
			signers: []model.Signer{
				signerOf(model.SignerPreAuthTx, 255),
				signerOf(model.SignerEd25519, 10),
			},
			threshold: 11,
			want:      false,
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
