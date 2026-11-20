package source

import (
	"context"
	"errors"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrencyBoundedTwelveCallers(t *testing.T) {
	// Settle before baseline
	time.Sleep(10 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	started := make(chan struct{}, 12)
	release := make(chan struct{})
	var inflight, peak int32

	account := accountMap(testAccount)
	body := mustJSON(t, account)

	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&inflight, 1)
		for {
			seen := atomic.LoadInt32(&peak)
			if current <= seen || atomic.CompareAndSwapInt32(&peak, seen, current) {
				break
			}
		}
		started <- struct{}{}
		<-release
		atomic.AddInt32(&inflight, -1)
		serve(w, http.StatusOK, body)
	})

	h := clientFor(t, srv, Options{MaxConcurrent: 2})

	const numCallers = 12
	var wg sync.WaitGroup
	errs := make(chan error, numCallers)

	for i := 0; i < numCallers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := h.Account(context.Background(), testAccount)
			errs <- err
		}()
	}

	// Exactly 2 callers should enter in-flight
	<-started
	<-started

	// Verify a 3rd caller cannot start while MaxConcurrent=2 is saturated
	select {
	case <-started:
		t.Fatal("a third request started while MaxConcurrent=2 was saturated")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("unexpected caller error: %v", err)
		}
	}

	if got := atomic.LoadInt32(&peak); got != 2 {
		t.Fatalf("peak concurrency = %d, want exactly 2", got)
	}

	srv.Close()
	time.Sleep(30 * time.Millisecond)
	if final := runtime.NumGoroutine(); final > initialGoroutines+2 {
		t.Fatalf("goroutine leak: before=%d, after=%d", initialGoroutines, final)
	}
}

func TestCancelDuringRetry(t *testing.T) {
	time.Sleep(10 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	var attempts int32
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	h := clientFor(t, srv, Options{MaxRetries: 3})

	sleepEntered := make(chan struct{}, 1)
	sleepDone := make(chan struct{}, 1)

	h.sleep = func(ctx context.Context, d time.Duration) error {
		select {
		case sleepEntered <- struct{}{}:
		default:
		}
		defer func() {
			select {
			case sleepDone <- struct{}{}:
			default:
			}
		}()
		return sleepContext(ctx, d)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resCh := make(chan error, 1)
	go func() {
		_, err := h.Account(ctx, testAccount)
		resCh <- err
	}()

	// Wait until sleep is entered during 429 retry backoff
	select {
	case <-sleepEntered:
	case <-time.After(time.Second):
		t.Fatal("did not enter sleep during retry")
	}

	// Cancel context while waiting in sleep
	cancel()

	// Assert the call returns immediately
	select {
	case err := <-resCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("call did not return promptly upon context cancellation")
	}

	// Assert sleep terminated and did not outlive the cancelled call
	select {
	case <-sleepDone:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("sleep outlived the cancelled request")
	}

	// Assert attempts stopped and didn't exhaust full maxRetries
	if got := atomic.LoadInt32(&attempts); got > 2 {
		t.Fatalf("attempts = %d, wanted retries to stop on cancellation", got)
	}

	srv.Close()
	time.Sleep(30 * time.Millisecond)
	if final := runtime.NumGoroutine(); final > initialGoroutines+2 {
		t.Fatalf("goroutine leak after cancellation: before=%d, after=%d", initialGoroutines, final)
	}
}
