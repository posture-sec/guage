package rules

import (
	"reflect"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/sep1"
)

// Context is passed to every rule by value, which copies the struct and not the
// arrays behind its slices: Snapshot.Signers, Snapshot.Balances, the declared
// signer list and the identity document's accounts all alias the caller's data.
// internal/run reuses the snapshot after Evaluate, so a rule that sorts or
// filters in place would corrupt every later rule and the report. Evaluate does
// not copy defensively on purpose; the contract is that rules do not mutate, and
// these tests are what hold them to it.

// purityContext builds a context that makes most of the catalogue fire, with
// every slice in an order that is neither sorted by key nor by weight in either
// direction, so an in-place sort anywhere moves at least one element.
func purityContext() Context {
	domain := "example.org"
	return Context{
		Account: policy.Account{
			ID:   testAccount,
			Name: "purity",
			Role: model.RoleIssuer,
			ExpectedSigners: expected(
				policy.ExpectedSigner{Key: "GZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ", Weight: 2},
				policy.ExpectedSigner{Key: secondAccount, Weight: 1},
				policy.ExpectedSigner{Key: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", Weight: 4},
			),
			ExpectedThresholds: &policy.ExpectedThresholds{Low: intPointer(9)},
			ExpectedFlags:      &policy.ExpectedFlags{AuthRequired: boolean(true)},
			HomeDomain:         &domain,
		},
		Snapshot: model.AccountSnapshot{
			ID:         testAccount,
			Network:    testnetPassphrase,
			Endpoint:   "https://horizon-testnet.stellar.org",
			Ledger:     123,
			Sequence:   "456",
			Thresholds: model.Thresholds{Low: 3, Medium: 2, High: 30},
			Signers: []model.Signer{
				{Key: testAccount, Type: model.SignerEd25519, Weight: 1},
				{Key: secondAccount, Type: model.SignerEd25519, Weight: 3},
				{Key: "XHASHXREFERENCEHASHXREFERENCEHASHXREFERENCEHASHXREFEREN", Type: model.SignerHashX, Weight: 2},
				{Key: "TPREAUTHREFERENCEPREAUTHREFERENCEPREAUTHREFERENCEPREAUT", Type: model.SignerPreAuthTx, Weight: 1},
				{Key: "GCZEROWEIGHTZEROWEIGHTZEROWEIGHTZEROWEIGHTZEROWEIGHTZER", Type: model.SignerEd25519, Weight: 0},
			},
			Flags:      model.Flags{ClawbackEnabled: true, AuthRevocable: true},
			HomeDomain: domain,
			Balances: []model.Balance{
				{AssetCode: "ZED", AssetIssuer: secondAccount, IsAuthorized: true},
				{AssetCode: "ABC", AssetIssuer: testAccount, ClawbackEnabled: true},
				{AssetCode: "MID", AssetIssuer: secondAccount},
			},
		},
		Identity: &sep1.Result{
			HomeDomain: domain,
			FailedAt:   sep1.StepOK,
			Identity: sep1.Identity{
				Accounts: []string{
					"GZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ",
					"GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
					"GMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM",
				},
			},
		},
		Network:  testnetPassphrase,
		Expected: testnetPassphrase,
	}
}

func intPointer(v int) *int { return &v }

// deepCopyContext copies every slice, pointer and pointed-to slice in a Context,
// so the copy shares no backing array with the original.
func deepCopyContext(c Context) Context {
	out := c
	if c.Snapshot.Signers != nil {
		out.Snapshot.Signers = append([]model.Signer(nil), c.Snapshot.Signers...)
	}
	if c.Snapshot.Balances != nil {
		out.Snapshot.Balances = append([]model.Balance(nil), c.Snapshot.Balances...)
	}
	if c.Account.ExpectedSigners != nil {
		list := append([]policy.ExpectedSigner(nil), (*c.Account.ExpectedSigners)...)
		out.Account.ExpectedSigners = &list
	}
	if c.Account.ExpectedThresholds != nil {
		thresholds := *c.Account.ExpectedThresholds
		out.Account.ExpectedThresholds = &thresholds
	}
	if c.Account.ExpectedFlags != nil {
		flags := *c.Account.ExpectedFlags
		out.Account.ExpectedFlags = &flags
	}
	if c.Account.HomeDomain != nil {
		domain := *c.Account.HomeDomain
		out.Account.HomeDomain = &domain
	}
	if c.Identity != nil {
		identity := *c.Identity
		if c.Identity.Identity.Accounts != nil {
			identity.Identity.Accounts = append([]string(nil), c.Identity.Identity.Accounts...)
		}
		out.Identity = &identity
	}
	return out
}

func TestEvaluationDoesNotMutate(t *testing.T) {
	t.Parallel()
	original := purityContext()
	reference := deepCopyContext(original)

	// The copy must not share an array with the original, or the equality below
	// would hold even after a mutation.
	if &reference.Snapshot.Signers[0] == &original.Snapshot.Signers[0] ||
		&reference.Snapshot.Balances[0] == &original.Snapshot.Balances[0] ||
		&(*reference.Account.ExpectedSigners)[0] == &(*original.Account.ExpectedSigners)[0] ||
		&reference.Identity.Identity.Accounts[0] == &original.Identity.Identity.Accounts[0] {
		t.Fatal("deepCopyContext shares a backing array with its input")
	}

	first := Evaluate(original)
	if len(first) < 8 {
		// The context is built to make most of the catalogue fire; if it stops
		// doing so the test is no longer exercising the rules it claims to.
		t.Fatalf("purity context raised %d findings, want at least 8: %+v", len(first), first)
	}

	assertContextUnchanged(t, "after the first Evaluate", original, reference)

	second := Evaluate(original)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("a second Evaluate returned different findings\nfirst:  %+v\nsecond: %+v", first, second)
	}
	assertContextUnchanged(t, "after the second Evaluate", original, reference)

	// Facts reads the same Context the rules do and is held to the same promise.
	_ = Facts(original)
	assertContextUnchanged(t, "after Facts", original, reference)
}

