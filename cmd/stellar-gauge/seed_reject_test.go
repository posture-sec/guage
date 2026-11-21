package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/sep1"
	"github.com/posture-sec/guage/internal/source"
)

// TestSeedIsRefusedBeforeAnyClientIsBuilt complements the output test by
// observing the ordering: a seed must be refused before a source or an identity
// fetcher exists, whatever position it holds among otherwise valid addresses.
func TestSeedIsRefusedBeforeAnyClientIsBuilt(t *testing.T) {
	t.Parallel()
	seed := testStrkey(secretSeedVersion, 32)

	cases := []struct {
		name string
		args []string
	}{
		{
			name: "seed alone",
			args: []string{"check", "--network", testNetwork, "--endpoint", "https://example.invalid", seed},
		},
		{
			name: "valid address then seed",
			args: []string{"check", "--network", testNetwork, "--endpoint", "https://example.invalid", accountOne, seed},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sourceBuilt := false
			identityBuilt := false
			d := deps{
				newSource: func(string) (source.Source, error) {
					sourceBuilt = true
					t.Fatal("a seed argument reached the Horizon source factory")
					return nil, nil
				},
				newIdentity: func() (sep1.Fetcher, error) {
					identityBuilt = true
					t.Fatal("a seed argument reached the identity fetcher factory")
					return nil, nil
				},
			}

			var stdout, stderr bytes.Buffer
			code := runCLI(tc.args, &stdout, &stderr, d)

			if code != 2 {
				t.Fatalf("exit code = %d, want 2 (stderr %q)", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), "rotate that key") {
				t.Errorf("stderr %q does not contain the rotate-the-key refusal", stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("a refused run wrote to stdout: %q", stdout.String())
			}
			if sourceBuilt {
				t.Error("the Horizon source factory ran before the seed was refused")
			}
			if identityBuilt {
				t.Error("the identity fetcher factory ran before the seed was refused")
			}
			if strings.Contains(stdout.String(), seed) || strings.Contains(stderr.String(), seed) {
				t.Error("the rejected seed reached the output")
			}
		})
	}
}
