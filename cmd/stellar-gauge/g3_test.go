package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/sep1"
)

// TestGateG3 is the fail-closed gate from ROADMAP.md: every listed failure must
// exit non-zero and must not print a clean report on stdout.
func TestGateG3(t *testing.T) {
	t.Parallel()

	down := startEndpoint(t, endpoint{})
	down.Close()

	missing := filepath.Join(t.TempDir(), "absent.toml")
	seed := testStrkey(secretSeedVersion, 32)

	cases := []struct {
		name        string
		run         func() (int, string, string)
		wantFinding string
	}{
		{
			name: "unreachable endpoint",
			run: func() (int, string, string) {
				return execCLI(t, "check", "--network", testNetwork, "--endpoint", down.URL, accountOne)
			},
		},
		{
			name: "oversize body",
			run: func() (int, string, string) {
				srv := startEndpoint(t, endpoint{oversize: true})
				return execCLI(t, "check", "--network", testNetwork, "--endpoint", srv.URL, accountOne)
			},
		},
		{
			name: "malformed JSON",
			run: func() (int, string, string) {
				srv := startEndpoint(t, endpoint{root: []byte(`{"network_passphrase":`)})
				return execCLI(t, "check", "--network", testNetwork, "--endpoint", srv.URL, accountOne)
			},
		},
		{
			// A passphrase mismatch is a finding about the endpoint, not a
			// transport failure, so the run still reports. It must not report a
			// pass, and it must not read any account from the wrong network.
			name: "wrong passphrase",
			run: func() (int, string, string) {
				srv := startEndpoint(t, endpoint{})
				return execCLI(t, "check", "--network", "mainnet", "--endpoint", srv.URL, accountOne)
			},
			wantFinding: "acct-network-mismatch",
		},
		{
			name: "unknown config key",
			run: func() (int, string, string) {
				path := writePolicy(t, policyFor("https://horizon-testnet.stellar.org")+"\nundocumented = 1\n")
				return execCLI(t, "audit", "--config", path)
			},
		},
		{
			name: "secret-seed input",
			run: func() (int, string, string) {
				path := writePolicy(t, policyFor("https://horizon-testnet.stellar.org")+"\ntrailing = \""+seed+"\"\n")
				return execCLI(t, "audit", "--config", path)
			},
		},
		{
			name: "missing config file",
			run: func() (int, string, string) {
				return execCLI(t, "audit", "--config", missing)
			},
		},
		{
			name: "seed on the command line",
			run: func() (int, string, string) {
				return execCLI(t, "check", "--network", testNetwork, "--endpoint", "https://example.invalid", seed)
			},
		},
		{
			// An identity target the guard refuses is a finding, and a warning at
			// --fail-on warn is enough to fail this gate: an account whose
			// published identity cannot be verified never reads as a pass. The
			// refusal itself is internal/sep1's test; this one is the exit code.
			name: "blocked stellar.toml target",
			run: func() (int, string, string) {
				srv := startEndpoint(t, endpoint{balance: readFixture(t, fixtureClawback)})
				blocked := &offlineIdentity{results: map[string]sep1.Result{
					"komunitin.org": {
						HomeDomain: "komunitin.org",
						FailedAt:   sep1.StepGuard,
						Detail:     "the host name resolves to a private address, which Gauge will not dial",
					},
				}}
				return execCLIWith(t, offlineDeps(blocked),
					"check", "--network", testNetwork, "--endpoint", srv.URL, "--fail-on", "warn", accountMissing)
			},
			wantFinding: "id-toml-unreachable",
		},
		{
			// No fetcher means no identity was verified, so the run stops rather
			// than printing a report that looks clean.
			name: "identity fetcher will not build",
			run: func() (int, string, string) {
				srv := startEndpoint(t, endpoint{})
				return execCLIWith(t, deps{newIdentity: func() (sep1.Fetcher, error) {
					return nil, errors.New("the test refuses to build a fetcher")
				}}, "check", "--network", testNetwork, "--endpoint", srv.URL, accountOne)
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := tc.run()
			if code == 0 {
				t.Fatalf("exit code = 0, want non-zero")
			}
			if tc.wantFinding != "" {
				if code != 1 {
					t.Errorf("exit code = %d, want 1 for a reported finding", code)
				}
				if !strings.Contains(stdout, tc.wantFinding) {
					t.Errorf("stdout does not carry the %s finding: %q", tc.wantFinding, stdout)
				}
				if strings.Contains(stdout, "exit code: 0") {
					t.Error("stdout reports a pass despite the finding")
				}
			} else {
				if stdout != "" {
					t.Errorf("stdout carries a report despite the failure: %q", stdout)
				}
				if stderr == "" {
					t.Error("stderr is empty, so the operator is not told why the run stopped")
				}
			}
			if strings.Contains(stderr, seed) {
				t.Error("stderr echoed the secret seed")
			}
		})
	}
}
