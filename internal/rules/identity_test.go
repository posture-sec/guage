package rules

import (
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/sep1"
)

const testDomain = "example.com"

func text(value string) *string { return &value }

// listed is a stellar.toml that was fetched under the guard and names the given
// accounts. The passphrase is the testnet one so a context built on it matches the
// network the run asked about.
func listed(accounts ...string) *sep1.Result {
	return &sep1.Result{
		HomeDomain: testDomain,
		FailedAt:   sep1.StepOK,
		Bytes:      120,
		Status:     200,
		Identity:   sep1.Identity{NetworkPassphrase: text(testnetPassphrase), Accounts: accounts},
	}
}

func failed(step sep1.Step, detail string, status int) *sep1.Result {
	return &sep1.Result{HomeDomain: testDomain, FailedAt: step, Detail: detail, Status: status}
}

func identityContext(account policy.Account, snapshot model.AccountSnapshot, result *sep1.Result) Context {
	snapshot.ID = testAccount
	return Context{
		Account:  account,
		Snapshot: snapshot,
		Identity: result,
		Network:  testnetPassphrase,
		Expected: testnetPassphrase,
	}
}

func declared(domain string) policy.Account {
	return policy.Account{ID: testAccount, HomeDomain: text(domain)}
}

func wantOneFinding(t *testing.T, findings []model.Finding, ruleID string) model.Finding {
	t.Helper()
	if len(findings) != 1 {
		t.Fatalf("got %d findings %+v, want exactly %q", len(findings), findings, ruleID)
	}
	f := findings[0]
	if f.RuleID != ruleID {
		t.Errorf("rule id = %q, want %q", f.RuleID, ruleID)
	}
	if f.Severity != model.SevWarn {
		t.Errorf("severity = %v, want warn", f.Severity)
	}
	if f.Account != testAccount {
		t.Errorf("account = %q, want the audited account", f.Account)
	}
	return f
}

func TestIDNoHomeDomain(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		account  policy.Account
		observed string
		want     bool
	}{
		{name: "declared and absent", account: declared(testDomain), want: true},
		{name: "declared and present", account: declared(testDomain), observed: testDomain},
		{name: "declared elsewhere", account: declared("other.example"), observed: testDomain},
		{name: "undeclared and absent", observed: ""},
		{name: "empty declaration is not a declaration", account: declared("")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := identityContext(tc.account, model.AccountSnapshot{HomeDomain: tc.observed}, listed(testAccount))
			findings := idNoHomeDomain(ctx)
			if !tc.want {
				if len(findings) != 0 {
					t.Fatalf("got %+v, want no findings", findings)
				}
				return
			}
			f := wantOneFinding(t, findings, "id-no-home-domain")
			want := `home_domain declared "example.com", observed none`
			if f.Evidence != want {
				t.Errorf("evidence = %q, want %q", f.Evidence, want)
			}
		})
	}
}

// An account with no declared home domain is not a finding, which is what lets
// check mode, where nothing is declared, stay silent on identity.
func TestIDNoHomeDomainSkipsUnreadAccounts(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"missing", "mismatch"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := identityContext(declared(testDomain), model.AccountSnapshot{}, nil)
			ctx.Missing = name == "missing"
			ctx.Mismatch = name == "mismatch"
			if got := idNoHomeDomain(ctx); len(got) != 0 {
				t.Errorf("got %+v, want no findings for an account that was not read", got)
			}
		})
	}
}

func TestIDTomlUnreachable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		result     *sep1.Result
		observed   string
		wantAbsent bool
		wantIn     []string
	}{
		{
			name:     "blocked by the guard",
			result:   failed(sep1.StepGuard, "the host name resolves to a private address, which Gauge will not dial", 0),
			observed: testDomain,
			wantIn:   []string{"step guard", "private address"},
		},
		{
			name:     "the server refused it",
			result:   failed(sep1.StepStatus, "the server refused the document", 404),
			observed: testDomain,
			wantIn:   []string{"step status", "status 404"},
		},
		{
			name:     "the document parsed as no one",
			result:   failed(sep1.StepNoKeys, "the document names no identity field", 200),
			observed: testDomain,
			wantIn:   []string{"step no-recognized-keys"},
		},
		{
			// Fail closed: an observed domain with no attempt recorded is not a pass.
			name:     "no attempt was made",
			result:   nil,
			observed: testDomain,
			wantIn:   []string{"step not-attempted", testDomain},
		},
		{
			name:       "reachable",
			result:     listed(testAccount),
			observed:   testDomain,
			wantAbsent: true,
		},
		{
			// Nothing was fetched because there is nothing to fetch for.
			name:       "no domain observed",
			result:     nil,
			wantAbsent: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := identityContext(policy.Account{ID: testAccount}, model.AccountSnapshot{HomeDomain: tc.observed}, tc.result)
			findings := idTomlUnreachable(ctx)
			if tc.wantAbsent {
				if len(findings) != 0 {
					t.Fatalf("got %+v, want no findings", findings)
				}
				return
			}
			f := wantOneFinding(t, findings, "id-toml-unreachable")
			for _, want := range tc.wantIn {
				if !strings.Contains(f.Evidence, want) {
					t.Errorf("evidence %q does not name %q", f.Evidence, want)
				}
			}
			if !strings.Contains(f.Message, testDomain) && tc.result != nil {
				t.Errorf("message %q does not name the domain that failed", f.Message)
			}
		})
	}
}

