package model

type Balance struct {
	AssetCode       string `json:"asset_code"`
	AssetIssuer     string `json:"asset_issuer"`
	IsAuthorized    bool   `json:"is_authorized"`
	ClawbackEnabled bool   `json:"clawback_enabled"`
}
