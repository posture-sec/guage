package main

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testNetwork     = "testnet"
	accountOne      = "GBK4NFJ7SVMUNPBIYBC4WNMCTKI6Z66MYYWZQSSLQNOSUSTMKVUKE7M3"
	accountMissing  = "GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG"
	testPassphrase  = "Test SDF Network ; September 2015"
	mainnetPassphr  = "Public Global Stellar Network ; September 2015"
	fixtureRoot     = "../../testdata/horizon/root-testnet.json"
	fixtureAccount  = "../../testdata/horizon/1-default.json"
	fixtureNotFound = "../../testdata/horizon/10-not-found.json"
)

type endpoint struct {
	root          []byte
	balance       []byte
	accountStatus int
	always500     bool
	oversize      bool
}

// startEndpoint serves a Horizon root and one account from the committed
// fixtures, so the CLI is exercised end to end without the real network.
func startEndpoint(t *testing.T, e endpoint) *httptest.Server {
	t.Helper()
	if e.root == nil {
		e.root = readFixture(t, fixtureRoot)
	}
	if e.balance == nil && !e.oversize {
		e.balance = readFixture(t, fixtureAccount)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if e.always500 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"title":"internal"}`))
			return
		}
		if r.URL.Path == "/" {
			if e.oversize {
				w.Header().Set("Content-Type", "application/hal+json; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(bytes.Repeat([]byte(" "), 1<<20+1))
				return
			}
			w.Header().Set("Content-Type", "application/hal+json; charset=utf-8")
			_, _ = w.Write(e.root)
			return
		}
		status := e.accountStatus
		if status == 0 {
			status = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/hal+json; charset=utf-8")
		w.WriteHeader(status)
		if e.balance != nil {
			_, _ = w.Write(e.balance)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return body
}

func execCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runCLI(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestDispatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "no arguments", wantCode: 2, wantStderr: "usage: stellar-gauge"},
		{name: "unknown command", args: []string{"bogus"}, wantCode: 2, wantStderr: `unknown command "bogus"`},
		{name: "version", args: []string{"version"}, wantCode: 0, wantStdout: "stellar-gauge " + version},
		{name: "help", args: []string{"help"}, wantCode: 0, wantStdout: "usage: stellar-gauge"},
		{name: "check help", args: []string{"check", "-h"}, wantCode: 0},
		{name: "audit help", args: []string{"audit", "-h"}, wantCode: 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := execCLI(t, tc.args...)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, tc.wantCode, stderr)
			}
			if !strings.Contains(stdout, tc.wantStdout) {
				t.Errorf("stdout %q does not contain %q", stdout, tc.wantStdout)
			}
			if !strings.Contains(stderr, tc.wantStderr) {
				t.Errorf("stderr %q does not contain %q", stderr, tc.wantStderr)
			}
		})
	}
}

func TestCheckRejectsBadArguments(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{
			name:    "no network",
			args:    []string{"check", "--endpoint", "https://example.invalid", accountOne},
			wantMsg: "--network is required",
		},
		{
			name:    "no endpoint",
			args:    []string{"check", "--network", testNetwork, accountOne},
			wantMsg: "exactly one --endpoint is required",
		},
		{
			name:    "two endpoints",
			args:    []string{"check", "--network", testNetwork, "--endpoint", "https://a.invalid", "--endpoint", "https://b.invalid", accountOne},
			wantMsg: "exactly one --endpoint is required",
		},
		{
			name:    "no accounts",
			args:    []string{"check", "--network", testNetwork, "--endpoint", "https://example.invalid"},
			wantMsg: "at least one G account address is required",
		},
		{
			name:    "bad fail-on",
			args:    []string{"check", "--network", testNetwork, "--endpoint", "https://example.invalid", "--fail-on", "info", accountOne},
			wantMsg: "--fail-on must be high or warn",
		},
		{
			name:    "bad format",
			args:    []string{"check", "--network", testNetwork, "--endpoint", "https://example.invalid", "--format", "yaml", accountOne},
			wantMsg: "--format must be text or json",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, _, stderr := execCLI(t, tc.args...)
			if code != 2 {
				t.Fatalf("exit code = %d, want 2 (stderr %q)", code, stderr)
			}
			if !strings.Contains(stderr, tc.wantMsg) {
				t.Errorf("stderr %q does not contain %q", stderr, tc.wantMsg)
			}
		})
	}
}

func TestCheckRejectsNonAccountValuesWithoutEchoingThem(t *testing.T) {
	t.Parallel()
	seed := testStrkey(secretSeedVersion, 32)
	muxed := testStrkey(muxedAccountVersion, 40)
	contractBytes := append([]byte{contractVersion << 3}, bytes.Repeat([]byte{0x7f}, 32)...)
	contract := checksummed(contractBytes)

	cases := []struct {
		name    string
		value   string
		wantMsg string
	}{
		{name: "secret seed", value: seed, wantMsg: "rotate that key"},
		{name: "muxed account", value: muxed, wantMsg: "muxed account addresses are not supported"},
		{name: "contract", value: contract, wantMsg: "contract addresses are not supported"},
		{name: "not a strkey", value: "not-an-account", wantMsg: "not a valid Stellar account address"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := execCLI(t, "check", "--network", testNetwork, "--endpoint", "https://example.invalid", tc.value)
			if code != 2 {
				t.Fatalf("exit code = %d, want 2", code)
			}
			if !strings.Contains(stderr, tc.wantMsg) {
				t.Errorf("stderr %q does not contain %q", stderr, tc.wantMsg)
			}
			if strings.Contains(stderr, tc.value) || strings.Contains(stdout, tc.value) {
				t.Errorf("the rejected value %q reached the output", tc.value)
			}
		})
	}
}

// strkey version bytes, from SEP-23.
const (
	secretSeedVersion   = 18
	muxedAccountVersion = 12
	contractVersion     = 2
)

// testStrkey builds a valid strkey at test time so no seed-shaped literal is
// ever committed to the repository.
func testStrkey(version byte, size int) string {
	raw := make([]byte, 1, size+3)
	raw[0] = version << 3
	for i := 0; i < size; i++ {
		raw = append(raw, byte(i+1))
	}
	return checksummed(raw)
}

func checksummed(raw []byte) string {
	var crc uint16
	for _, b := range raw {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	raw = append(raw, byte(crc), byte(crc>>8))
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
}

func TestCheckAuditsAnAccount(t *testing.T) {
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
	for _, want := range []string{
		"Gauge " + version,
		"network: " + testPassphrase,
		"ledger: 5466897",
		"account " + accountOne,
		"exit code: 0",
		"limits: Gauge reads public ledger state only",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
}

func TestCheckWritesJSON(t *testing.T) {
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
		ToolVersion string `json:"tool_version"`
		Network     string `json:"network"`
		Ledger      int64  `json:"ledger"`
		ExitCode    int    `json:"exit_code"`
		Accounts    []struct {
			Snapshot struct {
				ID string `json:"id"`
			} `json:"snapshot"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if payload.ToolVersion != version || payload.Network != testPassphrase || payload.ExitCode != 0 {
		t.Errorf("unexpected JSON header: %+v", payload)
	}
	if len(payload.Accounts) != 1 || payload.Accounts[0].Snapshot.ID != accountOne {
		t.Errorf("unexpected JSON accounts: %+v", payload.Accounts)
	}
}

