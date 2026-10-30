package policy

import (
	"encoding/base32"
	"errors"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

const (
	// Public SEP-23 test vectors, not secrets.
	accountOne = "GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ"
	accountTwo = "GBK4NFJ7SVMUNPBIYBC4WNMCTKI6Z66MYYWZQSSLQNOSUSTMKVUKE7M3"
)

func validPolicy() string {
	return `
version = 1
network = "testnet"
endpoints = ["https://horizon-testnet.stellar.org"]

[[account]]
id = "` + accountOne + `"
name = "treasury"
role = "treasury"
expected_signers = [ { key = "` + accountOne + `", weight = 1 }, { key = "` + accountTwo + `", weight = 1 } ]
expected_thresholds = { low = 1, medium = 2, high = 2 }
expected_flags = { auth_required = false, auth_revocable = false, auth_immutable = false, clawback = false }
home_domain = "example.com"
expect_locked = false

[[accept]]
rule = "flag-undeclared-clawback"
account = "` + accountTwo + `"
reason = "Regulated asset; clawback disclosed in terms"
expires = "2027-04-27"
`
}

func TestLoadValidPolicy(t *testing.T) {
	t.Parallel()
	cfg, err := Load(strings.NewReader(validPolicy()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Version != 1 || cfg.Network != "testnet" {
		t.Fatalf("header = %d %q", cfg.Version, cfg.Network)
	}
	if len(cfg.Endpoints) != 1 || cfg.Endpoints[0] != "https://horizon-testnet.stellar.org" {
		t.Fatalf("endpoints = %v", cfg.Endpoints)
	}
	if len(cfg.Accounts) != 1 {
		t.Fatalf("accounts = %d", len(cfg.Accounts))
	}
	a := cfg.Accounts[0]
	if a.ID != accountOne || a.Name != "treasury" || a.Role != model.RoleTreasury {
		t.Fatalf("account = %+v", a)
	}
	if a.ExpectedSigners == nil || len(*a.ExpectedSigners) != 2 {
		t.Fatalf("expected signers = %v", a.ExpectedSigners)
	}
	if a.ExpectedThresholds == nil || *a.ExpectedThresholds.Low != 1 || *a.ExpectedThresholds.High != 2 {
		t.Fatalf("expected thresholds = %+v", a.ExpectedThresholds)
	}
	if a.HomeDomain == nil || *a.HomeDomain != "example.com" {
		t.Fatalf("home domain = %v", a.HomeDomain)
	}
	if a.ExpectLocked == nil || *a.ExpectLocked {
		t.Fatalf("expect locked = %v", a.ExpectLocked)
	}
	if len(cfg.Accepts) != 1 || cfg.Accepts[0].Expires == nil || *cfg.Accepts[0].Expires != "2027-04-27" {
		t.Fatalf("accepts = %+v", cfg.Accepts)
	}
}

func TestLoadDefaultsRoleAndKeepsOmittedSeparate(t *testing.T) {
	t.Parallel()
	cfg, err := Load(strings.NewReader(`
version = 1
network = "mainnet"

[[account]]
id = "` + accountOne + `"

[[account]]
id = "` + accountTwo + `"
expected_thresholds = { low = 0 }
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Accounts[0].Role != model.RoleOther {
		t.Fatalf("default role = %q", cfg.Accounts[0].Role)
	}
	if cfg.Accounts[0].ExpectedThresholds != nil {
		t.Fatal("omitted expectations became non-nil")
	}
	th := cfg.Accounts[1].ExpectedThresholds
	if th == nil || th.Low == nil || *th.Low != 0 {
		t.Fatal("explicit zero threshold was not preserved")
	}
	if th.Medium != nil || th.High != nil {
		t.Fatal("omitted threshold fields became non-nil")
	}
}

func TestExpectedPassphrase(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		network string
		want    string
	}{
		{"testnet", "Test SDF Network ; September 2015"},
		{"mainnet", "Public Global Stellar Network ; September 2015"},
		{"Standalone ; January 2024", "Standalone ; January 2024"},
	} {
		t.Run(tt.network, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{Version: 1, Network: tt.network}
			if got := cfg.ExpectedPassphrase(); got != tt.want {
				t.Fatalf("passphrase = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadRejects(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		toml string
		want error
	}{
		{"missing version", `network = "testnet"`, ErrVersion},
		{"wrong version", "version = 2\nnetwork = \"testnet\"", ErrVersion},
		{"missing network", `version = 1`, ErrNetwork},
		{"unknown network name", "version = 1\nnetwork = \"futurenet\"", ErrNetwork},
		{"unknown top-level key", "version = 1\nnetwork = \"testnet\"\naccounts = []", ErrUnknownKey},
		{"unknown account key", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + accountOne + "\"\nowners = []", ErrUnknownKey},
		{"unknown threshold key", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + accountOne + "\"\nexpected_thresholds = { extreme = 1 }", ErrUnknownKey},
		{"duplicate account", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + accountOne + "\"\n[[account]]\nid = \"" + accountOne + "\"", ErrDuplicateAccount},
		{"multiplexed id", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUQ\"", ErrNotAccount},
		{"contract id", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"CA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUWDA\"", ErrNotAccount},
		{"bad checksum id", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + accountOne[:len(accountOne)-1] + "A\"", ErrNotAccount},
		{"unknown role", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + accountOne + "\"\nrole = \"admin\"", ErrRole},
		{"signer not an address", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + accountOne + "\"\nexpected_signers = [ { key = \"not-an-address\", weight = 1 } ]", ErrSignerKey},
		{"signer weight too high", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + accountOne + "\"\nexpected_signers = [ { key = \"" + accountTwo + "\", weight = 256 } ]", ErrWeight},
		{"negative signer weight", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + accountOne + "\"\nexpected_signers = [ { key = \"" + accountTwo + "\", weight = -1 } ]", ErrWeight},
		{"duplicate expected signer", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + accountOne + "\"\nexpected_signers = [ { key = \"" + accountTwo + "\", weight = 1 }, { key = \"" + accountTwo + "\", weight = 2 } ]", ErrDuplicateSigner},
		{"threshold too high", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + accountOne + "\"\nexpected_thresholds = { high = 256 }", ErrThreshold},
		{"negative threshold", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + accountOne + "\"\nexpected_thresholds = { low = -1 }", ErrThreshold},
		{"http endpoint", "version = 1\nnetwork = \"testnet\"\nendpoints = [\"http://horizon-testnet.stellar.org\"]", ErrEndpoint},
		{"endpoint without host", "version = 1\nnetwork = \"testnet\"\nendpoints = [\"https:///path\"]", ErrEndpoint},
		{"home domain with scheme", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + accountOne + "\"\nhome_domain = \"https://example.com\"", ErrHomeDomain},
		{"home domain is an IP", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + accountOne + "\"\nhome_domain = \"192.0.2.1\"", ErrHomeDomain},
		{"accept missing reason", "version = 1\nnetwork = \"testnet\"\n[[accept]]\nrule = \"flag-drift\"\naccount = \"" + accountOne + "\"", ErrAccept},
		{"accept wildcard rule", "version = 1\nnetwork = \"testnet\"\n[[accept]]\nrule = \"*\"\naccount = \"" + accountOne + "\"\nreason = \"r\"", ErrAcceptRule},
		{"accept wildcard account", "version = 1\nnetwork = \"testnet\"\n[[accept]]\nrule = \"flag-drift\"\naccount = \"*\"\nreason = \"r\"", ErrAccept},
		{"accept non-acceptable rule", "version = 1\nnetwork = \"testnet\"\n[[accept]]\nrule = \"acct-missing\"\naccount = \"" + accountOne + "\"\nreason = \"r\"", ErrAcceptRule},
		{"accept bad expiry", "version = 1\nnetwork = \"testnet\"\n[[accept]]\nrule = \"flag-drift\"\naccount = \"" + accountOne + "\"\nreason = \"r\"\nexpires = \"2027-13-01\"", ErrAccept},
		{"duplicate accept", "version = 1\nnetwork = \"testnet\"\n[[accept]]\nrule = \"flag-drift\"\naccount = \"" + accountOne + "\"\nreason = \"r\"\n[[accept]]\nrule = \"flag-drift\"\naccount = \"" + accountOne + "\"\nreason = \"s\"", ErrDuplicateAccept},
		{"syntax error", "version =", ErrSyntax},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(strings.NewReader(tt.toml))
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestLoadRejectsSecretSeed(t *testing.T) {
	t.Parallel()
	seed := testSeed()
	for _, tt := range []struct {
		name string
		toml string
	}{
		{"as an id", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + seed + "\""},
		{"in a name", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + accountOne + "\"\nname = \"" + seed + "\""},
		{"in a comment", "version = 1\nnetwork = \"testnet\"\n# " + seed + "\n[[account]]\nid = \"" + accountOne + "\""},
		{"lowercase", "version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + strings.ToLower(seed) + "\""},
		{"in an accept reason", "version = 1\nnetwork = \"testnet\"\n[[accept]]\nrule = \"flag-drift\"\naccount = \"" + accountOne + "\"\nreason = \"" + seed + "\""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(strings.NewReader(tt.toml))
			if !errors.Is(err, ErrSecretSeed) {
				t.Fatalf("error = %v, want %v", err, ErrSecretSeed)
			}
			if err != nil && (strings.Contains(err.Error(), seed) || strings.Contains(err.Error(), strings.ToLower(seed))) {
				t.Fatal("error message echoed the seed")
			}
		})
	}
}

func TestLoadRejectsOversizedInput(t *testing.T) {
	t.Parallel()
	big := "version = 1\nnetwork = \"testnet\"\n# " + strings.Repeat("x", MaxConfigBytes) + "\n"
	if _, err := Load(strings.NewReader(big)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("error = %v, want %v", err, ErrTooLarge)
	}
}

// testSeed builds a valid S strkey at run time. Its payload is all zeroes and
// it exists only inside this test, so no seed-shaped value is committed.
func testSeed() string {
	body := make([]byte, 32)
	raw := append([]byte{18 << 3}, body...)
	sum := crc16(raw)
	raw = append(raw, byte(sum), byte(sum>>8))
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
}

func crc16(data []byte) uint16 {
	var crc uint16
	for _, c := range data {
		crc ^= uint16(c) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func TestExamplePolicyLoads(t *testing.T) {
	t.Parallel()
	cfg, err := LoadFile("../../gauge.example.toml")
	if err != nil {
		t.Fatalf("gauge.example.toml: %v", err)
	}
	if len(cfg.Accounts) != 1 || cfg.Network != "testnet" {
		t.Fatalf("example policy = %+v", cfg)
	}
}
