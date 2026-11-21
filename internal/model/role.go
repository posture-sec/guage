package model

type Role string

const (
	RoleIssuer       Role = "issuer"
	RoleTreasury     Role = "treasury"
	RoleDistribution Role = "distribution"
	RoleMerchant     Role = "merchant"
	RoleOther        Role = "other"
)

// IsValid reports whether the role is one of the five a policy may declare.
// The type alone does not close the set: a report built by hand can still hold
// any string, so a consumer that reads a role as an operator's declaration must
// ask.
func (r Role) IsValid() bool {
	switch r {
	case RoleIssuer, RoleTreasury, RoleDistribution, RoleMerchant, RoleOther:
		return true
	}
	return false
}
