package source

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

func TestThresholdDecodeStrictness(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		edit    func(map[string]any)
		wantErr error
		check   func(*testing.T, model.AccountSnapshot)
	}{
		{
			name: "thresholds is null",
			edit: func(m map[string]any) {
				m["thresholds"] = nil
			},
			wantErr: ErrDecode,
		},
		{
			name: "thresholds is array",
			edit: func(m map[string]any) {
				m["thresholds"] = []any{1, 2, 3}
			},
			wantErr: ErrDecode,
		},
		{
			name: "thresholds is absent",
			edit: func(m map[string]any) {
				delete(m, "thresholds")
			},
			wantErr: ErrDecode,
		},
		{
			name: "null low_threshold tier",
			edit: func(m map[string]any) {
				m["thresholds"].(map[string]any)["low_threshold"] = nil
			},
			wantErr: ErrDecode,
		},
		{
			name: "null med_threshold tier",
			edit: func(m map[string]any) {
				m["thresholds"].(map[string]any)["med_threshold"] = nil
			},
			wantErr: ErrDecode,
		},
		{
			name: "null high_threshold tier",
			edit: func(m map[string]any) {
				m["thresholds"].(map[string]any)["high_threshold"] = nil
			},
			wantErr: ErrDecode,
		},
		{
			name: "string low_threshold tier",
			edit: func(m map[string]any) {
				m["thresholds"].(map[string]any)["low_threshold"] = "1"
			},
			wantErr: ErrDecode,
		},
		{
			name: "string med_threshold tier",
			edit: func(m map[string]any) {
				m["thresholds"].(map[string]any)["med_threshold"] = "1"
			},
			wantErr: ErrDecode,
		},
		{
			name: "string high_threshold tier",
			edit: func(m map[string]any) {
				m["thresholds"].(map[string]any)["high_threshold"] = "1"
			},
			wantErr: ErrDecode,
		},
		{
			name: "float low_threshold tier",
			edit: func(m map[string]any) {
				m["thresholds"].(map[string]any)["low_threshold"] = 1.5
			},
			wantErr: ErrDecode,
		},
		{
			name: "float med_threshold tier",
			edit: func(m map[string]any) {
				m["thresholds"].(map[string]any)["med_threshold"] = 2.5
			},
			wantErr: ErrDecode,
		},
		{
			name: "float high_threshold tier",
			edit: func(m map[string]any) {
				m["thresholds"].(map[string]any)["high_threshold"] = 3.5
			},
			wantErr: ErrDecode,
		},
		{
			name: "range pin: tier 256 is out of range",
			edit: func(m map[string]any) {
				m["thresholds"].(map[string]any)["low_threshold"] = 256
			},
			wantErr: ErrDecode,
		},
		{
			name: "range pin: tier negative is out of range",
			edit: func(m map[string]any) {
				m["thresholds"].(map[string]any)["low_threshold"] = -1
			},
			wantErr: ErrDecode,
		},
		{
			name: "range pin: tier 0 is valid",
			edit: func(m map[string]any) {
				t := m["thresholds"].(map[string]any)
				t["low_threshold"] = 0
				t["med_threshold"] = 10
				t["high_threshold"] = 20
			},
			check: func(t *testing.T, s model.AccountSnapshot) {
				if s.Thresholds.Low != 0 || s.Thresholds.Medium != 10 || s.Thresholds.High != 20 {
					t.Fatalf("thresholds = %+v, want low=0 med=10 high=20", s.Thresholds)
				}
			},
		},
		{
			name: "range pin: tier 255 is valid",
			edit: func(m map[string]any) {
				t := m["thresholds"].(map[string]any)
				t["low_threshold"] = 255
				t["med_threshold"] = 255
				t["high_threshold"] = 255
			},
			check: func(t *testing.T, s model.AccountSnapshot) {
				if s.Thresholds.Low != 255 || s.Thresholds.Medium != 255 || s.Thresholds.High != 255 {
					t.Fatalf("thresholds = %+v, want all 255", s.Thresholds)
				}
			},
		},
		{
			name: "successful read preserves non-zero threshold without defaulting to zero",
			edit: func(m map[string]any) {
				t := m["thresholds"].(map[string]any)
				t["low_threshold"] = 5
				t["med_threshold"] = 15
				t["high_threshold"] = 25
			},
			check: func(t *testing.T, s model.AccountSnapshot) {
				if s.Thresholds.Low != 5 || s.Thresholds.Medium != 15 || s.Thresholds.High != 25 {
					t.Fatalf("thresholds = %+v, want 5, 15, 25", s.Thresholds)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			account := accountMap(testAccount)
			tc.edit(account)
			body := mustJSON(t, account)
			srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
				serve(w, http.StatusOK, body)
			})
			snapshot, err := clientFor(t, srv, Options{}).Account(context.Background(), testAccount)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err = %v", err)
			}
			if tc.check != nil {
				tc.check(t, snapshot)
			}
		})
	}
}