func TestCheckFailsClosed(t *testing.T) {
	t.Parallel()
	down := startEndpoint(t, endpoint{})
	down.Close()

	cases := []struct {
		name     string
		server   endpoint
		serves   bool
		endpoint string
		network  string
		accounts []string
		wantCode int
	}{
		{name: "endpoint refused", endpoint: down.URL, network: testNetwork, accounts: []string{accountOne}, wantCode: 3},
		{name: "server error", server: endpoint{always500: true}, serves: true, network: testNetwork, accounts: []string{accountOne}, wantCode: 3},
		{name: "missing account", server: endpoint{accountStatus: http.StatusNotFound, balance: readFixture(t, fixtureNotFound)}, serves: true, network: testNetwork, accounts: []string{accountMissing}, wantCode: 3},
		{name: "wrong network", server: endpoint{}, serves: true, network: "mainnet", accounts: []string{accountOne}, wantCode: 3},
		{name: "oversize response", server: endpoint{oversize: true}, serves: true, network: testNetwork, accounts: []string{accountOne}, wantCode: 3},
		{name: "malformed root", server: endpoint{root: []byte(`{"network_passphrase":`)}, serves: true, network: testNetwork, accounts: []string{accountOne}, wantCode: 3},
		{name: "malformed account", server: endpoint{balance: []byte(`{"id":`)}, serves: true, network: testNetwork, accounts: []string{accountOne}, wantCode: 3},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			endpointURL := tc.endpoint
			if tc.serves {
				endpointURL = startEndpoint(t, tc.server).URL
			}
			args := append([]string{"check", "--network", tc.network, "--endpoint", endpointURL}, tc.accounts...)
			code, stdout, stderr := execCLI(t, args...)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, tc.wantCode, stderr)
			}
			if stdout != "" {
				t.Errorf("a failed run wrote a report to stdout: %q", stdout)
			}
			if !strings.Contains(stderr, "stellar-gauge: ") {
				t.Errorf("stderr %q does not name the tool", stderr)
			}
		})
	}
}

