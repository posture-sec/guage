package policy

import (
	"errors"
	"strings"
	"testing"
	"time"

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
		"id-toml-account-unlisted", "id-sep29-memo-required",
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

// Date is the expiry shape as a type, so the two questions a caller asks about it
// are answered in one place. A value that is not a calendar day is not valid, and
// only a valid value has a last day to hold on.
func TestDateValid(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		date Date
		want bool
	}{
		{"2027-01-01", true},
		{"2028-02-29", true},
		{"", false},
		{"2027-13-01", false},
		{"2027-04-31", false},
		{"2027-1-01", false},
		{"2027/01/01", false},
		{"2027-01-01T00:00:00Z", false},
	} {
		if got := tc.date.Valid(); got != tc.want {
			t.Errorf("Date(%q).Valid() = %v, want %v", tc.date, got, tc.want)
		}
	}
}

// The named day is the last day the acceptance holds and the cutover is midnight
// UTC, which is the rule internal/run applies. The two fixed zones are what makes
// this a test rather than an assertion about Format: each row is one clock reading
// and one UTC reading, and the answer follows the UTC one.
func TestDateExpired(t *testing.T) {
	t.Parallel()
	expiry := Date("2026-11-04")
	east := time.FixedZone("UTC+8", 8*60*60)
	west := time.FixedZone("UTC-5", -5*60*60)
	for _, tc := range []struct {
		name string
		now  time.Time
		want bool
	}{
		{"the expiry day itself is still covered", time.Date(2026, 11, 4, 0, 0, 1, 0, time.UTC), false},
		{"the first minute after it is not", time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC), true},
		{"a clock behind UTC cannot widen the window", time.Date(2026, 11, 4, 20, 0, 0, 0, west), true},
		{"a clock ahead of UTC cannot narrow it", time.Date(2026, 11, 5, 2, 0, 0, 0, east), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := expiry.Expired(tc.now); got != tc.want {
				t.Errorf("Expired(%s) = %v, want %v", tc.now.Format(time.RFC3339), got, tc.want)
			}
		})
	}
}

// An acceptance may name the network it holds for. The value is validated with the
// same function as the top-level network, so an alias and a full passphrase are
// both accepted and a near miss is refused rather than silently matching nothing.
func TestAcceptanceNetworkShape(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		network string
		wantErr error
	}{
		{"unpinned keeps the pre-existing meaning", "", nil},
		{"mainnet alias", "mainnet", nil},
		{"testnet alias", "testnet", nil},
		{"full passphrase", "Public Global Stellar Network ; September 2015", nil},
		{"wrong case", "Mainnet", ErrNetwork},
		{"surrounding space", " mainnet", ErrNetwork},
		{"two words without a passphrase separator", "main net", ErrNetwork},
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
network = "` + tc.network + `"
reason = "known drift"
`
			_, err := Load(strings.NewReader(body))
			if tc.wantErr == nil && err != nil {
				t.Fatalf("loading network %q: %v", tc.network, err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("network %q: err = %v, want %v", tc.network, err, tc.wantErr)
			}
		})
	}
}

// An expiry that is not a date has no last day to hold on, so it reads as expired
// rather than as an acceptance that never ends. Load refuses such a value; the
// method is what keeps a hand-built entry from outliving it.
func TestDateExpiredOnMalformedValue(t *testing.T) {
	t.Parallel()
	if !Date("the 4th of November").Expired(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("a malformed expiry must read as expired")
	}
}

// One account may pin the same rule on both networks, because those are different
// statements; the same triple twice is still a duplicate.
func TestAcceptanceDedupeIncludesNetwork(t *testing.T) {
	t.Parallel()
	body := func(entries string) string {
		return `version = 1
network = "testnet"
endpoints = ["https://horizon.testnet.example"]

[[account]]
id = "` + accountOne + `"

` + entries
	}
	entry := func(network string) string {
		return `[[accept]]
rule = "flag-drift"
account = "` + accountOne + `"
network = "` + network + `"
reason = "known drift"
`
	}

	cfg, err := Load(strings.NewReader(body(entry("mainnet") + "\n" + entry("testnet"))))
	if err != nil {
		t.Fatalf("pinning both networks: %v", err)
	}
	if len(cfg.Accepts) != 2 {
		t.Fatalf("accepts = %d, want 2", len(cfg.Accepts))
	}

	_, err = Load(strings.NewReader(body(entry("mainnet") + "\n" + entry("mainnet"))))
	if !errors.Is(err, ErrDuplicateAccept) {
		t.Fatalf("the same rule, account and network twice: err = %v, want ErrDuplicateAccept", err)
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
