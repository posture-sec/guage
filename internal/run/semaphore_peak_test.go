package run

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/rules"
)

type peakSource struct {
	started  chan string
	release  chan struct{}
	inflight int32
	peak     int32
}

func (p *peakSource) Network(ctx context.Context) (model.NetworkInfo, error) {
	return model.NetworkInfo{Passphrase: testnet, Ledger: 1}, nil
}

func (p *peakSource) Account(ctx context.Context, id string) (model.AccountSnapshot, error) {
	current := atomic.AddInt32(&p.inflight, 1)
	for {
		seen := atomic.LoadInt32(&p.peak)
		if current <= seen || atomic.CompareAndSwapInt32(&p.peak, seen, current) {
			break
		}
	}
	p.started <- id
	<-p.release
	atomic.AddInt32(&p.inflight, -1)
	return snapshot(id, int64(len(id))), nil
}

func TestAccountReadsPeakAtTheLimit(t *testing.T) {
	t.Parallel()
	ids := []string{accountOne, accountTwo, accountThree, "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF"}
	started := make(chan string, len(ids))
	src := &peakSource{started: started, release: make(chan struct{})}
	accounts := make([]policy.Account, len(ids))
	for i, id := range ids {
		accounts[i] = policy.Account{ID: id, Role: model.RoleOther}
	}
	job := baseJob(src)
	job.Accounts = accounts
	job.Concurrency = 2
	job.Evaluate = func(rules.Context) []model.Finding { return nil }

	done := make(chan model.Report, 1)
	failed := make(chan error, 1)
	go func() {
		result, err := Run(context.Background(), job)
		if err != nil {
			failed <- err
			return
		}
		done <- result
	}()

	seen := make(map[string]bool, 2)
	for i := 0; i < 2; i++ {
		select {
		case id := <-started:
			seen[id] = true
		case <-time.After(2 * time.Second):
			t.Fatal("fewer than two account reads started")
		}
	}
	select {
	case id := <-started:
		t.Fatalf("a third read started (%s) while two were in flight", id)
	case <-time.After(50 * time.Millisecond):
	}
	close(src.release)

	var result model.Report
	select {
	case err := <-failed:
		t.Fatalf("Run: %v", err)
	case result = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not finish")
	}
	if got := atomic.LoadInt32(&src.peak); got != 2 {
		t.Fatalf("peak concurrency = %d, want 2", got)
	}
	if len(result.Accounts) != len(ids) {
		t.Fatalf("accounts = %d, want %d", len(result.Accounts), len(ids))
	}
	for i, account := range result.Accounts {
		if account.Snapshot.ID != ids[i] {
			t.Fatalf("account %d = %q, want %q", i, account.Snapshot.ID, ids[i])
		}
		if account.Missing {
			t.Fatalf("account %d was not observed", i)
		}
	}
}
