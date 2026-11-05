package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/policy"
)

// PRD 6.1: "Facts always printed". Task 3.7's Done-when is a zero-config `check`
// that prints the facts it read plus the universal rules, so these cases read the
// CLI's own output rather than a renderer's: the fact block has to survive the
// run, the format and the exit code to reach the terminal.
const (
	factsBeforeJudgements = "  findings:"
	universalOnly         = "  declared: nothing; this run read no policy file, so only the universal rules ran"
)

// The example policy is the one file a first run is told to copy, so the strict
// parser accepting it is a shipped guarantee, not a fixture detail.
// `internal/policy/config_test.go` already loads it; what this adds is the half that
// only 3.7 can ask, that the example carries a value for every expectation kind the
// report prints a line for, so the file the CLI points an operator at demonstrates
// the whole `declared:` block rather than one field of it.
func TestExamplePolicyLoadsThroughTheStrictParser(t *testing.T) {
	t.Parallel()
	cfg, err := policy.LoadFile(filepath.Join("..", "..", "gauge.example.toml"))
	if err != nil {
		t.Fatalf("the shipped example does not load: %v", err)
	}
	if cfg.Network != "testnet" || len(cfg.Endpoints) != 1 || len(cfg.Accounts) != 1 {
		t.Fatalf("unexpected example shape: %+v", cfg)
	}
	account := cfg.Accounts[0]
	if string(account.Role) != "treasury" || account.Name != "treasury" {
		t.Errorf("example account declares %q/%q", account.Name, account.Role)
	}
	// Every expectation the report prints a line for is one the example carries,
	// so a nil here means a documented key stopped being read.
	if account.ExpectedSigners == nil || account.ExpectedThresholds == nil ||
		account.ExpectedFlags == nil || account.HomeDomain == nil || account.ExpectLocked == nil {
		t.Fatalf("the example declares a shape the loader dropped: %+v", account)
	}
	if *account.ExpectLocked {
		t.Error("the example declares a lock, which only an issuer may declare")
	}
}

func TestZeroConfigCheckPrintsFactsThenFindings(t *testing.T) {
	t.Parallel()
	srv := startEndpoint(t, endpoint{})
	code, stdout, stderr := execCLI(t, "check",
		"--network", testNetwork,
		"--endpoint", srv.URL,
		accountOne,
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr %q)", code, stderr)
	}
	// 1-default.json holds one weight-1 signer and three thresholds of 0, so every
	// tier needs a single signature and the line is a fact about that account. The
	// account publishes no home domain, so no identity line is owed: a run that
	// fetched nothing has nothing to report beyond the domain line above.
	for _, want := range []string{
		"  minimum signers to authorize: low 1, medium 1, high 1",
		universalOnly,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
	facts := strings.Index(stdout, "  minimum signers to authorize:")
	declared := strings.Index(stdout, universalOnly)
	findings := strings.Index(stdout, factsBeforeJudgements)
	if !(facts >= 0 && facts < declared && declared < findings) {
		t.Errorf("the report does not read facts then judgements: reachability %d, declared %d, findings %d",
			facts, declared, findings)
	}
	if strings.Contains(stdout, "network_passphrase") {
		t.Errorf("a run that fetched nothing printed document values anyway:\n%s", stdout)
	}
}

