package source

import (
	"context"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/posture-sec/guage/internal/model"
)

const (
	testAccount  = "GBK4NFJ7SVMUNPBIYBC4WNMCTKI6Z66MYYWZQSSLQNOSUSTMKVUKE7M3"
	otherAccount = "GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG"
)

func TestNewHorizonEndpoint(t *testing.T) {
	t.Parallel()
	accepted := []struct{ raw, want string }{
		{"https://horizon-testnet.stellar.org/", "https://horizon-testnet.stellar.org"},
		{"https://horizon.example.com:8443", "https://horizon.example.com:8443"},
		{"http://127.0.0.1:8000/", "http://127.0.0.1:8000"},
		{"http://[::1]:8000", "http://[::1]:8000"},
		{"http://localhost:8000", "http://localhost:8000"},
	}
	for _, tt := range accepted {
		t.Run("accept "+tt.raw, func(t *testing.T) {
			t.Parallel()
			h, err := NewHorizon(Options{Endpoint: tt.raw})
			if err != nil {
				t.Fatalf("NewHorizon rejected a usable endpoint: %v", err)
			}
			if h.endpoint != tt.want {
				t.Fatalf("endpoint = %q, want %q", h.endpoint, tt.want)
			}
			if cap(h.sem) != defaultMaxConcurrent {
				t.Fatalf("semaphore capacity = %d, want %d", cap(h.sem), defaultMaxConcurrent)
			}
		})
	}

	rejected := []string{
		"",
		"   ",
		"not a url",
		"://bad",
		"ftp://example.com",
		"http://example.com",
		"https://user:secret@example.com",
		"https://example.com/horizon",
		"https://example.com/?query=1",
		"https://example.com/#fragment",
		"https://",
	}
	for _, raw := range rejected {
		t.Run("reject "+raw, func(t *testing.T) {
			t.Parallel()
			_, err := NewHorizon(Options{Endpoint: raw})
			if !errors.Is(err, ErrEndpoint) {
				t.Fatalf("err = %v, want ErrEndpoint", err)
			}
			if raw != "" && strings.Contains(err.Error(), "secret") {
				t.Fatal("the error echoed the endpoint credentials")
			}
		})
	}
}

func TestNewHorizonRejectsNegativeOptions(t *testing.T) {
	t.Parallel()
	for name, opts := range map[string]Options{
		"timeout":        {Endpoint: "https://example.com", Timeout: -time.Second},
		"max body":       {Endpoint: "https://example.com", MaxBody: -1},
		"max retries":    {Endpoint: "https://example.com", MaxRetries: -1},
		"backoff":        {Endpoint: "https://example.com", Backoff: -time.Second},
		"max concurrent": {Endpoint: "https://example.com", MaxConcurrent: -1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewHorizon(opts); !errors.Is(err, ErrOptions) {
				t.Fatalf("err = %v, want ErrOptions", err)
			}
		})
	}
}

func TestNetwork(t *testing.T) {
	t.Parallel()
	root := fixture(t, "root-testnet.json")
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Errorf("path = %q, want /", r.URL.Path)
		}
		serve(w, http.StatusOK, root)
	})

	info, err := clientFor(t, srv, Options{}).Network(context.Background())
	if err != nil {
		t.Fatalf("Network: %v", err)
	}
	if info.Passphrase != "Test SDF Network ; September 2015" {
		t.Fatalf("passphrase = %q", info.Passphrase)
	}
	if info.Ledger != 5466897 {
		t.Fatalf("ledger = %d, want 5466897", info.Ledger)
	}
	if err := CheckPassphrase(info, "Test SDF Network ; September 2015"); err != nil {
		t.Fatalf("CheckPassphrase rejected the matching network: %v", err)
	}
	if err := CheckPassphrase(info, "Public Global Stellar Network ; September 2015"); !errors.Is(err, ErrPassphraseMismatch) {
		t.Fatalf("mismatch err = %v, want ErrPassphraseMismatch", err)
	}
	if err := CheckPassphrase(info, ""); !errors.Is(err, ErrOptions) {
		t.Fatalf("empty expectation err = %v, want ErrOptions", err)
	}
}

