package report

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/sanitize"
)

const (
	hostileOSC       = "\x1b]0;title\x07"
	hostileBidi      = "\u202e"
	hostileZeroWidth = "\u200b"
	hostileLongTail  = "TAIL.invalid"
)

func hostileText() string {
	return hostileOSC + " " + hostileBidi + hostileZeroWidth + " " + strings.Repeat("a.", 200) + hostileLongTail
}

func hostileReport() model.Report {
	return model.Report{
		ToolVersion: "seed",
		Network:     "seed",
		Ledger:      1,
		Accounts: []model.AccountReport{{
			Snapshot: model.AccountSnapshot{
				ID:         "seed",
				Network:    "seed",
				Endpoint:   "seed",
				Ledger:     1,
				Sequence:   "1",
				Thresholds: model.Thresholds{Low: 1, Medium: 1, High: 1},
				Signers:    []model.Signer{{Key: "seed", Sponsor: "seed", Type: model.SignerEd25519, Weight: 1}},
				Flags:      model.Flags{AuthRequired: true},
				HomeDomain: "seed.example",
				Sponsor:    "seed",
				Balances:   []model.Balance{{AssetCode: "seed", AssetIssuer: "seed", IsAuthorized: true}},
				Data:       map[string]string{"seed": "seed"},
			},
			Facts: &model.AccountFacts{
				Reachability: &model.Reachability{Low: value(1)},
				Declared:     &model.DeclaredFacts{Role: "other", Coordinator: value("seed.example")},
				Identity: &model.IdentityFacts{
					HomeDomain: "seed.example", Step: "seed", Accounts: []string{"seed"},
				},
				Coordinator: &model.CoordinatorFact{DataKey: "seed", Domain: "seed.example", Valid: true},
				Trustlines:  []model.TrustlineFact{{AssetCode: "seed", AssetIssuer: "seed", IssuerStatus: "seed"}},
			},
			Findings: []model.Finding{model.NewFinding("seed", model.SevWarn, "seed", "seed", "seed")},
			Skipped:  "seed",
		}},
		ExitCode: model.ExitFindings,
	}
}

func fillEveryString(target reflect.Value, value string) {
	switch target.Kind() {
	case reflect.Pointer:
		if target.IsNil() {
			if !target.CanSet() {
				return
			}
			target.Set(reflect.New(target.Type().Elem()))
		}
		fillEveryString(target.Elem(), value)
	case reflect.Struct:
		for i := 0; i < target.NumField(); i++ {
			fillEveryString(target.Field(i), value)
		}
	case reflect.Slice:
		for i := 0; i < target.Len(); i++ {
			fillEveryString(target.Index(i), value)
		}
	case reflect.Map:
		if target.IsNil() || !target.CanSet() {
			return
		}
		clean := reflect.MakeMapWithSize(target.Type(), target.Len())
		iter := target.MapRange()
		for iter.Next() {
			key := reflect.New(iter.Key().Type()).Elem()
			fillEveryString(key, value)
			item := reflect.New(iter.Value().Type()).Elem()
			item.Set(iter.Value())
			fillEveryString(item, value)
			clean.SetMapIndex(key, item)
		}
		target.Set(clean)
	case reflect.String:
		if target.CanSet() {
			target.SetString(value)
		}
	}
}

func assertCleanStrings(t *testing.T, value any) {
	t.Helper()
	switch typed := value.(type) {
	case string:
		for _, unwanted := range []string{"\x1b", "\x07", "title", hostileBidi, hostileZeroWidth, hostileLongTail} {
			if strings.Contains(typed, unwanted) {
				t.Fatalf("JSON output kept %q in %q", unwanted, typed)
			}
		}
		if utf8.RuneCountInString(typed) > sanitize.MaxFieldLen {
			t.Fatalf("JSON output field has %d runes, cap is %d: %q", utf8.RuneCountInString(typed), sanitize.MaxFieldLen, typed)
		}
	case map[string]any:
		for key, nested := range typed {
			assertCleanStrings(t, key)
			assertCleanStrings(t, nested)
		}
	case []any:
		for _, nested := range typed {
			assertCleanStrings(t, nested)
		}
	}
}

func TestEveryJSONStringIsSanitized(t *testing.T) {
	t.Parallel()
	report := hostileReport()
	payload := hostileText()
	fillEveryString(reflect.ValueOf(&report).Elem(), payload)

	var buf bytes.Buffer
	if err := JSON(&buf, report); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	assertCleanStrings(t, decoded)
}
