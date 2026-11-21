package sanitize

import (
	"testing"
	"unicode/utf8"
)

// TestDomainIDNMatrix covers the full matrix of IDN-flag behaviour described in
// ARCHITECTURE.md 6.8: the flag follows sanitization of the returned string, not
// the raw input, and punycode (xn--) stays ASCII and therefore never raises the
// flag.
//
// Each case asserts idn == (returned string contains a rune above U+007F),
// computed from the output rather than from the raw input, so the test fails if
// Domain scans s instead of d (the sanitised result).
func TestDomainIDNMatrix(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		in   string
		// wantOut is the expected sanitised host string.
		wantOut string
		// wantIDN is the expected flag value; it must equal whether wantOut
		// contains a rune above U+007F.
		wantIDN bool
	}{
		// Plain ASCII host: sanitisation is a no-op, flag is false.
		{
			name:    "ascii",
			in:      "example.com",
			wantOut: "example.com",
			wantIDN: false,
		},
		// Punycode ACE label: the string is entirely ASCII, so the flag is
		// false. Go's standard library performs no IDNA mapping (ARCHITECTURE.md
		// 6.8), so xn-- stays as-is and is not decoded to its Unicode form.
		{
			name:    "punycode xn--",
			in:      "xn--80ak6aa92e.com",
			wantOut: "xn--80ak6aa92e.com",
			wantIDN: false,
		},
		// Cyrillic lookalike: U+0430 (а) survives sanitisation because it is a
		// legitimate Unicode letter, not a control or dropped character. The flag
		// is true because the output contains a rune above U+007F.
		{
			name:    "cyrillic lookalike",
			in:      "ex\u0430mple.com",
			wantOut: "ex\u0430mple.com",
			wantIDN: true,
		},
		// Mixed Latin and Cyrillic: the Cyrillic rune survives, so the flag is
		// true. This is the mixed-script lookalike risk the report must name.
		{
			name:    "mixed latin and cyrillic",
			in:      "pаypal.com", // 'а' is U+0430, the rest are ASCII
			wantOut: "pаypal.com",
			wantIDN: true,
		},
		// Tag-masked host: Unicode tag characters (U+E0041…) are stripped by
		// Text/sanitisation before the IDN flag is computed. The printed host is
		// ASCII, so the flag is false. This proves that sanitisation runs first.
		{
			name:    "tag-masked",
			in:      "example\U000e002ecom", // tag full stop between "example" and "com"
			wantOut: "examplecom",
			wantIDN: false,
		},
		// Bidi-masked host: bidi override characters (U+202E etc.) are stripped
		// by sanitisation. The printed host is ASCII, so the flag is false.
		{
			name:    "bidi-masked",
			in:      "\u202eexample.com\u202c",
			wantOut: "example.com",
			wantIDN: false,
		},
		// C1-masked host: C1 controls (U+0080–U+009F) are stripped by
		// sanitisation. The printed host is ASCII, so the flag is false.
		{
			name:    "c1-masked",
			in:      "exam\u0085ple.com", // U+0085 is a C1 control / NEL
			wantOut: "example.com",
			wantIDN: false,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, idn := Domain(tt.in)

			// The flag must equal whether the returned string contains a rune
			// above U+007F, computed from the output, not the raw input.
			wantIDNFromOutput := false
			for _, r := range got {
				if r > 0x7f {
					wantIDNFromOutput = true
					break
				}
			}
			if wantIDNFromOutput != tt.wantIDN {
				t.Fatalf("test case %q: wantIDN=%v does not match whether wantOut %q contains a rune > U+007F (%v); fix the test case",
					tt.name, tt.wantIDN, tt.wantOut, wantIDNFromOutput)
			}

			if got != tt.wantOut {
				t.Errorf("Domain(%q) string = %q, want %q", tt.in, got, tt.wantOut)
			}
			if idn != tt.wantIDN {
				t.Errorf("Domain(%q) idn = %v, want %v", tt.in, idn, tt.wantIDN)
			}

			// Sanity: the returned string must be valid UTF-8.
			if !utf8.ValidString(got) {
				t.Errorf("Domain(%q) returned invalid UTF-8: %q", tt.in, got)
			}
		})
	}
}
