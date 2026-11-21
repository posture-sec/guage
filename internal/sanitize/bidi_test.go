package sanitize

import (
	"fmt"
	"testing"
)

// bidiControls lists every directional mark, override, and isolate control
// that must be stripped by sanitize.Text and isDropped.
var bidiControls = []struct {
	name string
	r    rune
}{
	{"ALM (Arabic letter mark)", '\u061c'},
	{"LRM (Left-to-right mark)", '\u200e'},
	{"RLM (Right-to-left mark)", '\u200f'},
	{"LRE (Left-to-right embedding)", '\u202a'},
	{"RLE (Right-to-left embedding)", '\u202b'},
	{"PDF (Pop directional formatting)", '\u202c'},
	{"LRO (Left-to-right override)", '\u202d'},
	{"RLO (Right-to-left override)", '\u202e'},
	{"LRI (Left-to-right isolate)", '\u2066'},
	{"RLI (Right-to-left isolate)", '\u2067'},
	{"FSI (First strong isolate)", '\u2068'},
	{"PDI (Pop directional isolate)", '\u2069'},
}

// bidiControlSet is a map of the exact runes dropped for bidi control checking.
var bidiControlSet = func() map[rune]bool {
	m := make(map[rune]bool)
	for _, c := range bidiControls {
		m[c.r] = true
	}
	return m
}()

func TestBidiControlsRemovedPerRune(t *testing.T) {
	t.Parallel()

	for _, tc := range bidiControls {
		tc := tc
		t.Run(fmt.Sprintf("%s_U+%04X", tc.name, tc.r), func(t *testing.T) {
			t.Parallel()
			in := fmt.Sprintf("a%cb", tc.r)
			got := Text(in)
			want := "ab"
			if got != want {
				t.Fatalf("Text(%q) = %q, want %q", in, got, want)
			}

			// Assert output holds no rune from the dropped set, checked rune by rune.
			for _, r := range got {
				if bidiControlSet[r] {
					t.Fatalf("Text(%q) output %q contains dropped bidi control U+%04X", in, got, r)
				}
			}
		})
	}

	t.Run("TrojanSourceLine", func(t *testing.T) {
		t.Parallel()
		// Trojan Source attack string: e.g. using RLO (U+202E) and PDI (U+2069)
		// Example: "admin\u202e\u2069/*check*/\u2066access"
		in := "user_\u202e\u2066admin\u2069\u202c_test"
		got := Text(in)
		for _, r := range got {
			if bidiControlSet[r] {
				t.Fatalf("Trojan source line output %q contains bidi control U+%04X", got, r)
			}
		}
		if got != "user_admin_test" {
			t.Fatalf("Text(%q) = %q, want %q", in, got, "user_admin_test")
		}
	})

	t.Run("SurvivingRTLRunes", func(t *testing.T) {
		t.Parallel()
		// Hebrew U+05D0 (Alef), Arabic U+0627 (Alef), and U+061F (Arabic question mark)
		// must survive sanitization.
		cases := []struct {
			name string
			in   string
			want string
		}{
			{"Hebrew U+05D0", "a\u05d0b", "a\u05d0b"},
			{"Arabic U+0627", "a\u0627b", "a\u0627b"},
			{"Arabic Question Mark U+061F", "a\u061fb", "a\u061fb"},
		}
		for _, tt := range cases {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				if got := Text(tt.in); got != tt.want {
					t.Fatalf("Text(%q) = %q, want %q", tt.in, got, tt.want)
				}
			})
		}
	})

	t.Run("DomainReportsFalseForDroppedBidiASCIIHost", func(t *testing.T) {
		t.Parallel()
		// Domain reports false for an ASCII host whose only non-ASCII rune was a dropped control.
		for _, tc := range bidiControls {
			tc := tc
			t.Run(fmt.Sprintf("%s_U+%04X", tc.name, tc.r), func(t *testing.T) {
				t.Parallel()
				in := fmt.Sprintf("example%c.com", tc.r)
				got, isIDN := Domain(in)
				if got != "example.com" {
					t.Errorf("Domain(%q) host = %q, want %q", in, got, "example.com")
				}
				if isIDN {
					t.Errorf("Domain(%q) isIDN = %v, want false", in, isIDN)
				}
				for _, r := range got {
					if bidiControlSet[r] {
						t.Errorf("Domain(%q) output %q contains dropped bidi control U+%04X", in, got, r)
					}
				}
			})
		}
	})
}
