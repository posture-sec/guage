package sanitize

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTextStripsControls(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{"ansi colour", "\x1b[31mred\x1b[0m", "red"},
		{"ansi cursor", "a\x1b[2Kb", "ab"},
		{"osc title", "a\x1b]0;pwned\x07b", "ab"},
		{"bare escape", "a\x1bb", "ab"},
		{"c0 and c1", "a\x00b\x07c\u0085d\u009fe", "abcde"},
		{"del", "a\u007fb", "ab"},
		{"bidi overrides", "vis\u202eeld\u202c", "viseld"},
		{"bidi isolates", "a\u2066b\u2069c", "abc"},
		{"bidi marks", "a\u200eb\u200fc\u061cd", "abcd"},
		{"zero width", "a\u200bb\u200cc\u200dd\u2060e\ufefff", "abcdef"},
		{"soft hyphen", "a\u00adb", "ab"},
		{"tag characters", "a\U000e0041b", "ab"},
		{"invalid utf8", "a\xffb", "ab"},
		{"line separators", "a\u2028b\u2029c", "abc"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Text(tt.in); got != tt.want {
				t.Fatalf("Text(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTextCollapsesWhitespace(t *testing.T) {
	t.Parallel()
	if got := Text("  a \n\t b   c  "); got != "a b c" {
		t.Fatalf("got %q", got)
	}
	if got := Text("a\u00a0b"); got != "a b" {
		t.Fatalf("non-breaking space not collapsed: %q", got)
	}
	if got := Text("\n\t "); got != "" {
		t.Fatalf("whitespace-only string survived: %q", got)
	}
}

func TestTextTruncates(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", MaxFieldLen+50)
	got := Text(long)
	if utf8.RuneCountInString(got) > MaxFieldLen {
		t.Fatalf("length %d exceeds the cap %d", utf8.RuneCountInString(got), MaxFieldLen)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatal("truncation is not visible")
	}
	if got := Text(strings.Repeat("ok", 10)); got != strings.Repeat("ok", 10) {
		t.Fatal("short string was altered")
	}
	for _, tt := range []struct {
		name string
		in   string
	}{
		{"multibyte", strings.Repeat("\u4e16", MaxFieldLen+5)},
		{"emoji", strings.Repeat("\U0001f600", MaxFieldLen+5)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cut := Text(tt.in)
			if !utf8.ValidString(cut) {
				t.Fatal("truncation split a rune")
			}
			if utf8.RuneCountInString(cut) > MaxFieldLen {
				t.Fatalf("length %d exceeds the cap", utf8.RuneCountInString(cut))
			}
		})
	}
}

func TestTextIsIdempotent(t *testing.T) {
	t.Parallel()
	in := "\x1b[1m  a\u202e b\u200b  \x07" + strings.Repeat("x", MaxFieldLen)
	once := Text(in)
	if twice := Text(once); twice != once {
		t.Fatalf("Text is not idempotent: %q then %q", once, twice)
	}
}

func TestDomainFlagsIDN(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		in      string
		want    string
		wantIDN bool
	}{
		{"ascii", "example.com", "example.com", false},
		{"escaped ascii", "exa\x1b[31mmple.com", "example.com", false},
		{"cyrillic lookalike", "ex\u0430mple.com", "ex\u0430mple.com", true},
		{"controls removed", "\u202eexample.com\u200b", "example.com", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, idn := Domain(tt.in)
			if got != tt.want || idn != tt.wantIDN {
				t.Fatalf("Domain(%q) = %q, %v; want %q, %v", tt.in, got, idn, tt.want, tt.wantIDN)
			}
		})
	}
}

func TestMarkdownEscapes(t *testing.T) {
	t.Parallel()
	got := Markdown("a*b_c[d]e<f>g#h|i&j`k\\l")
	want := "a\\*b\\_c\\[d\\]e\\<f\\>g\\#h\\|i\\&j\\`k\\\\l"
	if got != want {
		t.Fatalf("Markdown() = %q, want %q", got, want)
	}
	if got := Markdown("\x1b[31m**bold**\x1b[0m"); strings.ContainsRune(got, 0x1b) {
		t.Fatal("Markdown kept an escape character")
	}
}

func TestFenced(t *testing.T) {
	t.Parallel()
	plain := Fenced("evidence")
	if plain != "```\nevidence\n```" {
		t.Fatalf("Fenced() = %q", plain)
	}
	hostile := "a```b````c"
	got := Fenced(hostile)
	body := strings.TrimPrefix(got, "`````\n")
	body = strings.TrimSuffix(body, "\n`````")
	if body != hostile {
		t.Fatalf("Fenced altered the content: %q", body)
	}
	if got := Fenced("a\x1b[31mb\nc"); strings.ContainsRune(got, 0x1b) {
		t.Fatal("Fenced kept an escape character")
	}
	if got := Fenced("a\nb"); !strings.Contains(got, "a\nb") {
		t.Fatal("Fenced collapsed newlines")
	}
	if got := Fenced(""); got != "```\n\n```" {
		t.Fatalf("Fenced(empty) = %q", got)
	}
}
