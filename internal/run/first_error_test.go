package run

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
)

// orderedFailures makes two concurrent reads fail in a fixed order without a
// timer. B parks inside Account and announces it; A fails only once B is
// parked, so both semaphore slots are held and C is queued behind them; B is
// released by the run cancelling its context, which fetchAll does only after it
// has recorded A's error. C must never be read: once the first failure is
// recorded the run is over.
type orderedFailures struct {
	a, b, c    string
	errA, errB error
	bParked    chan struct{}
	cCalls     atomic.Int32
}

func (o *orderedFailures) Network(ctx context.Context) (model.NetworkInfo, error) {
	return model.NetworkInfo{Passphrase: testnet, Ledger: 1}, nil
}

func (o *orderedFailures) Account(ctx context.Context, id string) (model.AccountSnapshot, error) {
	switch id {
	case o.a:
		select {
		case <-o.bParked:
		case <-ctx.Done():
			return model.AccountSnapshot{}, errors.New("A was cancelled before it could fail first")
		}
		return model.AccountSnapshot{}, o.errA
	case o.b:
		close(o.bParked)
		// fetchAll cancels this context inside the same once.Do that stores the
		// first error, so reaching here means A's error is already recorded.
		<-ctx.Done()
		return model.AccountSnapshot{}, o.errB
	default:
		o.cCalls.Add(1)
		return model.AccountSnapshot{}, errors.New("C was read after the run had failed")
	}
}

func TestFirstAccountErrorSurvives(t *testing.T) {
	t.Parallel()
	errA := errors.New("account A failed")
	errB := errors.New("account B failed")
	fs := &orderedFailures{
		a: accountOne, b: accountTwo, c: accountThree,
		errA: errA, errB: errB,
		bParked: make(chan struct{}),
	}
	job := baseJob(fs)
	job.Concurrency = 2
	job.Accounts = []policy.Account{
		{ID: accountOne, Role: model.RoleOther},
		{ID: accountTwo, Role: model.RoleOther},
		{ID: accountThree, Role: model.RoleOther},
	}

	_, err := Run(context.Background(), job)
	if !errors.Is(err, errA) {
		t.Fatalf("err = %v, want it to wrap A's error", err)
	}
	if errors.Is(err, errB) {
		t.Fatalf("err = %v, the later failure replaced the first", err)
	}
	if !strings.Contains(err.Error(), accountOne) {
		t.Fatalf("err = %q, want it to name account A (%s)", err, accountOne)
	}
	if n := fs.cCalls.Load(); n != 0 {
		t.Fatalf("account C was read %d times after the first failure", n)
	}
}
