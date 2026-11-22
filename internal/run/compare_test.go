package run

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/rules"
	"github.com/posture-sec/guage/internal/source"
)

const (
	endpointA = "https://horizon-a.example"
	endpointB = "https://horizon-b.example"
	endpointC = "https://horizon-c.example"
)

// endpointFactory hands each endpoint its own fake, so a test can make one of
// them stale, filtered, missing or on another network while the others agree.
func endpointFactory(byEndpoint map[string]source.Source) SourceFactory {
	return func(endpoint string) (source.Source, error) {
		src, ok := byEndpoint[endpoint]
		if !ok {
			return nil, fmt.Errorf("no fake for %s", endpoint)
		}
		return src, nil
	}
}

func agreeingSource() *fakeSource {
	return &fakeSource{
		info: model.NetworkInfo{Passphrase: testnet, Ledger: 500},
		snapshots: map[string]model.AccountSnapshot{
			accountOne: snapshot(accountOne, 10),
			accountTwo: snapshot(accountTwo, 11),
		},
	}
}

func multiJob(byEndpoint map[string]source.Source, endpoints ...string) Job {
	job := baseJob(nil)
	job.Endpoints = endpoints
	job.NewSource = endpointFactory(byEndpoint)
	return job
}

func disagreements(findings []model.Finding) []model.Finding {
	var out []model.Finding
	for _, f := range findings {
		if f.RuleID == "src-disagreement" {
			out = append(out, f)
		}
	}
	return out
}

func TestRunEndpointsThatAgreeRaiseNothing(t *testing.T) {
	t.Parallel()
	a, b := agreeingSource(), agreeingSource()
	// A different latest ledger is two endpoints answering a moment apart, not a
	// disagreement about the account.
	b.info.Ledger = 501
	result, err := Run(context.Background(), multiJob(map[string]source.Source{endpointA: a, endpointB: b}, endpointA, endpointB))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, account := range result.Accounts {
		if len(account.Findings) != 0 {
			t.Fatalf("%s findings = %+v, want none", account.Snapshot.ID, account.Findings)
		}
	}
	if result.ExitCode != model.ExitPass {
		t.Fatalf("exit = %d, want %d", result.ExitCode, model.ExitPass)
	}
	for name, fs := range map[string]*fakeSource{"first": a, "second": b} {
		if got := len(fs.called()); got != 2 {
			t.Fatalf("%s endpoint read %d accounts, want both", name, got)
		}
	}
}

func TestRunReportsEachDisagreeingEndpoint(t *testing.T) {
	t.Parallel()
	a, b, c := agreeingSource(), agreeingSource(), agreeingSource()
	// The second endpoint is missing a signer the first serves: stale or filtered,
	// the run cannot tell which, and it must not pick one.
	filtered := snapshot(accountOne, 10)
	filtered.Signers = filtered.Signers[:1]
	b.snapshots[accountOne] = filtered

	result, err := Run(context.Background(), multiJob(
		map[string]source.Source{endpointA: a, endpointB: b, endpointC: c},
		endpointA, endpointB, endpointC))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	first := disagreements(result.Accounts[0].Findings)
	if len(first) != 1 {
		t.Fatalf("account one disagreements = %+v, want exactly one", result.Accounts[0].Findings)
	}
	got := first[0]
	if got.Severity != model.SevHigh || got.Account != accountOne {
		t.Fatalf("finding = %+v, want HIGH on account one", got)
	}
	if want := "endpoints[1] disagrees with endpoints[0] on: signers"; got.Evidence != want {
		t.Fatalf("evidence = %q, want %q", got.Evidence, want)
	}
	if got.Fingerprint == "" {
		t.Fatal("finding has no fingerprint")
	}
	if len(result.Accounts[1].Findings) != 0 {
		t.Fatalf("account two findings = %+v, want none", result.Accounts[1].Findings)
	}
	// The rules read the first endpoint's snapshot, never a merge: the second
	// endpoint's single signer did not replace the first's two.
	if got := len(result.Accounts[0].Snapshot.Signers); got != 2 {
		t.Fatalf("snapshot has %d signers, want the first endpoint's two", got)
	}
	if result.ExitCode != model.ExitFindings {
		t.Fatalf("exit = %d, want %d", result.ExitCode, model.ExitFindings)
	}
}

