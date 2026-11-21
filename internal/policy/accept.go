package policy

type Acceptance struct {
	Rule    string  `toml:"rule"`
	Account string  `toml:"account"`
	Network string  `toml:"network"`
	Reason  string  `toml:"reason"`
	Expires *string `toml:"expires"`
}

// ExpectedPassphrase returns the passphrase a network value stands for. The two
// named networks map to their published passphrases and any other value is the
// passphrase itself, which is the same mapping a policy's top-level network gets.
func ExpectedPassphrase(network string) string {
	switch network {
	case "mainnet":
		return "Public Global Stellar Network ; September 2015"
	case "testnet":
		return "Test SDF Network ; September 2015"
	default:
		return network
	}
}
