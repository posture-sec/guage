package model

import "strings"

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

// HasHomeDomain reports whether the account publishes a home domain. Horizon returns
// the field exactly as the account set it, and one captured mainnet account carries a
// single space there, so a value that is blank after trimming is an absent domain
// rather than a host to contact. Every question of the form "does this account have a
// home domain" asks it this way, including whether to fetch and what the identity
// rules may speak about.
func (a AccountSnapshot) HasHomeDomain() bool {
	return strings.TrimSpace(a.HomeDomain) != ""
}