func writePolicy(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gauge.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing the policy: %v", err)
	}
	return path
}

func policyFor(endpointURL string) string {
	return `version = 1
network = "testnet"
endpoints = ["` + endpointURL + `"]

[[account]]
id = "` + accountOne + `"
name = "treasury"
role = "treasury"
`
}

func TestAuditRunsFromAPolicyFile(t *testing.T) {
	t.Parallel()
	srv := startEndpoint(t, endpoint{})
	path := writePolicy(t, policyFor(srv.URL))
	code, stdout, stderr := execCLI(t, "audit", "--config", path)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stdout, "account "+accountOne) {
		t.Errorf("stdout does not mention the audited account:\n%s", stdout)
	}
}

func TestAuditFailsClosed(t *testing.T) {
	t.Parallel()
	srv := startEndpoint(t, endpoint{})
	seed := testStrkey(secretSeedVersion, 32)
	cases := []struct {
		name     string
		body     string
		wantCode int
		wantMsg  string
		never    string
	}{
		{
			name:     "unknown key",
			body:     policyFor(srv.URL) + "\nsurprise = true\n",
			wantCode: 2,
			wantMsg:  "does not define",
		},
		{
			name:     "secret seed",
			body:     policyFor(srv.URL) + "\n[[accept]]\nrule = \"flag-drift\"\naccount = \"" + seed + "\"\nreason = \"x\"\n",
			wantCode: 2,
			wantMsg:  "seed",
			never:    seed,
		},
		{
			name:     "two endpoints",
			body:     strings.Replace(policyFor(srv.URL), "endpoints = [\"", "endpoints = [\"https://second.invalid\", \"", 1),
			wantCode: 2,
			wantMsg:  "multi-endpoint",
		},
		{
			name:     "no accounts",
			body:     "version = 1\nnetwork = \"testnet\"\nendpoints = [\"" + srv.URL + "\"]\n",
			wantCode: 2,
			wantMsg:  "no accounts",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := writePolicy(t, tc.body)
			code, stdout, stderr := execCLI(t, "audit", "--config", path)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, tc.wantCode, stderr)
			}
			if !strings.Contains(stderr, tc.wantMsg) {
				t.Errorf("stderr %q does not contain %q", stderr, tc.wantMsg)
			}
			if tc.never != "" && (strings.Contains(stderr, tc.never) || strings.Contains(stdout, tc.never)) {
				t.Errorf("the rejected value reached the output")
			}
		})
	}
}

func TestAuditRejectsAPositionalArgument(t *testing.T) {
	t.Parallel()
	code, _, stderr := execCLI(t, "audit", accountOne)
	if code != 2 || !strings.Contains(stderr, "no positional arguments") {
		t.Fatalf("exit code = %d, stderr %q", code, stderr)
	}
}

func TestAuditRejectsMarkdownUntilItExists(t *testing.T) {
	t.Parallel()
	code, _, stderr := execCLI(t, "audit", "--format", "markdown")
	if code != 2 || !strings.Contains(stderr, "markdown output is not implemented yet") {
		t.Fatalf("exit code = %d, stderr %q", code, stderr)
	}
}

func TestAuditReportsAMissingConfig(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "absent.toml")
	code, _, stderr := execCLI(t, "audit", "--config", missing)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (stderr %q)", code, stderr)
	}
}

func TestCheckPassphraseIsNotHardcodedToTheEndpoint(t *testing.T) {
	t.Parallel()
	// A mainnet root must not satisfy a testnet request.
	root := []byte(`{"network_passphrase":"` + mainnetPassphr + `","history_latest_ledger":1}`)
	srv := startEndpoint(t, endpoint{root: root})
	code, stdout, _ := execCLI(t, "check", "--network", testNetwork, "--endpoint", srv.URL, accountOne)
	if code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}
	if stdout != "" {
		t.Errorf("a failed run wrote a report: %q", stdout)
	}
}