// A body is never part of a finding, and neither is a Detail the guard did not
// write. The rule only ever quotes the step name and its own fixed phrase.
func TestIDTomlUnreachableNamesNoBody(t *testing.T) {
	t.Parallel()
	ctx := identityContext(policy.Account{ID: testAccount}, model.AccountSnapshot{HomeDomain: testDomain},
		failed(sep1.StepParse, "the document is not a configuration file this parser accepts", 200))
	f := wantOneFinding(t, idTomlUnreachable(ctx), "id-toml-unreachable")
	if strings.Contains(f.Evidence, "NETWORK_PASSPHRASE") || strings.Contains(f.Evidence, "=") {
		t.Errorf("evidence %q quotes document content", f.Evidence)
	}
}

func TestIDTomlAccountUnlisted(t *testing.T) {
	t.Parallel()
	// A real address from the captured corpus, so the list under test holds
	// plausible entries rather than one Gauge invented.
	other := "GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ"
	cases := []struct {
		name   string
		result *sep1.Result
		wantIn []string
	}{
		{name: "listed", result: listed(testAccount)},
		{name: "reachable with no accounts", result: listed(), wantIn: []string{"lists 0 accounts", testDomain}},
		{name: "another account only", result: listed(other), wantIn: []string{"lists 1 account"}},
		{name: "with a malformed entry", result: func() *sep1.Result {
			r := listed(other)
			r.Identity.MalformedAccounts = 2
			return r
		}(), wantIn: []string{"2 entries that are not account addresses"}},
		{name: "unreachable is not this rule", result: failed(sep1.StepGuard, "blocked", 0)},
		{name: "no attempt is not this rule", result: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := identityContext(policy.Account{ID: testAccount}, model.AccountSnapshot{HomeDomain: testDomain}, tc.result)
			findings := idTomlAccountUnlisted(ctx)
			if len(tc.wantIn) == 0 {
				if len(findings) != 0 {
					t.Fatalf("got %+v, want no findings", findings)
				}
				return
			}
			f := wantOneFinding(t, findings, "id-toml-account-unlisted")
			for _, want := range tc.wantIn {
				if !strings.Contains(f.Evidence, want) {
					t.Errorf("evidence %q does not contain %q", f.Evidence, want)
				}
			}
		})
	}
}

func TestIdentityRulesSkipUnreadAccounts(t *testing.T) {
	t.Parallel()
	unreachable := failed(sep1.StepGuard, "blocked", 0)
	for _, name := range []string{"missing", "mismatch"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := identityContext(declared(testDomain), model.AccountSnapshot{HomeDomain: testDomain}, unreachable)
			ctx.Missing = name == "missing"
			ctx.Mismatch = name == "mismatch"
			for rule, check := range map[string]func(Context) []model.Finding{
				"id-no-home-domain":        idNoHomeDomain,
				"id-toml-unreachable":      idTomlUnreachable,
				"id-toml-account-unlisted": idTomlAccountUnlisted,
			} {
				if got := check(ctx); len(got) != 0 {
					t.Errorf("%s raised %+v for an account that was not read", rule, got)
				}
			}
		})
	}
}

// The three identity rules are acceptable risks: unlike a missing account or a
// network mismatch, an operator may decide a dead home domain is fine.
func TestIdentityRulesAreAcceptable(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"id-no-home-domain", "id-toml-unreachable", "id-toml-account-unlisted"} {
		if !policy.Acceptable(id) {
			t.Errorf("%s cannot be accepted, so [[accept]] cannot silence it", id)
		}
	}
}

// Two gaps this test pins rather than invents around. PRD 6.1 says the identity
// rules compare NETWORK_PASSPHRASE with the network of the run, and Appendix A
// says an issuer role expects a home domain, but no rule ID exists for either and
// the role table is task 3.4. The document is still carried so 3.7 can print the
// fact, and neither condition may silently fire a rule with someone else's ID.
func TestIdentityGapsAreFactsNotFindings(t *testing.T) {
	t.Parallel()
	mismatched := listed(testAccount)
	mismatched.Identity.NetworkPassphrase = text(mainnetPassphrase)
	ctx := identityContext(declared("other.example"), model.AccountSnapshot{HomeDomain: testDomain}, mismatched)
	if got := Evaluate(ctx); len(got) != 0 {
		t.Fatalf("got %+v, want no findings while the gaps are unresolved", got)
	}
	if ctx.Identity.Identity.NetworkPassphrase == nil || *ctx.Identity.Identity.NetworkPassphrase != mainnetPassphrase {
		t.Fatal("the document's passphrase was dropped, so no later rule or fact can name it")
	}
}

// A home domain that is only whitespace is an account that publishes nothing, which
// is the shape one corpus capture actually carries. It must not raise an unverified
// identity finding, it must not hide a domain the policy declared, and a stale result
// left in the context by some other caller must not make the toml rules speak about it.
// A non-ASCII host keeps the opposite behavior, refused at the guard and reported as
// unreachable, which internal/sep1's tests pin.
func TestABlankObservedDomainIsNoDomain(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, value string }{
		{"empty", ""},
		{"single space", " "},
		{"tab and space", "\t "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			snapshot := model.AccountSnapshot{HomeDomain: tc.value}
			if got := Evaluate(identityContext(policy.Account{}, snapshot, nil)); len(got) != 0 {
				t.Fatalf("findings = %+v, want none for a domain that is only whitespace", got)
			}
			stale := failed(sep1.StepHostname, "the home domain is not a host name Gauge will contact", 0)
			if got := Evaluate(identityContext(policy.Account{}, snapshot, stale)); len(got) != 0 {
				t.Fatalf("findings = %+v, want the toml rules silent about an account that publishes no domain", got)
			}
			ctx := identityContext(declared(testDomain), snapshot, nil)
			wantOneFinding(t, idNoHomeDomain(ctx), "id-no-home-domain")
		})
	}
}
