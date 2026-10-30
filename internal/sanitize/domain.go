package sanitize

// Domain sanitizes a host name and reports whether it contains non-ASCII
// characters. A non-ASCII host is an IDN, which is the lookalike risk the
// report has to name rather than hide.
func Domain(s string) (string, bool) {
	d := Text(s)
	for _, r := range d {
		if r > 0x7f {
			return d, true
		}
	}
	return d, false
}
