package model

type SignerType string

const (
	SignerEd25519   SignerType = "ed25519"
	SignerHashX     SignerType = "hash_x"
	SignerPreAuthTx SignerType = "pre_auth_tx"
)

type Signer struct {
	Key     string     `json:"key"`
	Type    SignerType `json:"type"`
	Weight  int        `json:"weight"`
	Sponsor string     `json:"sponsor"`
}
