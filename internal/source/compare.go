package source

import (
	"sort"

	"github.com/posture-sec/guage/internal/model"
)

// Reading is what one endpoint said about one account: a snapshot, or that it
// does not serve the account at all.
type Reading struct {
	Snapshot model.AccountSnapshot
	Missing  bool
}

// Compare returns the sorted names of the observed fields on which two
// endpoints' readings of the same account differ, or nil when they agree. It is
// pure and symmetric.
//
// The names are fixed strings and never values. They feed a finding's evidence,
// and so its fingerprint, which must not move with every ledger, and a value
// would carry a chain-derived string into evidence. The endpoint URL and the
// network passphrase are the run's own inputs rather than observations, and the
// order an endpoint lists signers or balances in carries no meaning, so none of
// those is compared.
//
// A disagreement is reported, never resolved: Compare does not say which side
// is right, because a stale endpoint and a filtering one look the same here.
func Compare(a, b Reading) []string {
	if a.Missing || b.Missing {
		if a.Missing == b.Missing {
			return nil
		}
		return []string{"existence"}
	}
	x, y := a.Snapshot, b.Snapshot
	var fields []string
	if x.Ledger != y.Ledger {
		fields = append(fields, "last_modified_ledger")
	}
	if x.Sequence != y.Sequence {
		fields = append(fields, "sequence")
	}
	if !x.Thresholds.Equal(y.Thresholds) {
		fields = append(fields, "thresholds")
	}
	if !x.Flags.Equal(y.Flags) {
		fields = append(fields, "flags")
	}
	if !sameMultiset(x.Signers, y.Signers) {
		fields = append(fields, "signers")
	}
	if x.HomeDomain != y.HomeDomain {
		fields = append(fields, "home_domain")
	}
	if x.Sponsor != y.Sponsor {
		fields = append(fields, "sponsor")
	}
	if !sameMultiset(x.Balances, y.Balances) {
		fields = append(fields, "balances")
	}
	sort.Strings(fields)
	return fields
}

// sameMultiset compares two lists ignoring order. Signers and balances both go
// through it: Horizon's listing order means nothing, and nothing in a decoded
// balance makes an entry unique.
func sameMultiset[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[T]int, len(a))
	for _, v := range a {
		seen[v]++
	}
	for _, v := range b {
		if seen[v] == 0 {
			return false
		}
		seen[v]--
	}
	return true
}
