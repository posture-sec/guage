package sanitize

// Domain sanitizes a host name and reports whether the sanitized string
// contains a rune above U+007F. The flag describes the printed host, not the
// raw input: a host whose only non-ASCII runes are dropped controls (bidi
// overrides, tag characters, C1 controls and similar) will be reported as
// ASCII because the sanitized output is ASCII. Punycode labels (xn--) are
// never decoded — Go's standard library performs no IDNA mapping — so a
// punycode host is always ASCII and always returns false.
func Domain(s string) (string, bool) {
	d := Text(s)
	for _, r := range d {
		if r > 0x7f {
			return d, true
		}
	}
	return d, false
}