// TestEachRuleLeavesTheContextAlone runs every registered rule on its own
// context, so a failure names the rule that mutated rather than only Evaluate.
func TestEachRuleLeavesTheContextAlone(t *testing.T) {
	t.Parallel()
	for _, rule := range All() {
		t.Run(rule.ID, func(t *testing.T) {
			t.Parallel()
			original := purityContext()
			reference := deepCopyContext(original)
			rule.Check(original)
			assertContextUnchanged(t, "after "+rule.ID, original, reference)
		})
	}
}

func assertContextUnchanged(t *testing.T, when string, got, want Context) {
	t.Helper()
	if len(got.Snapshot.Signers) != len(want.Snapshot.Signers) {
		t.Fatalf("%s: snapshot signers length %d, want %d", when, len(got.Snapshot.Signers), len(want.Snapshot.Signers))
	}
	for i := range want.Snapshot.Signers {
		if got.Snapshot.Signers[i] != want.Snapshot.Signers[i] {
			t.Errorf("%s: snapshot signer %d is %+v, want %+v", when, i, got.Snapshot.Signers[i], want.Snapshot.Signers[i])
		}
	}
	if len(got.Snapshot.Balances) != len(want.Snapshot.Balances) {
		t.Fatalf("%s: snapshot balances length %d, want %d", when, len(got.Snapshot.Balances), len(want.Snapshot.Balances))
	}
	for i := range want.Snapshot.Balances {
		if got.Snapshot.Balances[i] != want.Snapshot.Balances[i] {
			t.Errorf("%s: snapshot balance %d is %+v, want %+v", when, i, got.Snapshot.Balances[i], want.Snapshot.Balances[i])
		}
	}
	if !reflect.DeepEqual(got.Snapshot, want.Snapshot) {
		// The per-element checks above name a reordered or rewritten slice entry;
		// this catches every other field without dumping the whole snapshot.
		t.Errorf("%s: snapshot is not deeply equal to the copy taken before evaluation", when)
	}
	if !reflect.DeepEqual(got.Account, want.Account) {
		t.Errorf("%s: declared account changed\ngot:  %+v\nwant: %+v", when, got.Account, want.Account)
	}
	if !reflect.DeepEqual(got.Identity, want.Identity) {
		t.Errorf("%s: identity result changed\ngot:  %+v\nwant: %+v", when, got.Identity, want.Identity)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: context changed", when)
	}
}
