package policy

type Acceptance struct {
	Rule    string  `toml:"rule"`
	Account string  `toml:"account"`
	Reason  string  `toml:"reason"`
	Expires *string `toml:"expires"`
}
