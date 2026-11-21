package run

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/sep1"
	"github.com/posture-sec/guage/internal/source"
)

// cancelOnNetwork answers Network successfully and cancels the run's context
// while doing so, which is the window between stages: the stage that was running
// returns no error, and only the next boundary can notice the context ended.
type cancelOnNetwork struct {
	info   model.NetworkInfo
	cancel context.CancelFunc
}

func (c *cancelOnNetwork) Network(ctx context.Context) (model.NetworkInfo, error) {
	c.cancel()
	return c.info, nil
}

func (c *cancelOnNetwork) Account(ctx context.Context, id string) (model.AccountSnapshot, error) {
	return model.AccountSnapshot{}, errors.New("no account may be read from an endpoint on the wrong network")
}

// TestRunStopsWhenContextEndsBetweenStages pins the gap a wrong-network run used
// to fall through: observe returns a synthetic result without reading anything and
// identify returns early, so neither stage looks at the context, and a cancelled
// run produced a full report. A run whose context ended has no trustworthy report.
func TestRunStopsWhenContextEndsBetweenStages(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fs := &cancelOnNetwork{
		info:   model.NetworkInfo{Passphrase: "Public Global Stellar Network ; September 2015", Ledger: 1},
		cancel: cancel,
	}
	var built atomic.Int32
	job := baseJob(nil)
	job.NewSource = func(string) (source.Source, error) { return fs, nil }
	job.NewIdentity = func() (sep1.Fetcher, error) {
		built.Add(1)
		return &fakeIdentity{}, nil
	}

	result, err := Run(ctx, job)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if !reflect.DeepEqual(result, model.Report{}) {
		t.Fatalf("a cancelled run returned a report: %+v", result)
	}
	if n := built.Load(); n != 0 {
		t.Fatalf("identity factory called %d times, want 0", n)
	}
}
