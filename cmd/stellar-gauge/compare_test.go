package main

import (
	"strings"
	"testing"
)

const fixtureDisagrees = "../../testdata/horizon/16-endpoint-disagrees.json"

// Task 5.1 end to end: two fake Horizons behind the real client, so the decode,
// the passphrase check on each endpoint and the comparison all run as shipped.

func TestCheckWithEndpointsThatAgreePasses(t *testing.T) {
	t.Parallel()
	first := startEndpoint(t, endpoint{})
	second := startEndpoint(t, endpoint{})
	code, stdout, stderr := execCLI(t, "check", "--network", testNetwork,
		"--endpoint", first.URL, "--endpoint", second.URL, accountOne)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr %q)\n%s", code, stderr, stdout)
	}
	if strings.Contains(stdout, "src-disagreement") {
		t.Fatalf("agreeing endpoints reported a disagreement:\n%s", stdout)
	}
}

func TestCheckReportsEndpointsThatDisagree(t *testing.T) {
	t.Parallel()
	first := startEndpoint(t, endpoint{})
	second := startEndpoint(t, endpoint{balance: readFixture(t, fixtureDisagrees)})
	for _, format := range []string{"text", "json"} {
		format := format
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := execCLI(t, "check", "--network", testNetwork, "--format", format,
				"--endpoint", first.URL, "--endpoint", second.URL, accountOne)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1 (stderr %q)\n%s", code, stderr, stdout)
			}
			for _, want := range []string{"src-disagreement", "endpoints[1] disagrees with endpoints[0] on: signers, thresholds"} {
				if !strings.Contains(stdout, want) {
					t.Errorf("output does not carry %q:\n%s", want, stdout)
				}
			}
		})
	}
}

func TestCheckReportsAnAccountOneEndpointDoesNotServe(t *testing.T) {
	t.Parallel()
	first := startEndpoint(t, endpoint{})
	second := startEndpoint(t, endpoint{accountStatus: 404, balance: readFixture(t, fixtureNotFound)})
	code, stdout, stderr := execCLI(t, "check", "--network", testNetwork,
		"--endpoint", first.URL, "--endpoint", second.URL, accountOne)
	if code != 1 || !strings.Contains(stdout, "on: existence") {
		t.Fatalf("exit code = %d, want 1 naming existence (stderr %q)\n%s", code, stderr, stdout)
	}
}

func TestCheckFailsClosedWhenTheSecondEndpointFails(t *testing.T) {
	t.Parallel()
	first := startEndpoint(t, endpoint{})
	second := startEndpoint(t, endpoint{always500: true})
	code, stdout, stderr := execCLI(t, "check", "--network", testNetwork,
		"--endpoint", first.URL, "--endpoint", second.URL, accountOne)
	if code != 3 {
		t.Fatalf("exit code = %d, want 3 (stderr %q)\n%s", code, stderr, stdout)
	}
	if !strings.Contains(stderr, "endpoints[1]") {
		t.Fatalf("stderr does not name the endpoint that failed: %q", stderr)
	}
}

func TestAuditComparesEveryPolicyEndpoint(t *testing.T) {
	t.Parallel()
	first := startEndpoint(t, endpoint{})
	second := startEndpoint(t, endpoint{balance: readFixture(t, fixtureDisagrees)})
	body := strings.Replace(policyFor(first.URL), `endpoints = ["`, `endpoints = ["`+second.URL+`", "`, 1)
	code, stdout, stderr := execCLI(t, "audit", "--config", writePolicy(t, body))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr %q)\n%s", code, stderr, stdout)
	}
	// The policy lists the disagreeing endpoint first, so it is the one the rules
	// read and the captured one is endpoints[1]; the fields are the same either way.
	if !strings.Contains(stdout, "endpoints[1] disagrees with endpoints[0] on: signers, thresholds") {
		t.Fatalf("output does not carry the disagreement:\n%s", stdout)
	}
}