func TestRunReportsAnAccountOnlyOneEndpointServes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		missingOn    string
		wantMissing  bool
		wantEvidence string
	}{
		{"first endpoint returns not found", endpointA, true, "endpoints[1] disagrees with endpoints[0] on: existence"},
		{"second endpoint returns not found", endpointB, false, "endpoints[1] disagrees with endpoints[0] on: existence"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			byEndpoint := map[string]*fakeSource{endpointA: agreeingSource(), endpointB: agreeingSource()}
			delete(byEndpoint[tc.missingOn].snapshots, accountOne)
			sources := map[string]source.Source{endpointA: byEndpoint[endpointA], endpointB: byEndpoint[endpointB]}
			job := multiJob(sources, endpointA, endpointB)
			job.Evaluate = rules.Evaluate

			result, err := Run(context.Background(), job)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			account := result.Accounts[0]
			if account.Missing != tc.wantMissing {
				t.Fatalf("missing = %v, want %v: the report follows the first endpoint", account.Missing, tc.wantMissing)
			}
			found := disagreements(account.Findings)
			if len(found) != 1 || found[0].Evidence != tc.wantEvidence {
				t.Fatalf("disagreements = %+v, want one naming existence", found)
			}
			if result.ExitCode != model.ExitFindings {
				t.Fatalf("exit = %d, want %d", result.ExitCode, model.ExitFindings)
			}
		})
	}
}

func TestRunWithAnAccountNoEndpointServesHasNoDisagreement(t *testing.T) {
	t.Parallel()
	a, b := agreeingSource(), agreeingSource()
	delete(a.snapshots, accountOne)
	delete(b.snapshots, accountOne)
	result, err := Run(context.Background(), multiJob(map[string]source.Source{endpointA: a, endpointB: b}, endpointA, endpointB))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Accounts[0].Missing || len(disagreements(result.Accounts[0].Findings)) != 0 {
		t.Fatalf("account one = %+v, want missing and no disagreement", result.Accounts[0])
	}
}

// A second endpoint that cannot be read is not a second opinion that went
// quiet: the run has no comparison, so it has no trustworthy report either.
func TestRunFailsClosedWhenAnyEndpointFails(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		edit func(*fakeSource)
	}{
		{"network read fails", func(f *fakeSource) { f.networkErr = source.ErrStatus }},
		{"account read fails", func(f *fakeSource) { f.accountErr = map[string]error{accountTwo: source.ErrTooLarge} }},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := agreeingSource()
			tc.edit(b)
			_, err := Run(context.Background(), multiJob(map[string]source.Source{endpointA: agreeingSource(), endpointB: b}, endpointA, endpointB))
			if err == nil {
				t.Fatal("Run returned a report although the second endpoint failed")
			}
			if !strings.Contains(err.Error(), "endpoints[1]") {
				t.Fatalf("err = %v, want it to name the endpoint that failed", err)
			}
		})
	}
}

func TestRunReadsNothingWhenAnyEndpointIsOnAnotherNetwork(t *testing.T) {
	t.Parallel()
	const mainnet = "Public Global Stellar Network ; September 2015"
	a, b := agreeingSource(), agreeingSource()
	b.info.Passphrase = mainnet
	job := multiJob(map[string]source.Source{endpointA: a, endpointB: b}, endpointA, endpointB)
	var seen []rules.Context
	job.Evaluate = func(c rules.Context) []model.Finding {
		seen = append(seen, c)
		return rules.Evaluate(c)
	}

	result, err := Run(context.Background(), job)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(a.called()) != 0 || len(b.called()) != 0 {
		t.Fatalf("accounts read on a mixed-network run: %v, %v", a.called(), b.called())
	}
	for _, c := range seen {
		if !c.Mismatch || c.Network != mainnet || c.Snapshot.Endpoint != endpointB {
			t.Fatalf("context = mismatch %v, network %q, endpoint %q; want the endpoint that is off-network named",
				c.Mismatch, c.Network, c.Snapshot.Endpoint)
		}
	}
	for _, account := range result.Accounts {
		if account.Skipped == "" || len(disagreements(account.Findings)) != 0 {
			t.Fatalf("account = %+v, want skipped with no comparison", account)
		}
	}
	if result.ExitCode != model.ExitFindings {
		t.Fatalf("exit = %d, want %d", result.ExitCode, model.ExitFindings)
	}
}

