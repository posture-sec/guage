package rules

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/sep1"
)

// TestFactsContract pins the three promises facts.go makes in prose: the same
// context prints the same facts, a member with no answer is nil rather than a
// zero, and nothing in the block can become a finding. The last one is the
// reason the type exists at all: report.Exit reads findings only, so a fact has
// no severity to sort by, no rule ID to accept and no fingerprint to match an
// [[accept]] entry against.
func TestFactsContract(t *testing.T) {
	t.Parallel()

	t.Run("same context prints the same facts", func(t *testing.T) {
		t.Parallel()
		for id, ctx := range factsContractContexts() {
			first := Facts(ctx)
			second := Facts(ctx)
			if !reflect.DeepEqual(first, second) {
				t.Errorf("context %q printed different facts on two calls:\nfirst  %+v\nsecond %+v", id, first, second)
			}
		}
	})

	t.Run("reachability answers only for a read account", func(t *testing.T) {
		t.Parallel()
		readable := snapshotWith(model.Thresholds{Low: 1, Medium: 1, High: 1},
			signerEntry(testAccount, model.SignerEd25519, 1))
		for _, tt := range []struct {
			name string
			ctx  Context
			want bool
		}{
			{"readable snapshot", Context{Snapshot: readable}, true},
			{"missing account", Context{Snapshot: readable, Missing: true}, false},
			{"mismatched endpoint", Context{Snapshot: readable, Mismatch: true}, false},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got := Facts(tt.ctx).Reachability
				if (got != nil) != tt.want {
					t.Fatalf("Reachability = %v, want answer %t", got, tt.want)
				}
			})
		}
	})

	t.Run("identity is nil exactly when nothing was fetched", func(t *testing.T) {
		t.Parallel()
		for _, tt := range []struct {
			name   string
			ident  *sep1.Result
			wantOK bool
		}{
			{"no fetch", nil, false},
			{"unparsed document", &sep1.Result{HomeDomain: "example.com", FailedAt: sep1.StepParse, Status: 200}, true},
			{"parsed document", &sep1.Result{HomeDomain: "example.com", FailedAt: sep1.StepOK, Status: 200}, true},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got := Facts(Context{Identity: tt.ident, Expected: requestedNetwork}).Identity
				if (got != nil) != tt.wantOK {
					t.Fatalf("Identity = %v, want present %t", got, tt.wantOK)
				}
			})
		}
	})

	t.Run("no fact can become a finding", func(t *testing.T) {
		t.Parallel()
		factsType := reflect.TypeOf(model.AccountFacts{})
		for i := 0; i < factsType.NumField(); i++ {
			field := factsType.Field(i)
			if carriesVerdict(field.Type) {
				t.Errorf("model.AccountFacts.%s has type %s, which can carry a finding or a severity; "+
					"a fact line must not be sortable, acceptable or counted at exit", field.Name, field.Type)
			}
		}
		// The type walk cannot see a renamed JSON key, so the wire form of every
		// context this package can build is checked too.
		for id, ctx := range factsContractContexts() {
			data, err := json.Marshal(Facts(ctx))
			if err != nil {
				t.Fatalf("context %q: marshalling facts: %v", id, err)
			}
			var decoded any
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("context %q: decoding facts: %v", id, err)
			}
			for _, key := range verdictKeys(decoded) {
				t.Errorf("context %q printed a %q key in its facts", id, key)
			}
		}
	})
}

// factsContractContexts is one context per registered rule plus a fully
// populated one, so determinism and the wire check cover missing, mismatched,
// declared and fetched shapes rather than a single happy path.
func factsContractContexts() map[string]Context {
	contexts := map[string]Context{}
	for id, row := range ruleIDFiring() {
		contexts[id] = row.ctx
	}
	contexts["readable with facts"] = factsContractContext()
	return contexts
}

// factsContractContext populates every member the contract covers, including a
// trustline and a fetched document, so the reflection and wire checks see the
// largest block this package can build.
func factsContractContext() Context {
	domain := "example.org"
	return Context{
		Account: policyAccountWithDeclaration(domain),
		Snapshot: model.AccountSnapshot{
			ID:         testAccount,
			HomeDomain: domain,
			Thresholds: model.Thresholds{Low: 1, Medium: 1, High: 1},
			Signers: []model.Signer{
				signerEntry(testAccount, model.SignerEd25519, 1),
				signerEntry(secondAccount, model.SignerEd25519, 1),
			},
			Balances: []model.Balance{
				{AssetCode: "USDC", AssetIssuer: secondAccount, IsAuthorized: true},
			},
		},
		Identity: &sep1.Result{
			HomeDomain: domain,
			FailedAt:   sep1.StepOK,
			Status:     200,
			Bytes:      196,
			Identity:   sep1.Identity{NetworkPassphrase: text(requestedNetwork)},
		},
		Expected: requestedNetwork,
		Issuers: map[string]IssuerObservation{
			secondAccount: {Flags: &model.Flags{AuthRequired: true}},
		},
	}
}

func policyAccountWithDeclaration(domain string) policy.Account {
	expectedSigners := []policy.ExpectedSigner{{Key: testAccount, Weight: 1}}
	return policy.Account{
		ID:                 testAccount,
		Name:               "operator issuer",
		Role:               model.RoleIssuer,
		HomeDomain:         text(domain),
		ExpectLocked:       boolean(false),
		ExpectedSigners:    &expectedSigners,
		ExpectedThresholds: &policy.ExpectedThresholds{Low: number(1)},
		ExpectedFlags:      &policy.ExpectedFlags{AuthRequired: boolean(true)},
	}
}

// carriesVerdict reports whether a type, unwrapped through pointers, slices,
// arrays and maps, is a finding or a severity. A slice of facts is fine and a
// slice of findings is not, which is the difference the task asks for.
func carriesVerdict(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return carriesVerdict(t.Elem())
	case reflect.Map:
		return carriesVerdict(t.Elem()) || carriesVerdict(t.Key())
	}
	return t == reflect.TypeOf(model.Finding{}) || t == reflect.TypeOf(model.Severity(0))
}

// verdictKeys lists every key named severity or accepted anywhere in a decoded
// JSON document.
func verdictKeys(value any) []string {
	var found []string
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			if key == "severity" || key == "accepted" {
				found = append(found, key)
			}
			found = append(found, verdictKeys(child)...)
		}
	case []any:
		for _, child := range node {
			found = append(found, verdictKeys(child)...)
		}
	}
	return found
}
