package model_test

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// Evidence that prints the same must hash the same. The report drops Unicode
// format characters before printing, so the fingerprint input has to drop them
// too, or one printed finding can carry two digests.
func TestNormalizeFormatCharacters(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		with string
		base string
	}{
		{"zero width space", "alpha\u200bbeta", "alphabeta"},
		{"byte order mark", "\ufeffalpha beta", "alpha beta"},
		{"right-to-left override", "alpha\u202ebeta", "alphabeta"},
		{"word joiner", "alpha\u2060beta", "alphabeta"},
		{"format characters around whitespace", "\u200balpha\u200b \u200bbeta\ufeff", "alpha beta"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := model.NewFinding("sig-hashx", model.SevWarn, "account-reference", "message", tt.with)
			want := model.NewFinding("sig-hashx", model.SevWarn, "account-reference", "message", tt.base)
			if got.Fingerprint != want.Fingerprint {
				t.Fatalf("format character changed the fingerprint: %q and %q", tt.with, tt.base)
			}
		})
	}

	different := model.NewFinding("sig-hashx", model.SevWarn, "account-reference", "message", "gamma")
	same := model.NewFinding("sig-hashx", model.SevWarn, "account-reference", "message", "alpha")
	if different.Fingerprint == same.Fingerprint {
		t.Fatal("different evidence shares a fingerprint")
	}
}
