package sanitize

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// hostileCorpus is the shared input set every entry point has to survive. Each
// entry carries the payload between ordinary text, so a sanitizer that drops the
// payload together with its neighbours, or only at the edges, still shows up.
var hostileCorpus = []struct {
	name string
	in   string
}{
	{"osc 8 hyperlink", "a\x1b]8;;https://evil.example/\x1b\\click\x1b]8;;\x1b\\b"},
	{"csi colour", "a\x1b[31;1mred\x1b[0mb"},
	{"c1 csi U+009B", "a\u009b31mb"},
	{"rlo U+202E", "a\u202eexe.txtb"},
	{"zero width U+200B", "a\u200bb"},
	{"tag U+E0041", "a\U000E0041b"},
	{"nul", "a\x00b"},
	{"truncated utf-8", "a\xffb\xe2\x82"},
	{"backtick run", "a````b\n```c"},
}

// TestSanitizerEntryPointConformance holds Text, Markdown, Fenced and Domain to
// one contract over one corpus, so a renderer can pick any of them without the
// choice deciding whether a terminal escape reaches the reader. ARCHITECTURE.md
// 6.8 is the contract; the per-function tests above pin the exact outputs.
func TestSanitizerEntryPointConformance(t *testing.T) {
	t.Parallel()
	entryPoints := []struct {
		name string
		fn   func(string) string
	}{
		{"Text", Text},
		{"Markdown", Markdown},
		{"Fenced", Fenced},
		{"Domain", func(s string) string { d, _ := Domain(s); return d }},
	}
	for _, tt := range hostileCorpus {
		for _, ep := range entryPoints {
			t.Run(ep.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()
				out := ep.fn(tt.in)
				if !utf8.ValidString(out) {
					t.Fatalf("%s(%q) is not valid UTF-8: %q", ep.name, tt.in, out)
				}
				if strings.ContainsRune(out, 0x1b) {
					t.Fatalf("%s(%q) kept ESC: %q", ep.name, tt.in, out)
				}
				for _, r := range out {
					if isDropped(r) {
						t.Fatalf("%s(%q) kept dropped rune %U: %q", ep.name, tt.in, r, out)
					}
				}
			})
		}
		t.Run("Fenced structure/"+tt.name, func(t *testing.T) {
			t.Parallel()
			assertFenceShape(t, tt.in)
		})
	}
}

// assertFenceShape recomputes the fence from the body rather than calling
// fenceLen, so a bug in fenceLen cannot agree with itself.
func assertFenceShape(t *testing.T, in string) {
	t.Helper()
	body := stripControls(in)
	longest, run := 0, 0
	for _, r := range body {
		if r != '`' {
			run = 0
			continue
		}
		run++
		longest = max(longest, run)
	}
	n := max(3, longest+1)
	fence := strings.Repeat("`", n)

	out := Fenced(in)
	if want := fence + "\n" + body + "\n" + fence; out != want {
		t.Fatalf("Fenced(%q) = %q, want %q", in, out, want)
	}
	if n <= longest {
		t.Fatalf("fence of %d does not exceed the longest backtick run %d", n, longest)
	}
	lines := strings.Split(out, "\n")
	if got, want := len(lines), strings.Count(body, "\n")+3; got != want {
		t.Fatalf("Fenced(%q) has %d lines, want %d", in, got, want)
	}
	if lines[0] != fence || lines[len(lines)-1] != fence {
		t.Fatalf("Fenced(%q) does not open and close with the fence: %q", in, out)
	}
	for _, line := range lines[1 : len(lines)-1] {
		if strings.Contains(line, fence) {
			t.Fatalf("body line %q contains the fence", line)
		}
	}
}