// A fact line must not be able to fail a run. The served account is the shape that
// `other` reports at INFO and `treasury` at HIGH, so a zero-config check that exits
// 0 here also proves the reachability of a tier no declaration mentions is not
// itself a reason to fail.
func TestZeroConfigCheckFactsCarryNoSeverity(t *testing.T) {
	t.Parallel()
	srv := startEndpoint(t, endpoint{})
	code, stdout, stderr := execCLI(t, "check",
		"--network", testNetwork,
		"--endpoint", srv.URL,
		"--format", "json",
		accountOne,
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr %q)", code, stderr)
	}
	var payload struct {
		Accounts []struct {
			Facts *struct {
				Reachability *struct {
					Low    *int `json:"low"`
					Medium *int `json:"medium"`
					High   *int `json:"high"`
				} `json:"reachability"`
				Declared *json.RawMessage `json:"declared"`
			} `json:"facts"`
			Findings []struct {
				Severity string `json:"severity"`
			} `json:"findings"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if len(payload.Accounts) != 1 || payload.Accounts[0].Facts == nil {
		t.Fatalf("the JSON report carries no fact block: %s", stdout)
	}
	facts := payload.Accounts[0].Facts
	if facts.Reachability == nil || *facts.Reachability.Low != 1 || *facts.Reachability.High != 1 {
		t.Errorf("reachability = %+v", facts.Reachability)
	}
	// An absent declaration is null, not an object of zeros: the check run wrote no
	// policy file, so the report must not imply a role somebody chose.
	if facts.Declared != nil {
		t.Errorf("declared = %s, want null", string(*facts.Declared))
	}
	for _, f := range payload.Accounts[0].Findings {
		if f.Severity == "high" {
			t.Fatalf("a zero-config run reported a HIGH finding at the default --fail-on: %s", stdout)
		}
	}
}

// PRD 6.4 and design principle 5: neither the network nor the endpoint has a
// default, because a silent default to mainnet is how an audit reads the wrong
// ledger. Refusing is only half of it; the message has to name what to type.
func TestCheckUsageNamesBothPublicEndpoints(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "nothing supplied",
			args: []string{"check", accountOne},
			want: []string{"--network is required and has no default"},
		},
		{
			name: "network without an endpoint",
			args: []string{"check", "--network", testNetwork, accountOne},
			want: []string{"exactly one --endpoint is required", "no default",
				"https://horizon.stellar.org", "https://horizon-testnet.stellar.org"},
		},
		{
			name: "two endpoints",
			args: []string{"check", "--network", testNetwork,
				"--endpoint", "https://horizon-testnet.stellar.org",
				"--endpoint", "https://horizon.stellar.org", accountOne},
			want: []string{"exactly one --endpoint is required", "no default",
				"https://horizon.stellar.org", "https://horizon-testnet.stellar.org"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, _, stderr := execCLI(t, tc.args...)
			if code != 2 {
				t.Fatalf("exit code = %d, want 2 (stderr %q)", code, stderr)
			}
			for _, want := range tc.want {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr does not name %q:\n%s", want, stderr)
				}
			}
			if strings.Contains(stderr, "  Pass") {
				t.Errorf("the endpoint sentence carries a double space:\n%s", stderr)
			}
		})
	}
}

// The identity block is a fact about a document the run tried to read, so a domain
// that failed is printed with the step that decided it and none of the keys a parser
// would have filled in. `check` proves it end to end: the fetcher is the offline one
// and the account publishes komunitin.org.
func TestCheckPrintsAnUnreadableDocumentAsAFact(t *testing.T) {
	t.Parallel()
	srv := startEndpoint(t, endpoint{balance: readFixture(t, fixtureClawback)})
	code, stdout, stderr := execCLI(t, "check",
		"--network", testNetwork,
		"--endpoint", srv.URL,
		accountMissing,
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr %q)", code, stderr)
	}
	for _, want := range []string{
		"  identity document: komunitin.org did not serve a readable file",
		"    step: transport",
		"    detail: the CLI test holds no document",
		"    fetch: no response received",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
	for _, absent := range []string{"network_passphrase", "signing_key", "horizon_url", "version:"} {
		if strings.Contains(stdout, absent) {
			t.Errorf("a document that never parsed printed %q anyway:\n%s", absent, stdout)
		}
	}
}

// The other half of a zero-config first run: `audit` with no policy file is not a
// policy, and the operator needs to be told which file to copy rather than left to
// find gauge.example.toml in the repository.
func TestAuditWithoutAPolicyNamesTheExample(t *testing.T) {
	t.Parallel()
	code, _, stderr := execCLI(t, "audit")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (stderr %q)", code, stderr)
	}
	for _, want := range []string{"gauge.example.toml", "gauge.toml", "check"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not mention %q:\n%s", want, stderr)
		}
	}
	if strings.Count(stderr, exampleConfig) != 1 {
		t.Errorf("the example file is named more than once:\n%s", stderr)
	}
}

// A policy file that is unreadable is worth a hint; one that is malformed is not,
// because the operator found gauge.toml and the parser's own sentence is the
// answer. This is the difference between help and noise.
func TestAuditKeepsASyntaxFailureToTheParser(t *testing.T) {
	t.Parallel()
	bad := filepath.Join(t.TempDir(), "gauge.toml")
	if err := os.WriteFile(bad, []byte("version = 2\n"), 0o600); err != nil {
		t.Fatalf("writing the policy: %v", err)
	}
	code, _, stderr := execCLI(t, "audit", "--config", bad)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (stderr %q)", code, stderr)
	}
	if strings.Contains(stderr, exampleConfig) {
		t.Errorf("a policy that exists is told to copy an example:\n%s", stderr)
	}
}
