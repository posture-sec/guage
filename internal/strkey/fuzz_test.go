package strkey

import (
	"strings"
	"testing"
)

func FuzzClassify(f *testing.F) {
	for _, v := range sep23Valid {
		f.Add(v.s)
	}
	for _, v := range sep23Invalid {
		f.Add(v)
	}
	f.Add("")
	f.Add("S")
	f.Fuzz(func(t *testing.T, s string) {
		kind, err := Classify(s)
		if err != nil {
			return
		}
		if kind == "" {
			t.Fatal("a valid strkey has no kind")
		}
		if kind == AccountID && !IsAccountID(s) {
			t.Fatal("Classify accepted an account ID that IsAccountID rejects")
		}
		if kind != AccountID && IsAccountID(s) {
			t.Fatal("IsAccountID accepted a non-account strkey")
		}
		if kind == Seed != IsSeed(s) {
			t.Fatal("IsSeed disagrees with Classify")
		}
		if s != strings.ToUpper(s) {
			t.Fatalf("Classify accepted a non-uppercase strkey")
		}
	})
}
