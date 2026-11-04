package policy

import (
	"errors"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

func TestAcceptableIsExactlyTheNonAcceptableList(t *testing.T) {
	t.Parallel()
	// The three rules an accepted risk may never silence. This is the same table
	// internal/run consults before matching an acceptance and validate.go consults
	// before loading one, so a fourth entry added here must be refused in both.
	for _, rule := range []string{"acct-missing", "acct-network-mismatch", "sig-threshold-unreachable"} {
		if Acceptable(rule) {
			t.Errorf("%s is non-acceptable and must not be", rule)
		}
	}
	for _, rule := range []string{
		"sig-no-signing-weight", "sig-threshold-order", "sig-single-key-high", "sig-hashx",
		"sig-unexpected-signer", "sig-missing-expected", "sig-weight-drift", "thr-drift",
		"flag-drift", "flag-undeclared-clawback", "id-no-home-domain", "id-toml-unreachable",
		"id-toml-account-unlisted",
	} {
		if !Acceptable(rule) {
			t.Errorf("%s is acceptable and must not be refused", rule)
		}
	}
}

// Each non-acceptable rule is refused at load, not merely ignored at match time:
// an operator who writes one of these entries has to be told the acceptance will
// never hold rather than read a clean report that is quietly not clean.
func TestLoadRefusesEveryNonAcceptableRule(t *testing.T) {
	t.Parallel()
	for _, rule := range []string{"acct-missing", "acct-network-mismatch", "sig-threshold-unreachable"} {
		body := `version = 1
network = "testnet"
endpoints = ["https://horizon.testnet.example"]

[[account]]
id = "` + accountOne + `"

[[accept]]
rule = "` + rule + `"
account = "` + accountOne + `"
reason = "a decision that cannot be recorded this way"
`
		_, err := Load(strings.NewReader(body))
		if !errors.Is(err, ErrAcceptRule) {
			t.Errorf("accepting %s: err = %v, want ErrAcceptRule", rule, err)
		}
	}
}

// An expiry is a date the loader can parse or it is refused. The day itself is
// inclusive and the comparison is by UTC calendar day, which is internal/run's
// business; what load guarantees is that run never receives a string it cannot
// compare.
func TestAcceptanceExpiryShape(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		expires string
		wantErr error
	}{
		{"plain date", "2027-01-01", nil},
		{"leap day", "2028-02-29", nil},
		{"month out of range", "2027-13-01", ErrAccept},
		{"day out of range", "2027-04-31", ErrAccept},
		{"datetime", "2027-01-01T00:00:00Z", ErrAccept},
		{"day element not padded", "2027-1-01", ErrAccept},
		{"slash separated", "2027/01/01", ErrAccept},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := `version = 1
network = "testnet"
endpoints = ["https://horizon.testnet.example"]

[[account]]
id = "` + accountOne + `"

[[accept]]
rule = "flag-drift"
account = "` + accountOne + `"
reason = "known drift"
expires = "` + tc.expires + `"
`
			_, err := Load(strings.NewReader(body))
			if tc.wantErr == nil && err != nil {
				t.Fatalf("loading a valid expiry: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// The role column of a policy is the only place severity by role can come from, so
// an unset role is the other row rather than an unclassified account, and a value
// outside the five is refused.
func TestRoleDefaultsAndRefusals(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		line    string
		want    model.Role
		wantErr error
	}{
		{"unset role is other", "", model.RoleOther, nil},
		{"issuer", "role = \"issuer\"", model.RoleIssuer, nil},
		{"treasury", "role = \"treasury\"", model.RoleTreasury, nil},
		{"distribution", "role = \"distribution\"", model.RoleDistribution, nil},
		{"merchant", "role = \"merchant\"", model.RoleMerchant, nil},
		{"other", "role = \"other\"", model.RoleOther, nil},
		{"unknown role", "role = \"custodian\"", "", ErrRole},
		{"wrong case", "role = \"Issuer\"", "", ErrRole},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := "version = 1\nnetwork = \"testnet\"\nendpoints = [\"https://horizon.testnet.example\"]\n\n[[account]]\nid = \"" + accountOne + "\"\n" + tc.line + "\n"
			cfg, err := Load(strings.NewReader(body))
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := cfg.Accounts[0].Role; got != tc.want {
				t.Errorf("role = %q, want %q", got, tc.want)
			}
		})
	}
}
