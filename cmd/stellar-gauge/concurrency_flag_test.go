package main

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/source"
)

type gatedSource struct {
	started  chan struct{}
	release  chan struct{}
	mu       sync.Mutex
	inflight int
	peak     int
}

func (s *gatedSource) Network(context.Context) (model.NetworkInfo, error) {
	return model.NetworkInfo{Passphrase: testPassphrase, Ledger: 1}, nil
}

func (s *gatedSource) Account(_ context.Context, id string) (model.AccountSnapshot, error) {
	s.mu.Lock()
	s.inflight++
	if s.inflight > s.peak {
		s.peak = s.inflight
	}
	s.mu.Unlock()
	s.started <- struct{}{}
	<-s.release
	s.mu.Lock()
	s.inflight--
	s.mu.Unlock()
	return model.AccountSnapshot{
		ID: id, Sequence: "1", Ledger: 1,
		Thresholds: model.Thresholds{Low: 2, Medium: 2, High: 2},
		Signers: []model.Signer{
			{Key: id, Type: model.SignerEd25519, Weight: 1},
			{Key: "GBQHXYDEJD6NEIUQ5NYDBVNTRCACJXMLO32JUQHBGO524NMVYOMIN5CP", Type: model.SignerEd25519, Weight: 1},
		},
	}, nil
}

func (s *gatedSource) maxInFlight() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.peak
}

func TestCheckConcurrencyFlag(t *testing.T) {
	for _, tc := range []struct {
		name string
		flag []string
		want int
	}{
		{name: "explicit serial bound", flag: []string{"--concurrency", "1"}, want: 1},
		{name: "default bound", want: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const accountCount = 4
			src := &gatedSource{started: make(chan struct{}, accountCount), release: make(chan struct{}, accountCount)}
			args := []string{"check", "--network", testNetwork, "--endpoint", "https://horizon.example"}
			args = append(args, tc.flag...)
			args = append(args, accountOne, accountMissing, "GBQHXYDEJD6NEIUQ5NYDBVNTRCACJXMLO32JUQHBGO524NMVYOMIN5CP", accountOne)
			d := offlineDeps(nil)
			d.newSource = func(string) (source.Source, error) { return src, nil }
			done := make(chan int, 1)
			go func() { done <- runCLI(args, io.Discard, io.Discard, d) }()
			target := tc.want
			for i := 0; i < target; i++ {
				select {
				case <-src.started:
				case <-time.After(5 * time.Second):
					t.Fatal("account reads did not reach the requested concurrency")
				}
			}
			if tc.want == 1 {
				for i := 0; i < accountCount; i++ {
					src.release <- struct{}{}
					if i+1 < accountCount {
						select {
						case <-src.started:
						case <-time.After(5 * time.Second):
							t.Fatal("next serial account read did not start")
						}
					}
				}
			} else {
				for i := 0; i < accountCount; i++ {
					src.release <- struct{}{}
				}
			}
			select {
			case code := <-done:
				if code != int(model.ExitPass) {
					t.Fatalf("check exit = %d, want 0", code)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("check did not finish")
			}
			if peak := src.maxInFlight(); peak != tc.want {
				t.Fatalf("peak Account calls = %d, want %d", peak, tc.want)
			}
		})
	}
}

func TestCheckConcurrencyRange(t *testing.T) {
	for _, value := range []string{"0", "33", "-1", "not-a-number"} {
		t.Run(value, func(t *testing.T) {
			code, _, stderr := execCLI(t, "check", "--network", testNetwork, "--endpoint", "https://horizon.example", "--concurrency", value, accountOne)
			if code != 2 || !strings.Contains(stderr, "--concurrency must be between 1 and 32") {
				t.Fatalf("exit/stderr = %d/%q, want range usage error", code, stderr)
			}
		})
	}
}
