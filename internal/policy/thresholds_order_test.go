package policy

import (
	"errors"
	"strings"
	"testing"
)

// TestOutOfOrderThresholdsLoad records a decision rather than a fix. Account.validate
// range-checks each declared tier and never compares low, medium and high, because
// an account whose stored tiers are out of order is what sig-threshold-order
// reports. A declaration mirroring such an account must stay loadable, or an
// operator cannot audit the account that rule was written for, so an ordering check
// added here as a tidy-up would turn a legitimate policy into exit 2. The control
// row keeps the range check honest: 256 is still a config error.
func TestOutOfOrderThresholdsLoad(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		block string
		want  error
	}{
		{
			name:  "out of order tiers load",
			block: "expected_thresholds = { low = 3, medium = 1, high = 0 }",
		},
		{
			name:  "high above the range is rejected",
			block: "expected_thresholds = { high = 256 }",
			want:  ErrThreshold,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := Load(strings.NewReader(
				"version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + accountOne + "\"\n" + tt.block + "\n"))
			if tt.want != nil {
				if !errors.Is(err, tt.want) {
					t.Fatalf("error = %v, want %v", err, tt.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			th := cfg.Accounts[0].ExpectedThresholds
			if th == nil || th.Low == nil || th.Medium == nil || th.High == nil {
				t.Fatalf("thresholds did not survive the load: %+v", th)
			}
			if *th.Low != 3 || *th.Medium != 1 || *th.High != 0 {
				t.Fatalf("thresholds = %d/%d/%d, want 3/1/0", *th.Low, *th.Medium, *th.High)
			}
		})
	}
}
