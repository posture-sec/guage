package run

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/rules"
	"github.com/posture-sec/guage/internal/sep1"
	"github.com/posture-sec/guage/internal/source"
)

const (
	accountOne   = "GBK4NFJ7SVMUNPBIYBC4WNMCTKI6Z66MYYWZQSSLQNOSUSTMKVUKE7M3"
	accountTwo   = "GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG"
	accountThree = "GBQHXYDEJD6NEIUQ5NYDBVNTRCACJXMLO32JUQHBGO524NMVYOMIN5CP"
	testnet      = "Test SDF Network ; September 2015"
)

type fakeSource struct {
	info       model.NetworkInfo
	networkErr error
	snapshots  map[string]model.AccountSnapshot
	accountErr map[string]error
	rec        *order

	mu    sync.Mutex
	calls []string
}

func (f *fakeSource) Network(ctx context.Context) (model.NetworkInfo, error) {
	if f.networkErr != nil {
		return model.NetworkInfo{}, f.networkErr
	}
	return f.info, nil
}

func (f *fakeSource) Account(ctx context.Context, id string) (model.AccountSnapshot, error) {
	f.mu.Lock()
	f.calls = append(f.calls, id)
	f.mu.Unlock()
	f.rec.add("account:" + id)
	if err := f.accountErr[id]; err != nil {
		return model.AccountSnapshot{}, err
	}
	snapshot, ok := f.snapshots[id]
	if !ok {
		return model.AccountSnapshot{}, source.ErrNotFound
	}
	return snapshot, nil
}

func (f *fakeSource) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// order records the sequence a run's stages touched, so a test can assert that
// identity follows observation even though the fetches themselves are concurrent.
type order struct {
	mu     sync.Mutex
	events []string
}

func (o *order) add(event string) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, event)
}

func (o *order) list() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.events...)
}

// fakeIdentity is a layer-3 fake: it proves orchestration only (which domains were
// asked about, when, and how many at a time) and says nothing about the guard,
// which internal/sep1 tests against a real HTTP stack.
type fakeIdentity struct {
	results map[string]sep1.Result
	rec     *order

	mu    sync.Mutex
	calls []string
}

func (f *fakeIdentity) Fetch(ctx context.Context, domain string) sep1.Result {
	f.mu.Lock()
	f.calls = append(f.calls, domain)
	f.mu.Unlock()
	f.rec.add("identity:" + domain)
	if result, ok := f.results[domain]; ok {
		return result
	}
	return sep1.Result{HomeDomain: domain, FailedAt: sep1.StepTransport, Detail: "the fake holds no document"}
}

func (f *fakeIdentity) fetched() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func identityStub(fetcher sep1.Fetcher) IdentityFactory {
	return func() (sep1.Fetcher, error) { return fetcher, nil }
}

func withDomain(s model.AccountSnapshot, domain string) model.AccountSnapshot {
	s.HomeDomain = domain
	return s
}

func reachable(domain string) sep1.Result {
	return sep1.Result{HomeDomain: domain, FailedAt: sep1.StepOK, Bytes: 64}
}

func blocked(domain string) sep1.Result {
	return sep1.Result{
		HomeDomain: domain,
		FailedAt:   sep1.StepGuard,
		Detail:     "the host name resolves to a loopback address, which Gauge will not dial",
	}
}

func stubFactory(fs source.Source) SourceFactory {
	return func(string) (source.Source, error) { return fs, nil }
}

func baseJob(fs source.Source) Job {
	return Job{
		ToolVersion: "test",
		Network:     testnet,
		Endpoints:   []string{"https://horizon-testnet.example"},
		Accounts: []policy.Account{
			{ID: accountOne, Role: model.RoleOther},
			{ID: accountTwo, Role: model.RoleOther},
		},
		FailOn:    model.SevHigh,
		NewSource: stubFactory(fs),
	}
}

func snapshot(id string, ledger int64) model.AccountSnapshot {
	return model.AccountSnapshot{
		ID:       id,
		Sequence: "1",
		Ledger:   ledger,
		Signers:  []model.Signer{{Key: id, Type: model.SignerEd25519, Weight: 1}},
	}
}

