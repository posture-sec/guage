package model

type Thresholds struct {
	Low    int `json:"low"`
	Medium int `json:"medium"`
	High   int `json:"high"`
}

type Flags struct {
	AuthRequired    bool `json:"auth_required"`
	AuthRevocable   bool `json:"auth_revocable"`
	AuthImmutable   bool `json:"auth_immutable"`
	ClawbackEnabled bool `json:"clawback_enabled"`
}

type AccountSnapshot struct {
	ID         string     `json:"id"`
	Network    string     `json:"network"`
	Endpoint   string     `json:"endpoint"`
	Ledger     int64      `json:"ledger"`
	Sequence   string     `json:"sequence"`
	Thresholds Thresholds `json:"thresholds"`
	Signers    []Signer   `json:"signers"`
	Flags      Flags      `json:"flags"`
	HomeDomain string     `json:"home_domain"`
	Sponsor    string     `json:"sponsor"`
	Balances   []Balance  `json:"balances"`
}
