package rules

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/sep1"
)

// Task 3.7: the facts a report prints before its findings. These are the unit half
// of the promise the goldens show at report scale, and the shape that matters here is
// that nothing in an AccountFacts carries a rule ID, a severity or a fingerprint. A
// fact cannot be accepted, escalated by a role or counted at exit.

const requestedNetwork = "Test SDF Network ; September 2015"

// The pointer helpers text, boolean and number belong to this package's tests
// already; this file uses them rather than declaring its own.

func TestFactsNameEveryTierTheAccountStores(t *testing.T) {
	t.Parallel()
	facts := Facts(Context{
		Snapshot: snapshotWith(model.Thresholds{Low: 1, Medium: 5, High: 10},
			signerEntry(testAccount, model.SignerEd25519, 3),
			signerEntry(secondAccount, model.SignerEd25519, 4)),
	})
	reachability := facts.Reachability
	if reachability == nil {
		t.Fatal("a readable account has no reachability facts")
	}
	// Weights 4 and 3 against tiers 1, 5 and 10: the larger key covers the low and
	// medium tiers alone, and the two together are 7, so the high tier is dark.
	compareTier(t, "low", reachability.Low, number(1))
	compareTier(t, "medium", reachability.Medium, number(2))
	compareTier(t, "high", reachability.High, nil)
}

func compareTier(t *testing.T, name string, got, want *int) {
	t.Helper()
	switch {
	case want == nil && got != nil:
		t.Errorf("%s tier reports %d signers, want unreachable", name, *got)
	case want != nil && got == nil:
		t.Errorf("%s tier reports unreachable, want %d signers", name, *want)
	case want != nil && got != nil && *got != *want:
		t.Errorf("%s tier = %d signers, want %d", name, *got, *want)
	}
}

// TestFactsHaveNoAnswerForAnUnreadAccount is the facts equivalent of the rule
// guards: an account the endpoint did not serve, and an endpoint on the wrong
// network, have no signers to count, so the arithmetic says nothing rather than
// reporting three unreachable tiers it never looked at.
func TestFactsHaveNoAnswerForAnUnreadAccount(t *testing.T) {
	t.Parallel()
	readable := snapshotWith(model.Thresholds{}, signerEntry(testAccount, model.SignerEd25519, 1))
	for _, tt := range []struct {
		name string
		cx   Context
	}{
		{"missing", Context{Missing: true, Snapshot: readable}},
		{"mismatch", Context{Mismatch: true, Snapshot: readable}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if facts := Facts(tt.cx); facts.Reachability != nil {
				t.Fatalf("%s account carries reachability facts: %+v", tt.name, *facts.Reachability)
			}
		})
	}
}

// TestFactsCountADeclarationSeparatelyFromOmission keeps the distinction the loader
// preserves: an empty list is something an operator wrote, and an omitted key is not.
// The three signer rules stay silent for both, so only this block tells an operator
// which half of the file Gauge read.
func TestFactsCountADeclarationSeparatelyFromOmission(t *testing.T) {
	t.Parallel()
	empty := []policy.ExpectedSigner{}
	one := []policy.ExpectedSigner{{Key: testAccount, Weight: 1}}
	for _, tt := range []struct {
		name string
		give *[]policy.ExpectedSigner
		want *int
	}{
		{"omitted", nil, nil},
		{"declared empty", &empty, number(0)},
		{"one key", &one, number(1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			compareTier(t, "expected_signers",
				Facts(Context{Account: policy.Account{ExpectedSigners: tt.give}}).Declared.ExpectedSigners, tt.want)
		})
	}
}

func TestFactsKeepTheThresholdsADeclarationLeftOut(t *testing.T) {
	t.Parallel()
	expected := Facts(Context{Account: policy.Account{
		ExpectedThresholds: &policy.ExpectedThresholds{Low: number(0)},
	}}).Declared.ExpectedThresholds
	if expected == nil {
		t.Fatal("a declared thresholds block came back as undeclared")
	}
	compareTier(t, "low", expected.Low, number(0))
	compareTier(t, "medium", expected.Medium, nil)
	compareTier(t, "high", expected.High, nil)
}

// TestFactsMarkALockTheRoleMayNotDeclare is the report half of the gate
// internal/policy documents: a treasury's expect_locked is loaded and not refused, and
// changes nothing. The finding stays HIGH; this is where an operator finds out the line
// in the file did not apply.
func TestFactsMarkALockTheRoleMayNotDeclare(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		account policy.Account
		want    bool
	}{
		{"treasury declaring a lock", policy.Account{Role: model.RoleTreasury, ExpectLocked: boolean(true)}, true},
		{"issuer declaring a lock", policy.Account{Role: model.RoleIssuer, ExpectLocked: boolean(true)}, false},
		{"treasury declaring no lock", policy.Account{Role: model.RoleTreasury, ExpectLocked: boolean(false)}, false},
		{"no declaration at all", policy.Account{Role: model.RoleTreasury}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			declared := Facts(Context{Account: tt.account}).Declared
			if declared.ExpectLockedIgnored != tt.want {
				t.Errorf("expect_locked ignored = %t, want %t (%+v)", declared.ExpectLockedIgnored, tt.want, declared)
			}
			if declared.Role != string(tt.account.Role) {
				t.Errorf("declared role = %q, want %q", declared.Role, tt.account.Role)
			}
		})
	}
}

