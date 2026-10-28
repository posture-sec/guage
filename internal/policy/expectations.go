package policy

type ExpectedSigner struct {
	Key    string `toml:"key"`
	Weight int    `toml:"weight"`
}

// ExpectedThresholds uses pointers to distinguish omission from an explicit zero.
type ExpectedThresholds struct {
	Low    *int `toml:"low"`
	Medium *int `toml:"medium"`
	High   *int `toml:"high"`
}

type ExpectedFlags struct {
	AuthRequired    *bool `toml:"auth_required"`
	AuthRevocable   *bool `toml:"auth_revocable"`
	AuthImmutable   *bool `toml:"auth_immutable"`
	ClawbackEnabled *bool `toml:"clawback"`
}
