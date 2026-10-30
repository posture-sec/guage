package sanitize

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzText(f *testing.F) {
	for _, seed := range []string{
		"",
		"example.com",
		"\x1b[31mred\x1b[0m",
		"a\u202eb\u200bc\x00d",
		strings.Repeat("x", MaxFieldLen+1),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := Text(s)
		if !utf8.ValidString(out) {
			t.Fatal("Text produced invalid UTF-8")
		}
		if utf8.RuneCountInString(out) > MaxFieldLen {
			t.Fatal("Text exceeded the field length cap")
		}
		for _, r := range out {
			if isDropped(r) {
				t.Fatalf("Text kept a dropped rune %U", r)
			}
		}
		if again := Text(out); again != out {
			t.Fatalf("Text is not idempotent: %q then %q", out, again)
		}
	})
}

func FuzzFenced(f *testing.F) {
	for _, seed := range []string{"", "evidence", "a```b", "\x1b[31mx"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := Fenced(s)
		if !utf8.ValidString(out) {
			t.Fatal("Fenced produced invalid UTF-8")
		}
		lines := strings.SplitN(out, "\n", 2)
		if len(lines) != 2 {
			t.Fatal("Fenced has no body")
		}
		fence := lines[0]
		if strings.Trim(fence, "`") != "" || len(fence) < 3 {
			t.Fatalf("bad fence %q", fence)
		}
		body := strings.TrimSuffix(lines[1], "\n"+fence)
		if body != stripControls(s) {
			t.Fatalf("Fenced altered the content: %q", body)
		}
		if strings.Contains(body, fence) {
			t.Fatal("fence appears inside the body")
		}
	})
}
