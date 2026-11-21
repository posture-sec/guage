package sanitize

import "testing"

// TestTextDropsC1AndDEL pins the DEL and C1 removals at their boundaries. In
// 8-bit terminals U+009B is CSI and U+009D is OSC, so a hostile home domain
// could carry an escape introducer without the ESC byte escapeLen consumes.
// Hostile input is written as escapes; no control byte is embedded here.
func TestTextDropsC1AndDEL(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{"U+007E tilde survives", "a~b", "a~b"},
		{"U+007F DEL", "a\u007fb", "ab"},
		{"U+0080 lower C1 edge", "a\u0080b", "ab"},
		{"U+0085 next line", "a\u0085b", "ab"},
		{"U+009B CSI", "a\u009bb", "ab"},
		{"U+009C string terminator", "a\u009cb", "ab"},
		{"U+009D OSC", "a\u009db", "ab"},
		{"U+009F upper C1 edge", "a\u009fb", "ab"},
		{"8-bit CSI sequence leaves only its parameters", "a\u009b31mb", "a31mb"},
		{"run of C1 and DEL", "a\u007f\u0080\u0085\u009b\u009c\u009fb", "ab"},
		// U+00A0 is whitespace, U+00A1 and U+00E9 are printable Latin-1:
		// the drop is a control range, not the Latin-1 block.
		{"U+00A0 collapses to one space", "a\u00a0b", "a b"},
		{"U+00A1 survives", "a\u00a1b", "a\u00a1b"},
		{"U+00E9 survives", "caf\u00e9", "caf\u00e9"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Text(tt.in); got != tt.want {
				t.Fatalf("Text(%+q) = %+q, want %+q", tt.in, got, tt.want)
			}
		})
	}

	t.Run("C1 inside a host", func(t *testing.T) {
		t.Parallel()
		for _, in := range []string{
			"exa\u009bmple.com",
			"exa\u0080\u009b\u009fmple.com",
			"\u009dexample\u007f.com\u009c",
		} {
			if got := Text(in); got != "example.com" {
				t.Fatalf("Text(%+q) = %+q, want %q", in, got, "example.com")
			}
			d, idn := Domain(in)
			if d != "example.com" || idn {
				t.Fatalf("Domain(%+q) = %+q, %v, want %q, false", in, d, idn, "example.com")
			}
		}
	})
}
