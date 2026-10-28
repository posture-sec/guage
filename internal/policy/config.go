package policy

import "github.com/posture-sec/guage/internal/model"

type Config struct {
	Version   int          `toml:"version"`
	Network   string       `toml:"network"`
	Endpoints []string     `toml:"endpoints"`
	Accounts  []Account    `toml:"account"`
	Accepts   []Acceptance `toml:"accept"`
}

type Account struct {
	ID                 string              `toml:"id"`
	Name               string              `toml:"name"`
	Role               model.Role          `toml:"role"`
	ExpectedSigners    *[]ExpectedSigner   `toml:"expected_signers"`
	ExpectedThresholds *ExpectedThresholds `toml:"expected_thresholds"`
	ExpectedFlags      *ExpectedFlags      `toml:"expected_flags"`
	HomeDomain         *string             `toml:"home_domain"`
	ExpectLocked       *bool               `toml:"expect_locked"`
}
