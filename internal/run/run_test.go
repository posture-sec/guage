package run

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
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
	job.Evaluate = func(account policy.Account, snap model.AccountSnapshot) []model.Finding {
		if account.ID != accountOne {
			return nil
		}
		return []model.Finding{
			model.NewFinding("sig-test-warn", model.SevWarn, snap.ID, "a warning", "evidence"),
			model.NewFinding("sig-test-high", model.SevHigh, snap.ID, "a high finding", "evidence"),
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
			job.Evaluate = func(_ policy.Account, snap model.AccountSnapshot) []model.Finding {
				if snap.ID != accountOne {
					return nil
				}
				return []model.Finding{model.NewFinding(rule, model.SevHigh, snap.ID, "message", "evidence")}
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
			job.Evaluate = func(_ policy.Account, snap model.AccountSnapshot) []model.Finding {
				if snap.ID != accountOne {
					return nil
				}
				return []model.Finding{model.NewFinding("sig-test", tt.sev, snap.ID, "message", "evidence")}
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

func TestRunRejectsWrongNetworkBeforeFetching(t *testing.T) {
	t.Parallel()
	fs := &fakeSource{info: model.NetworkInfo{Passphrase: "Public Global Stellar Network ; September 2015", Ledger: 1}}
	_, err := Run(context.Background(), baseJob(fs))
	if !errors.Is(err, source.ErrPassphraseMismatch) {
		t.Fatalf("err = %v, want ErrPassphraseMismatch", err)
	}
	if calls := fs.called(); len(calls) != 0 {
		t.Fatalf("accounts were fetched: %v", calls)
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
		{
			name: "account is missing",
			fs: &fakeSource{
				info: model.NetworkInfo{Passphrase: testnet, Ledger: 1},
			},
			want: source.ErrNotFound,
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
	job.Evaluate = func(_ policy.Account, snap model.AccountSnapshot) []model.Finding {
		if snap.ID == accountOne {
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
		Evaluate: func(account policy.Account, _ model.AccountSnapshot) []model.Finding {
			seen = account
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
		Evaluate: func(account policy.Account, snap model.AccountSnapshot) []model.Finding {
			seen = account
			return []model.Finding{model.NewFinding("sig-test", model.SevHigh, snap.ID, "message", "evidence")}
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
	job.Evaluate = func(policy.Account, model.AccountSnapshot) []model.Finding { return nil }

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