func TestRunEndToEnd(t *testing.T) {
	t.Parallel()
	fs := &fakeSource{
		info: model.NetworkInfo{Passphrase: testnet, Ledger: 123},
		snapshots: map[string]model.AccountSnapshot{
			accountOne: snapshot(accountOne, 10),
			accountTwo: snapshot(accountTwo, 11),
		},
	}
	job := baseJob(fs)
	job.Evaluate = func(c rules.Context) []model.Finding {
		if c.Account.ID != accountOne {
			return nil
		}
		return []model.Finding{
			model.NewFinding("sig-test-warn", model.SevWarn, c.Snapshot.ID, "a warning", "evidence"),
			model.NewFinding("sig-test-high", model.SevHigh, c.Snapshot.ID, "a high finding", "evidence"),
		}
	}

	result, err := Run(context.Background(), job)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ToolVersion != "test" || result.Network != testnet || result.Ledger != 123 {
		t.Fatalf("report header = %q/%q/%d", result.ToolVersion, result.Network, result.Ledger)
	}
	if len(result.Accounts) != 2 {
		t.Fatalf("accounts = %d, want 2", len(result.Accounts))
	}
	if result.Accounts[0].Snapshot.ID != accountOne || result.Accounts[1].Snapshot.ID != accountTwo {
		t.Fatalf("account order = %q, %q", result.Accounts[0].Snapshot.ID, result.Accounts[1].Snapshot.ID)
	}
	for _, account := range result.Accounts {
		if account.Snapshot.Network != testnet {
			t.Fatalf("snapshot network = %q", account.Snapshot.Network)
		}
	}
	findings := result.Accounts[0].Findings
	if len(findings) != 2 || findings[0].RuleID != "sig-test-high" || findings[1].RuleID != "sig-test-warn" {
		t.Fatalf("findings = %+v", findings)
	}
	if findings[0].Fingerprint == "" {
		t.Fatal("finding has no fingerprint")
	}
	if result.ExitCode != model.ExitFindings {
		t.Fatalf("exit = %d, want %d", result.ExitCode, model.ExitFindings)
	}
}

func TestRunAccepts(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	future := "2027-01-01"
	past := "2026-01-01"

	cases := []struct {
		name       string
		accepts    []policy.Acceptance
		wantAccept bool
		wantExit   model.ExitCode
	}{
		{
			name:       "live acceptance covers the finding",
			accepts:    []policy.Acceptance{{Rule: "sig-test", Account: accountOne, Reason: "known", Expires: &future}},
			wantAccept: true,
			wantExit:   model.ExitPass,
		},
		{
			name:       "expired acceptance does not cover the finding",
			accepts:    []policy.Acceptance{{Rule: "sig-test", Account: accountOne, Reason: "known", Expires: &past}},
			wantAccept: false,
			wantExit:   model.ExitFindings,
		},
		{
			name:       "acceptance for another account does not apply",
			accepts:    []policy.Acceptance{{Rule: "sig-test", Account: accountTwo, Reason: "known"}},
			wantAccept: false,
			wantExit:   model.ExitFindings,
		},
		{
			name:       "non-acceptable rule cannot be silenced",
			accepts:    []policy.Acceptance{{Rule: "sig-threshold-unreachable", Account: accountOne, Reason: "known"}},
			wantAccept: false,
			wantExit:   model.ExitFindings,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := &fakeSource{
				info:      model.NetworkInfo{Passphrase: testnet, Ledger: 1},
				snapshots: map[string]model.AccountSnapshot{accountOne: snapshot(accountOne, 1), accountTwo: snapshot(accountTwo, 2)},
			}
			job := baseJob(fs)
			job.Accepts = tt.accepts
			job.Now = func() time.Time { return now }
			rule := "sig-test"
			if len(tt.accepts) == 1 && tt.accepts[0].Rule == "sig-threshold-unreachable" {
				rule = tt.accepts[0].Rule
			}
			job.Evaluate = func(c rules.Context) []model.Finding {
				if c.Snapshot.ID != accountOne {
					return nil
				}
				return []model.Finding{model.NewFinding(rule, model.SevHigh, c.Snapshot.ID, "message", "evidence")}
			}

			result, err := Run(context.Background(), job)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			got := result.Accounts[0].Findings[0].Accepted
			if got != tt.wantAccept {
				t.Fatalf("accepted = %v, want %v", got, tt.wantAccept)
			}
			if result.ExitCode != tt.wantExit {
				t.Fatalf("exit = %d, want %d", result.ExitCode, tt.wantExit)
			}
		})
	}
}

