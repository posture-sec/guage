package model

import "strings"

type Thresholds struct {
	Low    int `json:"low"`
	Medium int `json:"medium"`
	High   int `json:"high"`
}

// Valid reports whether every tier is inside 0 to 255, the range the schema fixes for
// a threshold. The policy loader range-checks declared thresholds; this is the same
// question for observed ones, so a decode or fixture holding -1 or 300 can be told
// apart from an account that really has that value. It checks each tier alone and says
// nothing about their order, which the sig-threshold-order rule owns.
func (t Thresholds) Valid() bool {
	return validThreshold(t.Low) && validThreshold(t.Medium) && validThreshold(t.High)
}

// Equal reports whether every tier matches. Unlike the compare closures in the drift
// rules it carries no message, because it answers "do these two observations agree"
// rather than "which field disagrees".
func (t Thresholds) Equal(other Thresholds) bool {
	return t == other
}

func validThreshold(v int) bool {
	return v >= 0 && v <= 255
}

type Flags struct {
	AuthRequired    bool `json:"auth_required"`
	AuthRevocable   bool `json:"auth_revocable"`
	AuthImmutable   bool `json:"auth_immutable"`
	ClawbackEnabled bool `json:"clawback_enabled"`
}

// Valid reports whether the flags can occur together. Every bool is in range and no
// exclusion between them is documented, so it is true for all sixteen combinations;
// a test pins that. The method exists so callers can treat both observation types the
// same way, and so a documented exclusion has one place to land.
func (f Flags) Valid() bool {
	return true
}

// Equal reports whether every flag matches.
func (f Flags) Equal(other Flags) bool {
	return f == other
}

type AccountSnapshot struct {
	ID         string            `json:"id"`
	Network    string            `json:"network"`
	Endpoint   string            `json:"endpoint"`
	Ledger     int64             `json:"ledger"`
	Sequence   string            `json:"sequence"`
	Thresholds Thresholds        `json:"thresholds"`
	Signers    []Signer          `json:"signers"`
	Flags      Flags             `json:"flags"`
	HomeDomain string            `json:"home_domain"`
	Sponsor    string            `json:"sponsor"`
	Balances   []Balance         `json:"balances"`
	Data       map[string]string `json:"data,omitempty"`
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
