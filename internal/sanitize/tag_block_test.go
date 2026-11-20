package sanitize

import "testing"

// TestTagBlockIsDropped pins the Unicode tag block, U+E0000 through U+E007F,
// at both edges and just past the upper one. Tag characters are invisible in
// a terminal yet can spell an ASCII word, so a copied report could carry a
// hidden payload. Inputs use escapes because the runes are invisible in review.
func TestTagBlockIsDropped(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{"U+E0000 lower edge", "a\U000e0000b", "ab"},
		{"U+E0001 language tag", "a\U000e0001b", "ab"},
		{"U+E0041 tag latin capital A", "a\U000e0041b", "ab"},
		{"U+E007E", "a\U000e007eb", "ab"},
		{"U+E007F upper edge, cancel tag", "a\U000e007fb", "ab"},
		// "HI" spelled in tag characters, wrapped in visible text.
		{"hidden word", "visible\U000e0048\U000e0049text", "visibletext"},
		{"hidden word with cancel", "home.example\U000e0001\U000e0048\U000e0049\U000e007f.org", "home.example.org"},
		// First runes outside the block: the bound stops at 0xE007F.
		{"U+E0080 survives", "a\U000e0080b", "a\U000e0080b"},
		{"U+EFFFF survives", "a\U000effffb", "a\U000effffb"},
		{"U+DFFFF below the block survives", "a\U000dffffb", "a\U000dffffb"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Text(tt.in); got != tt.want {
				t.Fatalf("Text(%+q) = %+q (% x), want %+q (% x)", tt.in, got, got, tt.want, tt.want)
			}
		})
	}
}
