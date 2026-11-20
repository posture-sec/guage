package sanitize

import (
	"strings"
	"testing"
)

// TestZeroWidthSet pins each zero-width code point isDropped removes, one row
// per rune, so a rune that leaves the set fails a subtest naming it. A
// zero-width rune occupies no terminal cell, so one left inside a key or a
// domain makes the printed value read as two strings.
func TestZeroWidthSet(t *testing.T) {
	t.Parallel()

	const key = "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7"
	const all = "\u200b\u200c\u200d\u2060\ufeff"

	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{"U+200B zero width space", "a\u200bb", "ab"},
		{"U+200C zero width non-joiner", "a\u200cb", "ab"},
		{"U+200D zero width joiner", "a\u200db", "ab"},
		{"U+2060 word joiner", "a\u2060b", "ab"},
		{"U+FEFF byte order mark", "a\ufeffb", "ab"},
		{"run of all five", "a" + all + "b", "ab"},
		{"run of all five, doubled", all + "a" + all + all + "b" + all, "ab"},
		{"inside a G key", key[:10] + "\u200b" + key[10:28] + "\u2060\ufeff" + key[28:50] + "\u200c\u200d" + key[50:], key},
		{"one per position in a G key", interleave(key, all), key},
		// Visible hyphens next to the set are payload, not formatting.
		{"U+2010 hyphen survives", "a\u200b\u2010\u200bb", "a\u2010b"},
		{"U+2011 non-breaking hyphen survives", "a\u2011\ufeffb", "a\u2011b"},
		// U+2003 is whitespace: it collapses to one ASCII space rather than
		// vanishing, so it cannot join two words the way a dropped rune does.
		{"U+2003 em space collapses", "a\u2003b", "a b"},
		{"U+2003 beside dropped runes collapses once", "a\u200b\u2003\u2003\ufeffb", "a b"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Text(tt.in); got != tt.want {
				t.Fatalf("Text(%+q) = %+q, want %+q", tt.in, got, tt.want)
			}
		})
	}
}

// interleave places one rune of sep, cycling, after every byte of s. The
// inputs are ASCII, so every byte is a rune boundary.
func interleave(s, sep string) string {
	seps := []rune(sep)
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		b.WriteByte(s[i])
		b.WriteRune(seps[i%len(seps)])
	}
	return b.String()
}