func TestRunFailOn(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		failOn model.Severity
		sev    model.Severity
		want   model.ExitCode
	}{
		{"warn below a high threshold passes", model.SevHigh, model.SevWarn, model.ExitPass},
		{"warn at a warn threshold fails", model.SevWarn, model.SevWarn, model.ExitFindings},
		{"info at an info threshold fails", model.SevInfo, model.SevInfo, model.ExitFindings},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := &fakeSource{
				info:      model.NetworkInfo{Passphrase: testnet, Ledger: 1},
				snapshots: map[string]model.AccountSnapshot{accountOne: snapshot(accountOne, 1), accountTwo: snapshot(accountTwo, 2)},
			}
			job := baseJob(fs)
			job.FailOn = tt.failOn
			job.Evaluate = func(c rules.Context) []model.Finding {
				if c.Snapshot.ID != accountOne {
					return nil
				}
				return []model.Finding{model.NewFinding("sig-test", tt.sev, c.Snapshot.ID, "message", "evidence")}
			}
			result, err := Run(context.Background(), job)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if result.ExitCode != tt.want {
				t.Fatalf("exit = %d, want %d", result.ExitCode, tt.want)
			}
		})
	}
}

func TestRunReportsWrongNetworkWithoutFetching(t *testing.T) {
	t.Parallel()
	fs := &fakeSource{info: model.NetworkInfo{Passphrase: "Public Global Stellar Network ; September 2015", Ledger: 1}}
	job := baseJob(fs)
	job.Evaluate = rules.Evaluate
	result, err := Run(context.Background(), job)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if calls := fs.called(); len(calls) != 0 {
		t.Fatalf("accounts were fetched from the wrong network: %v", calls)
	}
	if result.ExitCode != model.ExitFindings {
		t.Fatalf("exit = %d, want %d", result.ExitCode, model.ExitFindings)
	}
	if len(result.Accounts) != 2 {
		t.Fatalf("accounts = %d, want 2", len(result.Accounts))
	}
	for _, account := range result.Accounts {
		if account.Skipped == "" {
			t.Fatalf("account %s was not marked skipped", account.Snapshot.ID)
		}
		if len(account.Findings) != 1 || account.Findings[0].RuleID != "acct-network-mismatch" || account.Findings[0].Severity != model.SevHigh {
			t.Fatalf("findings = %+v", account.Findings)
		}
	}
}

func TestRunTreatsAMissingAccountAsAFinding(t *testing.T) {
	t.Parallel()
	fs := &fakeSource{
		info:      model.NetworkInfo{Passphrase: testnet, Ledger: 1},
		snapshots: map[string]model.AccountSnapshot{accountTwo: snapshot(accountTwo, 2)},
	}
	job := baseJob(fs)
	job.Evaluate = rules.Evaluate
	result, err := Run(context.Background(), job)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	missing := result.Accounts[0]
	if !missing.Missing {
		t.Fatal("the missing account was not marked missing")
	}
	if missing.Snapshot.ID != accountOne || missing.Snapshot.Endpoint != job.Endpoints[0] {
		t.Fatalf("missing snapshot = %+v", missing.Snapshot)
	}
	if len(missing.Findings) != 1 || missing.Findings[0].RuleID != "acct-missing" || missing.Findings[0].Severity != model.SevHigh {
		t.Fatalf("findings = %+v", missing.Findings)
	}
	if result.Accounts[1].Missing || len(result.Accounts[1].Findings) != 0 {
		t.Fatalf("the readable account was affected: %+v", result.Accounts[1])
	}
	if result.ExitCode != model.ExitFindings {
		t.Fatalf("exit = %d, want %d", result.ExitCode, model.ExitFindings)
	}
}

