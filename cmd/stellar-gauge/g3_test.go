package main

import (
	"path/filepath"
	"strings"
	"testing"
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
		name string
		run  func() (int, string, string)
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
			name: "wrong passphrase",
			run: func() (int, string, string) {
				srv := startEndpoint(t, endpoint{})
				return execCLI(t, "check", "--network", "mainnet", "--endpoint", srv.URL, accountOne)
			},
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
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := tc.run()
			if code == 0 {
				t.Fatalf("exit code = 0, want non-zero")
			}
			if stdout != "" {
				t.Errorf("stdout carries a report despite the failure: %q", stdout)
			}
			if stderr == "" {
				t.Error("stderr is empty, so the operator is not told why the run stopped")
			}
			if strings.Contains(stderr, seed) {
				t.Error("stderr echoed the secret seed")
			}
		})
	}
}
