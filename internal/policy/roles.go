package policy

// ExpectedPassphrase returns the network passphrase a run must see from its
// endpoint. The two named networks map to their published passphrases; any
// other value is the passphrase itself.
func (c *Config) ExpectedPassphrase() string {
	switch c.Network {
	case "mainnet":
		return "Public Global Stellar Network ; September 2015"
	case "testnet":
		return "Test SDF Network ; September 2015"
	default:
		return c.Network
	}
}
