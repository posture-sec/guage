package source

import (
	"reflect"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

const (
	compareAccount = "GBK4NFJ7SVMUNPBIYBC4WNMCTKI6Z66MYYWZQSSLQNOSUSTMKVUKE7M3"
	compareSigner  = "GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG"
	compareIssuer  = "GBQHXYDEJD6NEIUQ5NYDBVNTRCACJXMLO32JUQHBGO524NMVYOMIN5CP"
)

func compareBase() model.AccountSnapshot {
	return model.AccountSnapshot{
		ID:         compareAccount,
		Network:    "Test SDF Network ; September 2015",
		Endpoint:   "https://a.example",
		Ledger:     100,
		Sequence:   "4294967296",
		Thresholds: model.Thresholds{Low: 1, Medium: 2, High: 2},
		Signers: []model.Signer{
			{Key: compareAccount, Type: model.SignerEd25519, Weight: 1},
			{Key: compareSigner, Type: model.SignerEd25519, Weight: 1},
		},
		Flags:      model.Flags{AuthRequired: true},
		HomeDomain: "example.com",
		Balances: []model.Balance{
			{IsAuthorized: true},
			{AssetCode: "USD", AssetIssuer: compareIssuer, IsAuthorized: true},
		},
	}
}

func TestCompareNamesEveryFieldThatDiffers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		edit func(*model.AccountSnapshot)
		want []string
	}{
		{"identical", func(*model.AccountSnapshot) {}, nil},
		{"ledger", func(s *model.AccountSnapshot) { s.Ledger = 99 }, []string{"last_modified_ledger"}},
		{"sequence", func(s *model.AccountSnapshot) { s.Sequence = "4294967295" }, []string{"sequence"}},
		{"thresholds", func(s *model.AccountSnapshot) { s.Thresholds.High = 3 }, []string{"thresholds"}},
		{"flags", func(s *model.AccountSnapshot) { s.Flags.ClawbackEnabled = true }, []string{"flags"}},
		{"home domain", func(s *model.AccountSnapshot) { s.HomeDomain = "evil.example" }, []string{"home_domain"}},
		{"sponsor", func(s *model.AccountSnapshot) { s.Sponsor = compareIssuer }, []string{"sponsor"}},
		{"signer removed", func(s *model.AccountSnapshot) { s.Signers = s.Signers[:1] }, []string{"signers"}},
		{"signer weight", func(s *model.AccountSnapshot) {
			s.Signers = []model.Signer{s.Signers[0], {Key: compareSigner, Type: model.SignerEd25519, Weight: 2}}
		}, []string{"signers"}},
		{"signer sponsor", func(s *model.AccountSnapshot) {
			s.Signers = []model.Signer{s.Signers[0], {Key: compareSigner, Type: model.SignerEd25519, Weight: 1, Sponsor: compareIssuer}}
		}, []string{"signers"}},
		{"balance authorization", func(s *model.AccountSnapshot) {
			s.Balances = []model.Balance{s.Balances[0], {AssetCode: "USD", AssetIssuer: compareIssuer}}
		}, []string{"balances"}},
		{"balance removed", func(s *model.AccountSnapshot) { s.Balances = s.Balances[:1] }, []string{"balances"}},
		{"several, reported sorted", func(s *model.AccountSnapshot) {
			s.Thresholds.Low = 0
			s.Ledger = 1
			s.Signers = nil
			s.HomeDomain = ""
		}, []string{"home_domain", "last_modified_ledger", "signers", "thresholds"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			other := compareBase()
			other.Signers = append([]model.Signer(nil), other.Signers...)
			other.Balances = append([]model.Balance(nil), other.Balances...)
			tc.edit(&other)
			got := Compare(Reading{Snapshot: compareBase()}, Reading{Snapshot: other})
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Compare = %v, want %v", got, tc.want)
			}
			// The comparison is symmetric: which endpoint is listed first decides
			// which snapshot the rules read, not whether a disagreement exists.
			back := Compare(Reading{Snapshot: other}, Reading{Snapshot: compareBase()})
			if !reflect.DeepEqual(back, tc.want) {
				t.Fatalf("Compare reversed = %v, want %v", back, tc.want)
			}
		})
	}
}

// The endpoint URL and the network passphrase are the run's own inputs, not
// something either endpoint observed about the account, and the order Horizon lists
// signers and balances in carries no meaning, so none of them is a disagreement.
func TestCompareIgnoresOrderAndRunMetadata(t *testing.T) {
	t.Parallel()
	other := compareBase()
	other.Endpoint = "https://b.example"
	other.Network = "something the run will overwrite"
	other.Signers = []model.Signer{other.Signers[1], other.Signers[0]}
	other.Balances = []model.Balance{other.Balances[1], other.Balances[0]}
	if got := Compare(Reading{Snapshot: compareBase()}, Reading{Snapshot: other}); got != nil {
		t.Fatalf("Compare = %v, want no disagreement", got)
	}
}

func TestCompareTreatsExistenceAsAField(t *testing.T) {
	t.Parallel()
	missing := Reading{Missing: true, Snapshot: model.AccountSnapshot{ID: compareAccount}}
	present := Reading{Snapshot: compareBase()}

	if got := Compare(missing, missing); got != nil {
		t.Fatalf("both missing = %v, want agreement", got)
	}
	for _, pair := range [][2]Reading{{missing, present}, {present, missing}} {
		got := Compare(pair[0], pair[1])
		if !reflect.DeepEqual(got, []string{"existence"}) {
			t.Fatalf("one missing = %v, want [existence] and nothing about the fields one side never read", got)
		}
	}
}

// Evidence feeds the finding fingerprint and reaches output, so it carries field
// names only: a value would make the fingerprint move with every ledger and would
// put a chain-derived string, such as a hostile home domain, into evidence.
func TestCompareNeverCarriesValues(t *testing.T) {
	t.Parallel()
	other := compareBase()
	other.HomeDomain = "attacker\u202e.example"
	other.Sequence = "77777"
	other.Ledger = 424242
	for _, field := range Compare(Reading{Snapshot: compareBase()}, Reading{Snapshot: other}) {
		for _, value := range []string{"attacker", "77777", "424242", "example.com"} {
			if strings.Contains(field, value) {
				t.Fatalf("field %q carries an observed value", field)
			}
		}
	}
}