// TestFactsSayNothingAboutADocumentThatNeverParsed is the boundary the report cannot
// cross: an unread body is not a document that named no keys. The parser kept nothing,
// so the four identity keys print as nothing rather than as absences the issuer chose.
func TestFactsSayNothingAboutADocumentThatNeverParsed(t *testing.T) {
	t.Parallel()
	result := &sep1.Result{
		HomeDomain: "example.com",
		FailedAt:   sep1.StepParse,
		Detail:     "the document is not a configuration file this parser accepts",
		Status:     200,
		Bytes:      512,
		Identity: sep1.Identity{
			NetworkPassphrase: text(requestedNetwork),
			SigningKey:        text(testAccount),
			Accounts:          []string{secondAccount},
		},
	}
	identity := Facts(Context{Identity: result, Expected: requestedNetwork}).Identity
	if identity == nil {
		t.Fatal("a fetched document printed no identity facts")
	}
	if identity.Reachable {
		t.Error("a document that failed to parse reports itself readable")
	}
	if identity.Step != "parse" || identity.Detail == "" {
		t.Errorf("step %q detail %q, want the deciding step and its phrase", identity.Step, identity.Detail)
	}
	if identity.Status != 200 || identity.Bytes != 512 {
		t.Errorf("fetch facts lost: %+v", identity)
	}
	if identity.NetworkPassphrase != nil || identity.SigningKey != nil ||
		identity.HorizonURL != nil || identity.Version != nil || identity.NetworkMatches != nil {
		t.Errorf("an unparsed document printed parsed values: %+v", identity)
	}
	if len(identity.Accounts) != 0 {
		t.Errorf("an unparsed document listed %d accounts", len(identity.Accounts))
	}
}

func TestFactsOfADocumentNeverFetchedAreAbsent(t *testing.T) {
	t.Parallel()
	if identity := Facts(Context{Expected: requestedNetwork}).Identity; identity != nil {
		t.Fatalf("a run that fetched nothing printed %+v", identity)
	}
}

// TestFactsCompareTheDocumentPassphraseToTheRequestedNetwork is where the value
// internal/sep1 has carried since 3.5 "so task 3.7 can print it" finally lands. No
// rule covers a disagreement, so the comparison is a fact and the report is the only
// place an operator reads it.
func TestFactsCompareTheDocumentPassphraseToTheRequestedNetwork(t *testing.T) {
	t.Parallel()
	const otherNetwork = "Public Global Stellar Network ; September 2015"
	for _, tt := range []struct {
		name    string
		stated  *string
		matches *bool
	}{
		{"the document agrees", text(requestedNetwork), boolean(true)},
		{"the document names another network", text(otherNetwork), boolean(false)},
		{"the document names none", nil, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := &sep1.Result{
				HomeDomain: "example.com",
				FailedAt:   sep1.StepOK,
				Status:     200,
				Bytes:      64,
				Identity:   sep1.Identity{NetworkPassphrase: tt.stated},
			}
			identity := Facts(Context{Identity: result, Expected: requestedNetwork}).Identity
			if !identity.Reachable {
				t.Fatal("a parsed document reports itself unreadable")
			}
			if tt.matches == nil {
				if identity.NetworkMatches != nil || identity.NetworkPassphrase != nil {
					t.Fatalf("a document naming no passphrase printed %+v", identity)
				}
				return
			}
			if identity.NetworkPassphrase == nil || *identity.NetworkPassphrase != *tt.stated {
				t.Fatalf("passphrase = %v, want it carried through", identity.NetworkPassphrase)
			}
			if identity.NetworkMatches == nil {
				t.Fatal("a stated passphrase was not compared")
			}
			if *identity.NetworkMatches != *tt.matches {
				t.Errorf("matches = %t, want %t", *identity.NetworkMatches, *tt.matches)
			}
		})
	}
}

// TestFactsCarryTheShapeOfAReadableDocument covers the keys a reachable result
// carries, including the entries the parser rejected rather than kept: a report that
// showed only the two good addresses would read as a document with nothing wrong in it.
func TestFactsCarryTheShapeOfAReadableDocument(t *testing.T) {
	t.Parallel()
	result := &sep1.Result{
		HomeDomain:  "example.com",
		FailedAt:    sep1.StepOK,
		Status:      200,
		Hops:        2,
		Bytes:       196,
		CORSPresent: true,
		Identity: sep1.Identity{
			NetworkPassphrase: text(requestedNetwork),
			Version:           text("v0.8.3"),
			Accounts:          []string{testAccount, secondAccount},
			MalformedAccounts: 1,
		},
	}
	identity := Facts(Context{Identity: result, Expected: requestedNetwork}).Identity
	if identity.Step != "ok" || identity.Hops != 2 || identity.Bytes != 196 || !identity.CORSPresent {
		t.Errorf("fetch shape lost: %+v", identity)
	}
	if identity.SigningKey != nil || identity.HorizonURL != nil {
		t.Error("a key the document never named printed as present")
	}
	if len(identity.Accounts) != 2 || identity.MalformedAccounts != 1 {
		t.Errorf("accounts = %d listed, %d malformed", len(identity.Accounts), identity.MalformedAccounts)
	}
}
