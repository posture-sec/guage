package model

type Role string

const (
	RoleIssuer       Role = "issuer"
	RoleTreasury     Role = "treasury"
	RoleDistribution Role = "distribution"
	RoleMerchant     Role = "merchant"
	RoleOther        Role = "other"
)
