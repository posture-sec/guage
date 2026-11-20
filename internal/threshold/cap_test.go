package threshold_test

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/threshold"
)

// TestEnvelopeCap pins the 20-signature envelope cap from ARCHITECTURE.md 6.3
// at its boundary. An account can hold 20 signers plus a nonzero master, so
// it can carry 21 participating weights while a transaction presents at most
// 20 signatures. The rows check two things the totals in
// TestReachableAndMinSigners do not: that the 21st weight never counts, and
// that the 20 weights kept are the largest rather than the first 20 read.
// The large weight in the mixed rows is listed last on purpose, so a build
// that truncates before it sorts drops it and fails here.
func TestEnvelopeCap(t *testing.T) {
	t.Parallel()
	masterOne := signerOf(model.SignerEd25519, 1)
	big := signerOf(model.SignerEd25519, 255)
	for _, tt := range []struct {
		name      string
		signers   []model.Signer
		threshold int
		want      bool
		wantMin   int
		wantOK    bool
	}{
		{
			name:      "21 equal weights reach tier 20 with all 20 signatures",
			signers:   repeat(model.SignerEd25519, 1, 21),
			threshold: 20,
			want:      true,
			wantMin:   20,
			wantOK:    true,
		},
		{
			name:      "21 equal weights cannot reach tier 21",
			signers:   repeat(model.SignerEd25519, 1, 21),
			threshold: 21,
			want:      false,
		},
		{
			name:      "21 equal weights of mixed types cannot reach tier 21",
			signers:   append(repeat(model.SignerHashX, 1, 11), repeat(model.SignerSignedPayload, 1, 10)...),
			threshold: 21,
			want:      false,
		},
		{
			name:      "20 weight-10 signers plus a weight-1 master reach 200",
			signers:   append(repeat(model.SignerEd25519, 10, 20), masterOne),
			threshold: 200,
			want:      true,
			wantMin:   20,
			wantOK:    true,
		},
		{
			name:      "20 weight-10 signers plus a weight-1 master cannot reach 201",
			signers:   append(repeat(model.SignerEd25519, 10, 20), masterOne),
			threshold: 201,
			want:      false,
		},
		{
			name:      "a weight-255 signer read last is kept and suffices alone",
			signers:   append(repeat(model.SignerEd25519, 1, 20), big),
			threshold: 255,
			want:      true,
			wantMin:   1,
			wantOK:    true,
		},
		{
			name:      "a weight-255 signer read last plus 19 weight-1 signers reach 274",
			signers:   append(repeat(model.SignerEd25519, 1, 20), big),
			threshold: 274,
			want:      true,
			wantMin:   20,
			wantOK:    true,
		},
		{
			name:      "a weight-255 signer plus 20 weight-1 signers cannot reach 275",
			signers:   append(repeat(model.SignerEd25519, 1, 20), big),
			threshold: 275,
			want:      false,
		},
		{
			name:      "a pre-auth entry does not take an envelope slot",
			signers:   append(repeat(model.SignerEd25519, 1, 20), signerOf(model.SignerPreAuthTx, 255)),
			threshold: 20,
			want:      true,
			wantMin:   20,
			wantOK:    true,
		},
		{
			name:      "zero weights do not take an envelope slot",
			signers:   append(repeat(model.SignerEd25519, 0, 5), repeat(model.SignerEd25519, 1, 20)...),
			threshold: 20,
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