// The loader refuses an [[accept]] for src-disagreement, and the run refuses to
// honor one that reached it some other way, so a role or an acceptance can never
// turn a disagreement into a passing run.
func TestRunNeverAcceptsADisagreement(t *testing.T) {
	t.Parallel()
	a, b := agreeingSource(), agreeingSource()
	stale := snapshot(accountOne, 9)
	b.snapshots[accountOne] = stale
	job := multiJob(map[string]source.Source{endpointA: a, endpointB: b}, endpointA, endpointB)
	job.Accounts[0].Role = model.RoleMerchant
	job.Accepts = []policy.Acceptance{{Rule: "src-disagreement", Account: accountOne, Reason: "lagging replica"}}

	result, err := Run(context.Background(), job)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := disagreements(result.Accounts[0].Findings)
	if len(found) != 1 || found[0].Accepted || found[0].Severity != model.SevHigh {
		t.Fatalf("disagreements = %+v, want one unaccepted HIGH", found)
	}
	if found[0].Evidence != "endpoints[1] disagrees with endpoints[0] on: last_modified_ledger" {
		t.Fatalf("evidence = %q", found[0].Evidence)
	}
	if result.ExitCode != model.ExitFindings {
		t.Fatalf("exit = %d, want %d", result.ExitCode, model.ExitFindings)
	}
}

// Endpoints are read one after another, each through the same bounded pool, so
// adding an endpoint multiplies the requests a run makes but never the sockets
// it holds open at once.
func TestFetchConcurrencyIsBoundedAcrossEndpoints(t *testing.T) {
	t.Parallel()
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	var inflight, peak int32
	shared := func() *blockingSource {
		return &blockingSource{started: started, release: release, inflight: &inflight, peak: &peak}
	}
	job := multiJob(map[string]source.Source{endpointA: shared(), endpointB: shared()}, endpointA, endpointB)
	job.Concurrency = 1

	done := make(chan error, 1)
	go func() {
		_, err := Run(context.Background(), job)
		done <- err
	}()

	<-started
	select {
	case <-started:
		t.Fatal("a second fetch started while the first was in flight")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := atomic.LoadInt32(&peak); got != 1 {
		t.Fatalf("peak concurrency = %d, want 1", got)
	}
}

func TestRunBoundsAndDeduplicatesEndpoints(t *testing.T) {
	t.Parallel()
	many := make(map[string]source.Source)
	var names []string
	for i := 0; i < maxEndpoints+1; i++ {
		name := fmt.Sprintf("https://horizon-%d.example", i)
		many[name] = agreeingSource()
		names = append(names, name)
	}
	if _, err := Run(context.Background(), multiJob(many, names[:maxEndpoints]...)); err != nil {
		t.Fatalf("%d endpoints: %v, want a report", maxEndpoints, err)
	}
	if _, err := Run(context.Background(), multiJob(many, names...)); !errors.Is(err, ErrUsage) {
		t.Fatalf("%d endpoints: err = %v, want ErrUsage", maxEndpoints+1, err)
	}
	// Comparing an endpoint with itself would print a clean comparison that
	// compared nothing.
	twice := multiJob(map[string]source.Source{endpointA: agreeingSource()}, endpointA, endpointA)
	if _, err := Run(context.Background(), twice); !errors.Is(err, ErrUsage) {
		t.Fatalf("duplicate endpoint: err = %v, want ErrUsage", err)
	}
}