func TestNetworkRejectsIncompleteRoot(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"no passphrase": `{"history_latest_ledger": 10}`,
		"no ledger":     `{"network_passphrase": "Test SDF Network ; September 2015"}`,
		"empty ledger":  `{"network_passphrase": "Test SDF Network ; September 2015", "history_latest_ledger": 0}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
				serve(w, http.StatusOK, []byte(body))
			})
			_, err := clientFor(t, srv, Options{}).Network(context.Background())
			if !errors.Is(err, ErrDecode) {
				t.Fatalf("err = %v, want ErrDecode", err)
			}
		})
	}
}

func TestAccountFromFixtures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		fixture string
		id      string
		check   func(*testing.T, model.AccountSnapshot)
	}{
		{
			name:    "default",
			fixture: "1-default.json",
			id:      "GBK4NFJ7SVMUNPBIYBC4WNMCTKI6Z66MYYWZQSSLQNOSUSTMKVUKE7M3",
			check: func(t *testing.T, s model.AccountSnapshot) {
				if s.Sequence != "23472095056887808" || s.Ledger != 5465023 {
					t.Fatalf("sequence/ledger = %q/%d", s.Sequence, s.Ledger)
				}
				if s.Thresholds != (model.Thresholds{}) || s.Flags != (model.Flags{}) {
					t.Fatalf("thresholds/flags = %+v/%+v", s.Thresholds, s.Flags)
				}
				if len(s.Signers) != 1 || s.Signers[0].Key != s.ID || s.Signers[0].Type != model.SignerEd25519 || s.Signers[0].Weight != 1 {
					t.Fatalf("signers = %+v", s.Signers)
				}
				if len(s.Balances) != 1 || !s.Balances[0].IsAuthorized {
					t.Fatalf("balances = %+v", s.Balances)
				}
				if s.HomeDomain != "" || s.Sponsor != "" {
					t.Fatalf("home domain/sponsor = %q/%q", s.HomeDomain, s.Sponsor)
				}
			},
		},
		{
			name:    "extra signer",
			fixture: "7-extra-signer.json",
			id:      "GBQHXYDEJD6NEIUQ5NYDBVNTRCACJXMLO32JUQHBGO524NMVYOMIN5CP",
			check: func(t *testing.T, s model.AccountSnapshot) {
				if len(s.Signers) != 6 {
					t.Fatalf("signers = %d, want 6", len(s.Signers))
				}
				if s.Thresholds != (model.Thresholds{Low: 3, Medium: 3, High: 3}) {
					t.Fatalf("thresholds = %+v", s.Thresholds)
				}
				var zeroWeight int
				for _, signer := range s.Signers {
					if signer.Weight == 0 {
						zeroWeight++
					}
				}
				if zeroWeight != 1 {
					t.Fatalf("zero-weight signers = %d, want 1", zeroWeight)
				}
				if s.Ledger != 5466881 || s.Sequence != "57891864273137" {
					t.Fatalf("ledger/sequence = %d/%q", s.Ledger, s.Sequence)
				}
			},
		},
		{
			name:    "clawback and sponsor",
			fixture: "8-clawback-undeclared.json",
			id:      "GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG",
			check: func(t *testing.T, s model.AccountSnapshot) {
				want := model.Flags{AuthRequired: true, AuthRevocable: true, ClawbackEnabled: true}
				if s.Flags != want {
					t.Fatalf("flags = %+v, want %+v", s.Flags, want)
				}
				if s.HomeDomain != "komunitin.org" {
					t.Fatalf("home domain = %q", s.HomeDomain)
				}
				if s.Sponsor != "GCHEQPEHYQTHK4EVTPV7ASSRLJ6XZBZKEGZ3FWYXRLLQZ7L35PO7OUBM" {
					t.Fatalf("sponsor = %q", s.Sponsor)
				}
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := fixture(t, tt.fixture)
			srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/accounts/"+tt.id {
					t.Errorf("path = %q", r.URL.Path)
				}
				serve(w, http.StatusOK, body)
			})
			snapshot, err := clientFor(t, srv, Options{}).Account(context.Background(), tt.id)
			if err != nil {
				t.Fatalf("Account: %v", err)
			}
			if snapshot.ID != tt.id {
				t.Fatalf("id = %q", snapshot.ID)
			}
			if snapshot.Endpoint != srv.URL {
				t.Fatalf("endpoint = %q, want %q", snapshot.Endpoint, srv.URL)
			}
			tt.check(t, snapshot)
		})
	}
}

func TestAccountRejectsInvalidRequests(t *testing.T) {
	t.Parallel()
	var calls int32
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		serve(w, http.StatusOK, []byte(`{}`))
	})
	h := clientFor(t, srv, Options{})
	for _, id := range []string{"", "GBK4NFJ7SVMUNPBIYBC4WNMCTKI6Z66MYYWZQSSLQNOSUSTMKVUKE7M2", "g" + testAccount[1:], "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUQ"} {
		if _, err := h.Account(context.Background(), id); !errors.Is(err, ErrRequest) {
			t.Fatalf("id %q: err = %v, want ErrRequest", id, err)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("server calls = %d, want 0", got)
	}
}

func TestAccountFailureModes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		opts         Options
		status       int
		body         string
		wantErr      error
		wantAttempts int32
	}{
		{name: "not found", status: http.StatusNotFound, body: string(fixture(t, "10-not-found.json")), wantErr: ErrNotFound, wantAttempts: 1},
		{name: "bad request", status: http.StatusBadRequest, body: `{}`, wantErr: ErrStatus, wantAttempts: 1},
		{name: "forbidden", status: http.StatusForbidden, body: `{}`, wantErr: ErrStatus, wantAttempts: 1},
		{name: "method not allowed", status: http.StatusMethodNotAllowed, body: `{}`, wantErr: ErrStatus, wantAttempts: 1},
		{name: "redirect", status: http.StatusFound, body: ``, wantErr: ErrStatus, wantAttempts: 1},
		{name: "oversized", status: http.StatusOK, body: strings.Repeat("x", 100), opts: Options{MaxBody: 32}, wantErr: ErrTooLarge, wantAttempts: 1},
		{name: "malformed", status: http.StatusOK, body: `{"account_id":`, wantErr: ErrDecode, wantAttempts: 1},
		{name: "server error", status: http.StatusInternalServerError, body: `{}`, opts: Options{MaxRetries: 2}, wantErr: ErrStatus, wantAttempts: 3},
		{name: "gateway timeout", status: http.StatusBadGateway, body: `{}`, opts: Options{MaxRetries: 1}, wantErr: ErrStatus, wantAttempts: 2},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var attempts int32
			srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&attempts, 1)
				if tt.status == http.StatusFound {
					w.Header().Set("Location", "https://elsewhere.invalid/account")
				}
				serve(w, tt.status, []byte(tt.body))
			})
			_, err := clientFor(t, srv, tt.opts).Account(context.Background(), testAccount)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got := atomic.LoadInt32(&attempts); got != tt.wantAttempts {
				t.Fatalf("attempts = %d, want %d", got, tt.wantAttempts)
			}
		})
	}
}

func TestFailureBodiesAreNeverEchoed(t *testing.T) {
	t.Parallel()
	const marker = "MARKER-response-body-should-not-appear"
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		serve(w, http.StatusInternalServerError, []byte(marker))
	})
	h := clientFor(t, srv, Options{MaxRetries: 1})
	_, err := h.Account(context.Background(), testAccount)
	if err == nil {
		t.Fatal("a 500 was accepted")
	}
	if strings.Contains(err.Error(), marker) {
		t.Fatal("the error contains the response body")
	}
}

func TestRetriesBackOffAndGiveUp(t *testing.T) {
	t.Parallel()
	var attempts int32
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			serve(w, http.StatusInternalServerError, []byte(`{}`))
			return
		}
		serve(w, http.StatusOK, fixture(t, "1-default.json"))
	})
	h := clientFor(t, srv, Options{MaxRetries: 3, Backoff: 4 * time.Millisecond})
	var waits []time.Duration
	h.sleep = func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}
	if _, err := h.Account(context.Background(), testAccount); err != nil {
		t.Fatalf("Account: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
	want := []time.Duration{4 * time.Millisecond, 8 * time.Millisecond}
	if len(waits) != len(want) {
		t.Fatalf("waits = %v", waits)
	}
	for i := range want {
		if waits[i] != want[i] {
			t.Fatalf("waits = %v, want %v", waits, want)
		}
	}
}

func TestRetryAfterIsHonouredAndCapped(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{"seconds", "2", 2 * time.Second},
		{"capped", "600", maxRetryAfter},
		{"date falls back", "Wed, 21 Oct 2026 07:28:00 GMT", defaultBackoff},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var attempts int32
			srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
				if atomic.AddInt32(&attempts, 1) == 1 {
					w.Header().Set("Retry-After", tt.header)
					serve(w, http.StatusTooManyRequests, []byte(`{}`))
					return
				}
				serve(w, http.StatusOK, fixture(t, "1-default.json"))
			})
			h := clientFor(t, srv, Options{MaxRetries: 1})
			var waits []time.Duration
			h.sleep = func(ctx context.Context, d time.Duration) error {
				waits = append(waits, d)
				return nil
			}
			if _, err := h.Account(context.Background(), testAccount); err != nil {
				t.Fatalf("Account: %v", err)
			}
			if len(waits) != 1 || waits[0] != tt.want {
				t.Fatalf("waits = %v, want [%v]", waits, tt.want)
			}
		})
	}
}

func TestSlowResponseTimesOut(t *testing.T) {
	t.Parallel()
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		serve(w, http.StatusOK, fixture(t, "1-default.json"))
	})
	h := clientFor(t, srv, Options{Timeout: 20 * time.Millisecond, MaxRetries: 1})
	start := time.Now()
	_, err := h.Account(context.Background(), testAccount)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want a deadline", err)
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("took %v, so the timeout did not bound the call", elapsed)
	}
}

func TestCancelledContextStopsBeforeTheRequest(t *testing.T) {
	t.Parallel()
	var calls int32
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		serve(w, http.StatusOK, fixture(t, "1-default.json"))
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := clientFor(t, srv, Options{}).Account(ctx, testAccount)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("server calls = %d, want 0", got)
	}
}

func TestAccountShapeStrictness(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		edit    func(map[string]any)
		wantErr error
		check   func(*testing.T, model.AccountSnapshot)
	}{
		{
			name: "unknown top level key is ignored",
			edit: func(m map[string]any) { m["future_field"] = 1 },
		},
		{
			name:    "missing account id",
			edit:    func(m map[string]any) { delete(m, "account_id") },
			wantErr: ErrDecode,
		},
		{
			name:    "different account id",
			edit:    func(m map[string]any) { m["account_id"] = otherAccount },
			wantErr: ErrDecode,
		},
		{
			name:    "missing sequence",
			edit:    func(m map[string]any) { delete(m, "sequence") },
			wantErr: ErrDecode,
		},
		{
			name:    "sequence not decimal",
			edit:    func(m map[string]any) { m["sequence"] = "12x" },
			wantErr: ErrDecode,
		},
		{
			name:    "sequence as a number",
			edit:    func(m map[string]any) { m["sequence"] = 12345 },
			wantErr: ErrDecode,
		},
		{
			name:    "missing ledger",
			edit:    func(m map[string]any) { delete(m, "last_modified_ledger") },
			wantErr: ErrDecode,
		},
		{
			name:    "zero ledger",
			edit:    func(m map[string]any) { m["last_modified_ledger"] = 0 },
			wantErr: ErrDecode,
		},
		{
			name:    "missing thresholds",
			edit:    func(m map[string]any) { delete(m, "thresholds") },
			wantErr: ErrDecode,
		},
		{
			name: "unknown threshold key",
			edit: func(m map[string]any) {
				m["thresholds"].(map[string]any)["future_threshold"] = 1
			},
			wantErr: ErrDecode,
		},
		{
			name: "missing threshold tier",
			edit: func(m map[string]any) {
				delete(m["thresholds"].(map[string]any), "high_threshold")
			},
			wantErr: ErrDecode,
		},
		{
			name: "threshold above range",
			edit: func(m map[string]any) {
				m["thresholds"].(map[string]any)["low_threshold"] = 256
			},
			wantErr: ErrDecode,
		},
		{
			name:    "missing flags",
			edit:    func(m map[string]any) { delete(m, "flags") },
			wantErr: ErrDecode,
		},
		{
			name: "unknown flag",
			edit: func(m map[string]any) {
				m["flags"].(map[string]any)["auth_future"] = true
			},
			wantErr: ErrDecode,
		},
		{
			name: "missing flag",
			edit: func(m map[string]any) {
				delete(m["flags"].(map[string]any), "auth_clawback_enabled")
			},
			wantErr: ErrDecode,
		},
		{
			name: "flag is not a boolean",
			edit: func(m map[string]any) {
				m["flags"].(map[string]any)["auth_required"] = "false"
			},
			wantErr: ErrDecode,
		},
		{
			name:    "missing signers",
			edit:    func(m map[string]any) { delete(m, "signers") },
			wantErr: ErrDecode,
		},
		{
			name: "empty signers is a valid state",
			edit: func(m map[string]any) { m["signers"] = []any{} },
			check: func(t *testing.T, s model.AccountSnapshot) {
				if s.Signers == nil || len(s.Signers) != 0 {
					t.Fatalf("signers = %#v, want an empty slice", s.Signers)
				}
			},
		},
		{
			name: "signer without weight",
			edit: func(m map[string]any) {
				delete(m["signers"].([]any)[0].(map[string]any), "weight")
			},
			wantErr: ErrDecode,
		},
		{
			name: "signer weight above range",
			edit: func(m map[string]any) {
				m["signers"].([]any)[0].(map[string]any)["weight"] = 256
			},
			wantErr: ErrDecode,
		},
		{
			name: "unknown signer type",
			edit: func(m map[string]any) {
				m["signers"].([]any)[0].(map[string]any)["type"] = "future_key"
			},
			wantErr: ErrDecode,
		},
		{
			name: "signer key does not match type",
			edit: func(m map[string]any) {
				m["signers"].([]any)[0].(map[string]any)["key"] = testStrkey(23, 32)
			},
			wantErr: ErrDecode,
		},
		{
			name: "duplicate signer",
			edit: func(m map[string]any) {
				m["signers"] = append(m["signers"].([]any), map[string]any{
					"key": testAccount, "type": "ed25519_public_key", "weight": 1,
				})
			},
			wantErr: ErrDecode,
		},
		{
			name: "hash x signer",
			edit: func(m map[string]any) {
				m["signers"] = append(m["signers"].([]any), map[string]any{
					"key": testStrkey(23, 32), "type": "sha256_hash", "weight": 2,
				})
			},
			check: func(t *testing.T, s model.AccountSnapshot) {
				if len(s.Signers) != 2 || s.Signers[1].Type != model.SignerHashX || s.Signers[1].Weight != 2 {
					t.Fatalf("signers = %+v", s.Signers)
				}
			},
		},
		{
			name: "preauth signer",
			edit: func(m map[string]any) {
				m["signers"] = append(m["signers"].([]any), map[string]any{
					"key": testStrkey(19, 32), "type": "preauth_tx", "weight": 1,
				})
			},
			check: func(t *testing.T, s model.AccountSnapshot) {
				if len(s.Signers) != 2 || s.Signers[1].Type != model.SignerPreAuthTx {
					t.Fatalf("signers = %+v", s.Signers)
				}
			},
		},
		{
			name: "signed payload signer",
			edit: func(m map[string]any) {
				m["signers"] = append(m["signers"].([]any), map[string]any{
					"key": testSignedPayloadStrkey(), "type": "ed25519_signed_payload", "weight": 3,
				})
			},
			check: func(t *testing.T, s model.AccountSnapshot) {
				if len(s.Signers) != 2 || s.Signers[1].Type != model.SignerSignedPayload || s.Signers[1].Weight != 3 {
					t.Fatalf("signers = %+v", s.Signers)
				}
			},
		},
		{
			name: "signed payload key under the ed25519 type",
			edit: func(m map[string]any) {
				m["signers"].([]any)[0].(map[string]any)["key"] = testSignedPayloadStrkey()
			},
			wantErr: ErrDecode,
		},
		{
			name:    "missing balances",
			edit:    func(m map[string]any) { delete(m, "balances") },
			wantErr: ErrDecode,
		},
		{
			name: "empty balances is a valid state",
			edit: func(m map[string]any) { m["balances"] = []any{} },
			check: func(t *testing.T, s model.AccountSnapshot) {
				if s.Balances == nil || len(s.Balances) != 0 {
					t.Fatalf("balances = %#v, want an empty slice", s.Balances)
				}
			},
		},
		{
			name: "credit balance",
			edit: func(m map[string]any) { m["balances"] = []any{creditBalance()} },
			check: func(t *testing.T, s model.AccountSnapshot) {
				want := model.Balance{AssetCode: "USDC", AssetIssuer: otherAccount, IsAuthorized: true}
				if len(s.Balances) != 1 || s.Balances[0] != want {
					t.Fatalf("balances = %+v, want %+v", s.Balances, want)
				}
			},
		},
		{
			name: "unauthorized credit balance",
			edit: func(m map[string]any) {
				balance := creditBalance()
				balance["is_authorized"] = false
				m["balances"] = []any{balance}
			},
			check: func(t *testing.T, s model.AccountSnapshot) {
				if s.Balances[0].IsAuthorized {
					t.Fatal("an unauthorized trustline was reported as authorized")
				}
			},
		},
		{
			name: "clawback credit balance",
			edit: func(m map[string]any) {
				balance := creditBalance()
				balance["is_clawback_enabled"] = true
				m["balances"] = []any{balance}
			},
			check: func(t *testing.T, s model.AccountSnapshot) {
				if !s.Balances[0].ClawbackEnabled {
					t.Fatal("clawback was dropped from the balance")
				}
			},
		},
		{
			name: "credit balance without authorization flag",
			edit: func(m map[string]any) {
				balance := creditBalance()
				delete(balance, "is_authorized")
				m["balances"] = []any{balance}
			},
			wantErr: ErrDecode,
		},
		{
			name: "credit balance without issuer",
			edit: func(m map[string]any) {
				balance := creditBalance()
				delete(balance, "asset_issuer")
				m["balances"] = []any{balance}
			},
			wantErr: ErrDecode,
		},
		{
			name: "unmodelled asset type",
			edit: func(m map[string]any) {
				m["balances"] = []any{map[string]any{
					"asset_type": "liquidity_pool_shares", "liquidity_pool_id": "abc",
				}}
			},
			wantErr: ErrDecode,
		},
		{
			name: "null home domain and sponsor",
			edit: func(m map[string]any) {
				m["home_domain"] = nil
				m["sponsor"] = nil
			},
			check: func(t *testing.T, s model.AccountSnapshot) {
				if s.HomeDomain != "" || s.Sponsor != "" {
					t.Fatalf("home domain/sponsor = %q/%q", s.HomeDomain, s.Sponsor)
				}
			},
		},
		{
			name: "account data entry populated",
			edit: func(m map[string]any) {
				m["data"] = map[string]any{
					"config.memo_required": "MQ==",
					"custom_key":           "dmFsdWU=",
				}
			},
			check: func(t *testing.T, s model.AccountSnapshot) {
				if len(s.Data) != 2 {
					t.Fatalf("len(s.Data) = %d, want 2", len(s.Data))
				}
				if s.Data["config.memo_required"] != "MQ==" {
					t.Errorf("s.Data[config.memo_required] = %q, want \"MQ==\"", s.Data["config.memo_required"])
				}
				if s.Data["custom_key"] != "dmFsdWU=" {
					t.Errorf("s.Data[custom_key] = %q, want \"dmFsdWU=\"", s.Data["custom_key"])
				}
			},
		},
		{
			name: "account data entry with non-string value",
			edit: func(m map[string]any) {
				m["data"] = map[string]any{
					"config.memo_required": 123,
				}
			},
			wantErr: ErrDecode,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			account := accountMap(testAccount)
			tt.edit(account)
			body := mustJSON(t, account)
			srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
				serve(w, http.StatusOK, body)
			})
			snapshot, err := clientFor(t, srv, Options{}).Account(context.Background(), testAccount)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Account: %v", err)
			}
			if tt.check != nil {
				tt.check(t, snapshot)
			}
		})
	}
}

func TestConcurrencyIsBounded(t *testing.T) {
	t.Parallel()
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	var inflight, peak int32
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
		serve(w, http.StatusNotFound, []byte(`{}`))
	})
	h := clientFor(t, srv, Options{MaxConcurrent: 1})

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := h.Account(context.Background(), testAccount)
			results <- err
		}()
	}

	<-started
	select {
	case <-started:
		t.Fatal("a second request started while the first was in flight")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	wg.Wait()
	close(results)
	for err := range results {
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	}
	if got := atomic.LoadInt32(&peak); got != 1 {
		t.Fatalf("peak concurrency = %d, want 1", got)
	}
}

func TestRequestHeaders(t *testing.T) {
	t.Parallel()
	var gotUserAgent, gotAccept string
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		serve(w, http.StatusNotFound, []byte(`{}`))
	})
	h := clientFor(t, srv, Options{UserAgent: "stellar-gauge/test"})
	if _, err := h.Account(context.Background(), testAccount); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if gotUserAgent != "stellar-gauge/test" {
		t.Fatalf("user agent = %q", gotUserAgent)
	}
	if gotAccept != "application/json" {
		t.Fatalf("accept = %q", gotAccept)
	}
}

func accountMap(id string) map[string]any {
	return map[string]any{
		"account_id":           id,
		"sequence":             "23472095056887808",
		"last_modified_ledger": 5465023,
		"thresholds": map[string]any{
			"low_threshold": 0, "med_threshold": 0, "high_threshold": 0,
		},
		"flags": map[string]any{
			"auth_required": false, "auth_revocable": false,
			"auth_immutable": false, "auth_clawback_enabled": false,
		},
		"signers": []any{map[string]any{
			"weight": 1, "key": id, "type": "ed25519_public_key",
		}},
		"balances": []any{map[string]any{"asset_type": "native"}},
	}
}

func creditBalance() map[string]any {
	return map[string]any{
		"asset_type":    "credit_alphanum4",
		"asset_code":    "USDC",
		"asset_issuer":  otherAccount,
		"is_authorized": true,
	}
}

// testStrkey builds a valid strkey of the given version byte at test time, so
// no seed-shaped or hash-shaped literal is committed to the repository.
func testStrkey(version byte, size int) string {
	body := make([]byte, size)
	for i := range body {
		body[i] = byte(version)
	}
	return testStrkeyBody(version, body)
}

// testSignedPayloadStrkey builds a valid P strkey: 32 key bytes, a big-endian
// payload length and a payload, the shape ARCHITECTURE.md 6.2 requires.
func testSignedPayloadStrkey() string {
	body := make([]byte, 32)
	for i := range body {
		body[i] = 15
	}
	body = append(body, 0, 0, 0, 4)
	return testStrkeyBody(15, append(body, 1, 2, 3, 4))
}

func testStrkeyBody(version byte, body []byte) string {
	raw := make([]byte, 0, 1+len(body)+2)
	raw = append(raw, version<<3)
	raw = append(raw, body...)
	var crc uint16
	for _, b := range raw {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	raw = append(raw, byte(crc), byte(crc>>8))
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "horizon", name))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return body
}

func startServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func clientFor(t *testing.T, srv *httptest.Server, opts Options) *Horizon {
	t.Helper()
	opts.Endpoint = srv.URL
	h, err := NewHorizon(opts)
	if err != nil {
		t.Fatalf("NewHorizon: %v", err)
	}
	h.sleep = func(context.Context, time.Duration) error { return nil }
	return h
}

func serve(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/hal+json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshalling the test body: %v", err)
	}
	return body
}

func testAccountID(i int) string {
	body := make([]byte, 32)
	body[0] = byte(i >> 8)
	body[1] = byte(i)
	for j := 2; j < 32; j++ {
		body[j] = 0x42
	}
	return testStrkeyBody(6, body) // 6 is VersionByteAccountID (prefix 'G')
}

func TestManyAccountBoundedConcurrency(t *testing.T) {
	t.Parallel()
	const accountCount = 50
	const maxConcurrent = 4

	var inflight, peak int32
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&inflight, 1)
		for {
			seen := atomic.LoadInt32(&peak)
			if current <= seen || atomic.CompareAndSwapInt32(&peak, seen, current) {
				break
			}
		}
		// Brief delay to give concurrent goroutines opportunities to overlap.
		time.Sleep(5 * time.Millisecond)
		atomic.AddInt32(&inflight, -1)

		id := strings.TrimPrefix(r.URL.Path, "/accounts/")
		serve(w, http.StatusOK, mustJSON(t, accountMap(id)))
	})

	h := clientFor(t, srv, Options{MaxConcurrent: maxConcurrent})

	var wg sync.WaitGroup
	errs := make(chan error, accountCount)
	for i := 0; i < accountCount; i++ {
		wg.Add(1)
		id := testAccountID(i)
		go func(accID string) {
			defer wg.Done()
			snap, err := h.Account(context.Background(), accID)
			if err != nil {
				errs <- err
				return
			}
			if snap.ID != accID {
				errs <- fmt.Errorf("unexpected id: got %s, want %s", snap.ID, accID)
			}
		}(id)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("unexpected error fetching account: %v", err)
	}

	observedPeak := atomic.LoadInt32(&peak)
	if observedPeak > maxConcurrent {
		t.Fatalf("peak concurrency = %d, exceeded cap of %d", observedPeak, maxConcurrent)
	}
	if observedPeak < 2 {
		t.Fatalf("peak concurrency = %d, expected multiple concurrent requests", observedPeak)
	}
}

func TestManyAccountRateLimitingAndBackoff(t *testing.T) {
	t.Parallel()
	const accountCount = 50

	// Simulating rate limiting: first attempt for each account returns 429 with Retry-After: 1s.
	// Second attempt succeeds.
	var attempts sync.Map
	var total429s int32
	var sleepCalls int32
	var sleepWaits sync.Map

	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/accounts/")
		val, _ := attempts.LoadOrStore(id, new(int32))
		countPtr := val.(*int32)
		attemptNum := atomic.AddInt32(countPtr, 1)

		if attemptNum == 1 {
			atomic.AddInt32(&total429s, 1)
			w.Header().Set("Retry-After", "1")
			serve(w, http.StatusTooManyRequests, []byte(`{"type":"https://stellar.org/horizon-errors/too_many_requests","title":"Timeout"}`))
			return
		}
		serve(w, http.StatusOK, mustJSON(t, accountMap(id)))
	})

	h := clientFor(t, srv, Options{MaxRetries: 2, Backoff: 10 * time.Millisecond})
	h.sleep = func(ctx context.Context, d time.Duration) error {
		atomic.AddInt32(&sleepCalls, 1)
		sleepWaits.Store(d, true)
		return nil
	}

	var wg sync.WaitGroup
	errs := make(chan error, accountCount)
	for i := 0; i < accountCount; i++ {
		wg.Add(1)
		id := testAccountID(i)
		go func(accID string) {
			defer wg.Done()
			snap, err := h.Account(context.Background(), accID)
			if err != nil {
				errs <- fmt.Errorf("account %s failed: %w", accID, err)
				return
			}
			if snap.ID != accID {
				errs <- fmt.Errorf("id mismatch: got %s, want %s", snap.ID, accID)
			}
		}(id)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("unexpected error during rate-limit test: %v", err)
	}

	if got429 := atomic.LoadInt32(&total429s); got429 != accountCount {
		t.Fatalf("total 429 responses = %d, want %d", got429, accountCount)
	}
	if gotSleeps := atomic.LoadInt32(&sleepCalls); gotSleeps != accountCount {
		t.Fatalf("total sleep calls = %d, want %d", gotSleeps, accountCount)
	}
	// Verify that the 1s Retry-After header was honored.
	if _, ok := sleepWaits.Load(1 * time.Second); !ok {
		t.Fatal("expected sleep duration of 1s from Retry-After was not observed")
	}
}

func TestManyAccountRateLimitExhaustionFailsClosed(t *testing.T) {
	t.Parallel()
	const accountCount = 20
	const maxRetries = 2

	var totalRequests int32
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&totalRequests, 1)
		serve(w, http.StatusTooManyRequests, []byte(`{"title":"rate limit exceeded"}`))
	})

	h := clientFor(t, srv, Options{MaxRetries: maxRetries})
	var sleepCalls int32
	h.sleep = func(ctx context.Context, d time.Duration) error {
		atomic.AddInt32(&sleepCalls, 1)
		return nil
	}

	var wg sync.WaitGroup
	errs := make(chan error, accountCount)
	for i := 0; i < accountCount; i++ {
		wg.Add(1)
		id := testAccountID(i)
		go func(accID string) {
			defer wg.Done()
			_, err := h.Account(context.Background(), accID)
			errs <- err
		}(id)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if !errors.Is(err, ErrStatus) {
			t.Fatalf("got err %v, want ErrStatus", err)
		}
		if strings.Contains(err.Error(), "rate limit exceeded") {
			t.Fatalf("error echoed response body: %v", err)
		}
	}

	expectedAttempts := int32(accountCount * (maxRetries + 1))
	if got := atomic.LoadInt32(&totalRequests); got != expectedAttempts {
		t.Fatalf("total requests = %d, want %d", got, expectedAttempts)
	}
	expectedSleeps := int32(accountCount * maxRetries)
	if got := atomic.LoadInt32(&sleepCalls); got != expectedSleeps {
		t.Fatalf("total sleeps = %d, want %d", got, expectedSleeps)
	}
}

func TestManyAccountTimeoutsAndSizeCapsUnderLoad(t *testing.T) {
	t.Parallel()
	const accountCount = 60

	// 20 normal accounts, 20 oversized accounts, 20 slow accounts (timing out)
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/accounts/")
		// Use index encoding in id body: body[0]<<8 | body[1]
		var idx int
		for i := 0; i < accountCount; i++ {
			if testAccountID(i) == id {
				idx = i
				break
			}
		}

		switch {
		case idx < 20: // Normal
			serve(w, http.StatusOK, mustJSON(t, accountMap(id)))
		case idx < 40: // Oversized
			serve(w, http.StatusOK, []byte(strings.Repeat(" ", 2000)+string(mustJSON(t, accountMap(id)))))
		default: // Slow
			time.Sleep(100 * time.Millisecond)
			serve(w, http.StatusOK, mustJSON(t, accountMap(id)))
		}
	})

	h := clientFor(t, srv, Options{
		MaxBody:    1000, // normal is ~500 bytes; oversized is >2500 bytes to trigger ErrTooLarge
		Timeout:    20 * time.Millisecond,
		MaxRetries: 0, // no retries so timeout fails quickly
	})

	var wg sync.WaitGroup
	type result struct {
		idx int
		err error
	}
	results := make(chan result, accountCount)

	for i := 0; i < accountCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := h.Account(context.Background(), testAccountID(idx))
			results <- result{idx: idx, err: err}
		}(i)
	}

	wg.Wait()
	close(results)

	for res := range results {
		switch {
		case res.idx < 20:
			if res.err != nil {
				t.Fatalf("normal account %d failed: %v", res.idx, res.err)
			}
		case res.idx < 40:
			if !errors.Is(res.err, ErrTooLarge) {
				t.Fatalf("account %d: err = %v, want ErrTooLarge", res.idx, res.err)
			}
		default:
			if !errors.Is(res.err, context.DeadlineExceeded) {
				t.Fatalf("account %d: err = %v, want context.DeadlineExceeded", res.idx, res.err)
			}
		}
	}
}

func TestDefaultOptionsAlignWithDocumentedTargets(t *testing.T) {
	t.Parallel()
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		serve(w, http.StatusOK, fixture(t, "1-default.json"))
	})
	h, err := NewHorizon(Options{Endpoint: srv.URL})
	if err != nil {
		t.Fatalf("NewHorizon: %v", err)
	}

	// Verify the 0.12 and PRD 6.4/7 documented defaults:
	// - Concurrency cap: 8 (defaultMaxConcurrent)
	// - Timeout: 10s (defaultTimeout)
	// - MaxBody: 1 MiB (defaultMaxBody)
	// - MaxRetries: 3 (defaultMaxRetries)
	// - Backoff: 200ms (defaultBackoff)
	if cap(h.sem) != 8 {
		t.Fatalf("default max concurrency = %d, want 8", cap(h.sem))
	}
	if h.timeout != 10*time.Second {
		t.Fatalf("default timeout = %v, want 10s", h.timeout)
	}
	if h.maxBody != 1<<20 {
		t.Fatalf("default max body = %d, want 1048576 (1 MiB)", h.maxBody)
	}
	if h.maxRetries != 3 {
		t.Fatalf("default max retries = %d, want 3", h.maxRetries)
	}
	if h.backoff != 200*time.Millisecond {
		t.Fatalf("default backoff = %v, want 200ms", h.backoff)
	}

	// Backoff calculation caps at maxBackoff (5s)
	if b := backoffFor(h.backoff, 10); b != 5*time.Second {
		t.Fatalf("backoffFor(10) = %v, want 5s cap", b)
	}

	// Retry-After caps at maxRetryAfter (5s)
	hreq := http.Header{}
	hreq.Set("Retry-After", "120")
	if d := retryAfter(hreq); d != 5*time.Second {
		t.Fatalf("retryAfter(120) = %v, want 5s cap", d)
	}
}
