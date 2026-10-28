package model

type NetworkInfo struct {
	Passphrase string `json:"passphrase"`
	Ledger     int64  `json:"ledger"`
}