func TestRunReturnsSourceErrors(t *testing.T) {
	t.Parallel()
	networkBoom := errors.New("boom")
	cases := []struct {
		name string
		fs   *fakeSource
		want error
	}{
		{
			name: "network read fails",
			fs:   &fakeSource{networkErr: networkBoom},
			want: networkBoom,
		},
		{
			name: "account read fails",
			fs: &fakeSource{
				info:       model.NetworkInfo{Passphrase: testnet, Ledger: 1},
				snapshots:  map[string]model.AccountSnapshot{accountTwo: snapshot(accountTwo, 2)},
				accountErr: map[string]error{accountOne: source.ErrStatus},
			},
			want: source.ErrStatus,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Run(context.Background(), baseJob(tt.fs))
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRunRecoversRulePanics(t *testing.T) {
	t.Parallel()
	fs := &fakeSource{
		info:      model.NetworkInfo{Passphrase: testnet, Ledger: 1},
		snapshots: map[string]model.AccountSnapshot{accountOne: snapshot(accountOne, 1), accountTwo: snapshot(accountTwo, 2)},
	}
	job := baseJob(fs)
	job.Evaluate = func(c rules.Context) []model.Finding {
		if c.Snapshot.ID == accountOne {
			panic("a rule broke")
		}
		return nil
	}
	result, err := Run(context.Background(), job)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != model.ExitSource {
		t.Fatalf("exit = %d, want %d", result.ExitCode, model.ExitSource)
	}
	if len(result.Accounts) != 2 {
		t.Fatalf("accounts = %d, want 2", len(result.Accounts))
	}
	findings := result.Accounts[0].Findings
	if len(findings) != 1 || findings[0].RuleID != "src-rule-panic" || findings[0].Severity != model.SevHigh {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestRunRejectsInvalidJobs(t *testing.T) {
	t.Parallel()
	valid := func() Job {
		return baseJob(&fakeSource{info: model.NetworkInfo{Passphrase: testnet, Ledger: 1}})
	}
	cases := []struct {
		name string
		edit func(*Job)
	}{
		{"no network", func(j *Job) { j.Network = "" }},
		{"no endpoint", func(j *Job) { j.Endpoints = nil }},
		{"two endpoints", func(j *Job) { j.Endpoints = append(j.Endpoints, "https://second.example") }},
		{"no accounts", func(j *Job) { j.Accounts = nil }},
		{"negative concurrency", func(j *Job) { j.Concurrency = -1 }},
		{"unknown failure threshold", func(j *Job) { j.FailOn = model.Severity(9) }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			job := valid()
			tt.edit(&job)
			if _, err := Run(context.Background(), job); !errors.Is(err, ErrUsage) {
				t.Fatalf("err = %v, want ErrUsage", err)
			}
		})
	}
}

func TestCheckBuildsAccountsWithoutExpectations(t *testing.T) {
	t.Parallel()
	fs := &fakeSource{
		info:      model.NetworkInfo{Passphrase: testnet, Ledger: 1},
		snapshots: map[string]model.AccountSnapshot{accountOne: snapshot(accountOne, 1)},
	}
	var seen policy.Account
	_, err := Check(context.Background(), CheckRequest{
		ToolVersion: "test",
		Network:     "testnet",
		Endpoints:   []string{"https://horizon-testnet.example"},
		Accounts:    []string{accountOne},
		FailOn:      model.SevHigh,
		NewSource:   stubFactory(fs),
		Evaluate: func(c rules.Context) []model.Finding {
			seen = c.Account
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if seen.ID != accountOne || seen.Role != model.RoleOther {
		t.Fatalf("account = %+v", seen)
	}
	if seen.ExpectedSigners != nil || seen.ExpectedThresholds != nil || seen.ExpectedFlags != nil {
		t.Fatalf("check passed expectations: %+v", seen)
	}
}

func TestCheckRejectsNonAccountIDs(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"", "GBK4NFJ7SVMUNPBIYBC4WNMCTKI6Z66MYYWZQSSLQNOSUSTMKVUKE7M2", "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUQ"} {
		_, err := Check(context.Background(), CheckRequest{
			Network:   "testnet",
			Endpoints: []string{"https://horizon-testnet.example"},
			Accounts:  []string{id},
		})
		if !errors.Is(err, ErrUsage) {
			t.Fatalf("id %q: err = %v, want ErrUsage", id, err)
		}
	}
}

func TestAuditUsesPolicyNetworkAndAccepts(t *testing.T) {
	t.Parallel()
	fs := &fakeSource{
		info:      model.NetworkInfo{Passphrase: testnet, Ledger: 1},
		snapshots: map[string]model.AccountSnapshot{accountOne: snapshot(accountOne, 1)},
	}
	weight := 2
	expiry := "2027-01-01"
	cfg := &policy.Config{
		Version:   1,
		Network:   "testnet",
		Endpoints: []string{"https://horizon-testnet.example"},
		Accounts: []policy.Account{{
			ID:              accountOne,
			Role:            model.RoleTreasury,
			ExpectedSigners: &[]policy.ExpectedSigner{{Key: accountOne, Weight: weight}},
		}},
		Accepts: []policy.Acceptance{{Rule: "sig-test", Account: accountOne, Reason: "known", Expires: &expiry}},
	}
	var seen policy.Account
	result, err := Audit(context.Background(), cfg, AuditOptions{
		ToolVersion: "test",
		FailOn:      model.SevHigh,
		NewSource:   stubFactory(fs),
		Now:         func() time.Time { return time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC) },
		Evaluate: func(c rules.Context) []model.Finding {
			seen = c.Account
			return []model.Finding{model.NewFinding("sig-test", model.SevHigh, c.Snapshot.ID, "message", "evidence")}
		},
	})
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if seen.Role != model.RoleTreasury || seen.ExpectedSigners == nil {
		t.Fatalf("audit lost the policy account: %+v", seen)
	}
	if !result.Accounts[0].Findings[0].Accepted || result.ExitCode != model.ExitPass {
		t.Fatalf("acceptance was not applied: %+v", result.Accounts[0].Findings)
	}
}

func TestAuditRejectsNilPolicy(t *testing.T) {
	t.Parallel()
	if _, err := Audit(context.Background(), nil, AuditOptions{}); !errors.Is(err, ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestFetchConcurrencyIsBounded(t *testing.T) {
	t.Parallel()
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	var inflight, peak int32
	fs := &blockingSource{started: started, release: release, inflight: &inflight, peak: &peak}
	job := baseJob(fs)
	job.Concurrency = 1
	job.Evaluate = func(rules.Context) []model.Finding { return nil }

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

type blockingSource struct {
	started  chan struct{}
	release  chan struct{}
	inflight *int32
	peak     *int32
}

func (b *blockingSource) Network(ctx context.Context) (model.NetworkInfo, error) {
	return model.NetworkInfo{Passphrase: testnet, Ledger: 1}, nil
}

func (b *blockingSource) Account(ctx context.Context, id string) (model.AccountSnapshot, error) {
	current := atomic.AddInt32(b.inflight, 1)
	for {
		seen := atomic.LoadInt32(b.peak)
		if current <= seen || atomic.CompareAndSwapInt32(b.peak, seen, current) {
			break
		}
	}
	b.started <- struct{}{}
	<-b.release
	atomic.AddInt32(b.inflight, -1)
	return snapshot(id, 1), nil
}

func TestRunFetchesIdentityAfterTheAccounts(t *testing.T) {
	t.Parallel()
	rec := &order{}
	fs := &fakeSource{
		info: model.NetworkInfo{Passphrase: testnet, Ledger: 1},
		snapshots: map[string]model.AccountSnapshot{
			accountOne: withDomain(snapshot(accountOne, 1), "example.org"),
			accountTwo: snapshot(accountTwo, 2),
		},
		rec: rec,
	}
	fetches := &fakeIdentity{
		rec:     rec,
		results: map[string]sep1.Result{"example.org": reachable("example.org")},
	}
	job := baseJob(fs)
	job.NewIdentity = identityStub(fetches)
	seen := map[string]*sep1.Result{}
	job.Evaluate = func(c rules.Context) []model.Finding {
		seen[c.Snapshot.ID] = c.Identity
		return nil
	}

	if _, err := Run(context.Background(), job); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := fetches.fetched(); len(got) != 1 || got[0] != "example.org" {
		t.Fatalf("identity fetches = %v, want only the observed home domain", got)
	}
	// The account reads are concurrent, so the promise is that identity comes
	// last, not that the accounts arrive in any order.
	events := rec.list()
	last := len(events) - 1
	if last < 2 || !strings.HasPrefix(events[last], "identity:") {
		t.Fatalf("events = %v, want the identity fetch after every account read", events)
	}
	for _, event := range events[:last] {
		if !strings.HasPrefix(event, "account:") {
			t.Fatalf("events = %v, want the account reads first", events)
		}
	}
	if seen[accountOne] == nil || !seen[accountOne].Reachable() {
		t.Fatalf("account one identity = %v, want the fetched result", seen[accountOne])
	}
	if seen[accountTwo] != nil {
		t.Fatalf("an account publishing no home domain was handed a result: %+v", seen[accountTwo])
	}
}

func TestRunUnreachableTomlIsAFindingNotAnAbort(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		failOn   model.Severity
		wantExit model.ExitCode
	}{
		{"a warn below a high threshold passes", model.SevHigh, model.ExitPass},
		{"a warn at a warn threshold fails", model.SevWarn, model.ExitFindings},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := &fakeSource{
				info: model.NetworkInfo{Passphrase: testnet, Ledger: 1},
				snapshots: map[string]model.AccountSnapshot{
					accountOne: withDomain(snapshot(accountOne, 1), "example.org"),
					accountTwo: snapshot(accountTwo, 2),
				},
			}
			job := baseJob(fs)
			job.FailOn = tt.failOn
			job.NewIdentity = identityStub(&fakeIdentity{
				results: map[string]sep1.Result{"example.org": blocked("example.org")},
			})
			job.Evaluate = rules.Evaluate
			result, err := Run(context.Background(), job)
			if err != nil {
				t.Fatalf("a blocked identity target must not abort the run: %v", err)
			}
			findings := result.Accounts[0].Findings
			if len(findings) != 1 || findings[0].RuleID != "id-toml-unreachable" || findings[0].Severity != model.SevWarn {
				t.Fatalf("findings = %+v, want one id-toml-unreachable warning", findings)
			}
			if !strings.Contains(findings[0].Evidence, "step guard") {
				t.Fatalf("evidence = %q, want the failed step named", findings[0].Evidence)
			}
			if len(result.Accounts[1].Findings) != 0 {
				t.Fatalf("the account with no home domain raised %+v", result.Accounts[1].Findings)
			}
			if result.ExitCode != tt.wantExit {
				t.Fatalf("exit = %d, want %d", result.ExitCode, tt.wantExit)
			}
		})
	}
}

func TestRunSkipsIdentityOnANetworkMismatch(t *testing.T) {
	t.Parallel()
	fs := &fakeSource{info: model.NetworkInfo{Passphrase: "Public Global Stellar Network ; September 2015", Ledger: 1}}
	job := baseJob(fs)
	// Refusing to build the fetcher proves the whole stage is skipped rather than
	// merely producing no results: nothing is set up against a wrong-network endpoint.
	job.NewIdentity = func() (sep1.Fetcher, error) {
		return nil, errors.New("the identity fetcher must not be built")
	}
	job.Evaluate = rules.Evaluate
	result, err := Run(context.Background(), job)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, account := range result.Accounts {
		if len(account.Findings) != 1 || account.Findings[0].RuleID != "acct-network-mismatch" {
			t.Fatalf("account %s findings = %+v", account.Snapshot.ID, account.Findings)
		}
	}
}

func TestRunAsksIdentityOnlyAboutObservedDomains(t *testing.T) {
	t.Parallel()
	fs := &fakeSource{
		info:      model.NetworkInfo{Passphrase: testnet, Ledger: 1},
		snapshots: map[string]model.AccountSnapshot{accountTwo: snapshot(accountTwo, 2)},
	}
	fetches := &fakeIdentity{}
	job := baseJob(fs)
	job.NewIdentity = identityStub(fetches)
	job.Evaluate = rules.Evaluate
	result, err := Run(context.Background(), job)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := fetches.fetched(); len(got) != 0 {
		t.Fatalf("identity fetches = %v, want none for a missing account and an account with no home domain", got)
	}
	missing := result.Accounts[0]
	if len(missing.Findings) != 1 || missing.Findings[0].RuleID != "acct-missing" {
		t.Fatalf("the missing account raised %+v, want only acct-missing", missing.Findings)
	}
	if len(result.Accounts[1].Findings) != 0 {
		t.Fatalf("findings = %+v", result.Accounts[1].Findings)
	}
}

// The orchestration half of the same rule: a home domain that is only whitespace,
// which is what one corpus capture actually carries, is an account that publishes
// nothing. No fetch is attempted for it, so no identity finding can be raised about
// it. A padded but real domain is the opposite case and is fetched.
func TestRunTreatsABlankObservedDomainAsAbsent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		domain    string
		wantFetch bool
	}{
		{"single space", " ", false},
		{"tab", "\t", false},
		{"padded real domain", " example.org ", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fs := &fakeSource{
				info: model.NetworkInfo{Passphrase: testnet, Ledger: 1},
				snapshots: map[string]model.AccountSnapshot{
					accountOne: withDomain(snapshot(accountOne, 10), tc.domain),
				},
			}
			fetches := &fakeIdentity{results: map[string]sep1.Result{"example.org": reachable("example.org")}}
			job := baseJob(fs)
			job.Accounts = []policy.Account{{ID: accountOne, Role: model.RoleOther}}
			job.NewIdentity = identityStub(fetches)
			job.Evaluate = rules.Evaluate
			result, err := Run(context.Background(), job)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			attempted := len(fetches.fetched()) > 0
			if attempted != tc.wantFetch {
				t.Fatalf("identity fetches = %q, want attempted = %t", fetches.fetched(), tc.wantFetch)
			}
			if !tc.wantFetch {
				for _, account := range result.Accounts {
					if len(account.Findings) != 0 {
						t.Fatalf("findings for %s = %+v, want none", account.Snapshot.ID, account.Findings)
					}
				}
			}
		})
	}
}

func TestIdentityConcurrencyIsBounded(t *testing.T) {
	t.Parallel()
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var inflight, peak int32
	fs := &fakeSource{
		info: model.NetworkInfo{Passphrase: testnet, Ledger: 1},
		snapshots: map[string]model.AccountSnapshot{
			accountOne: withDomain(snapshot(accountOne, 1), "example.org"),
			accountTwo: withDomain(snapshot(accountTwo, 2), "example.com"),
		},
	}
	job := baseJob(fs)
	job.Concurrency = 1
	job.NewIdentity = identityStub(&blockingIdentity{started: started, release: release, inflight: &inflight, peak: &peak})
	job.Evaluate = func(rules.Context) []model.Finding { return nil }

	done := make(chan error, 1)
	go func() {
		_, err := Run(context.Background(), job)
		done <- err
	}()

	<-started
	select {
	case <-started:
		t.Fatal("a second identity fetch started while the first was in flight")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := atomic.LoadInt32(&peak); got != 1 {
		t.Fatalf("peak identity concurrency = %d, want 1", got)
	}
}

type blockingIdentity struct {
	started  chan struct{}
	release  chan struct{}
	inflight *int32
	peak     *int32
}

func (b *blockingIdentity) Fetch(ctx context.Context, domain string) sep1.Result {
	current := atomic.AddInt32(b.inflight, 1)
	for {
		seen := atomic.LoadInt32(b.peak)
		if current <= seen || atomic.CompareAndSwapInt32(b.peak, seen, current) {
			break
		}
	}
	b.started <- struct{}{}
	<-b.release
	atomic.AddInt32(b.inflight, -1)
	return sep1.Result{HomeDomain: domain, FailedAt: sep1.StepOK}
}

func TestDefaultIdentityIsTheGuardedClient(t *testing.T) {
	t.Parallel()
	// NewClient only assembles a transport: no name is resolved and no socket is
	// opened until Fetch runs, which no test lets happen.
	fetcher, err := defaultIdentity("test")()
	if err != nil {
		t.Fatalf("defaultIdentity: %v", err)
	}
	if _, ok := fetcher.(*sep1.Client); !ok {
		t.Fatalf("default fetcher = %T, want the guarded client rather than a skip", fetcher)
	}
}

func TestRunIdentityFactoryFailureAborts(t *testing.T) {
	t.Parallel()
	boom := errors.New("no fetcher")
	fs := &fakeSource{
		info:      model.NetworkInfo{Passphrase: testnet, Ledger: 1},
		snapshots: map[string]model.AccountSnapshot{accountOne: withDomain(snapshot(accountOne, 1), "example.org")},
	}
	job := baseJob(fs)
	job.NewIdentity = func() (sep1.Fetcher, error) { return nil, boom }
	if _, err := Run(context.Background(), job); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the factory error", err)
	}
}

func TestCheckAndAuditForwardTheIdentityFetcher(t *testing.T) {
	t.Parallel()
	domainSource := func() source.Source {
		return &fakeSource{
			info:      model.NetworkInfo{Passphrase: testnet, Ledger: 1},
			snapshots: map[string]model.AccountSnapshot{accountOne: withDomain(snapshot(accountOne, 1), "example.org")},
		}
	}
	cases := []struct {
		name string
		run  func(IdentityFactory, Evaluator) error
	}{
		{
			name: "check",
			run: func(newIdentity IdentityFactory, evaluate Evaluator) error {
				_, err := Check(context.Background(), CheckRequest{
					ToolVersion: "test",
					Network:     "testnet",
					Endpoints:   []string{"https://horizon-testnet.example"},
					Accounts:    []string{accountOne},
					FailOn:      model.SevHigh,
					NewSource:   stubFactory(domainSource()),
					NewIdentity: newIdentity,
					Evaluate:    evaluate,
				})
				return err
			},
		},
		{
			name: "audit",
			run: func(newIdentity IdentityFactory, evaluate Evaluator) error {
				cfg := &policy.Config{
					Version:   1,
					Network:   "testnet",
					Endpoints: []string{"https://horizon-testnet.example"},
					Accounts:  []policy.Account{{ID: accountOne, Role: model.RoleTreasury}},
				}
				_, err := Audit(context.Background(), cfg, AuditOptions{
					ToolVersion: "test",
					FailOn:      model.SevHigh,
					NewSource:   stubFactory(domainSource()),
					NewIdentity: newIdentity,
					Evaluate:    evaluate,
					Now:         func() time.Time { return time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC) },
				})
				return err
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var seen *sep1.Result
			fetcher := identityStub(&fakeIdentity{
				results: map[string]sep1.Result{"example.org": reachable("example.org")},
			})
			err := tt.run(fetcher, func(c rules.Context) []model.Finding {
				seen = c.Identity
				return nil
			})
			if err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if seen == nil || seen.HomeDomain != "example.org" {
				t.Fatalf("%s identity = %v, want the fetched result", tt.name, seen)
			}
		})
	}
}
