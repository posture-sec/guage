package sanitize

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxFieldLen is the longest single field any renderer prints. Chain-derived
// strings are truncated to it so a hostile home domain cannot flood a report.
const MaxFieldLen = 256

// Text removes control characters, terminal escape sequences, bidirectional
// overrides and zero-width characters, collapses whitespace and truncates to
// MaxFieldLen. It is safe to apply twice.
func Text(s string) string {
	return truncate(collapseSpace(stripControls(s)), MaxFieldLen)
}

func stripControls(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}
		if r == 0x1b {
			i += escapeLen(s[i:])
			continue
		}
		if isDropped(r) {
			i += size
			continue
		}
		b.WriteString(s[i : i+size])
		i += size
	}
	return b.String()
}

// escapeLen returns the length of the escape sequence at the start of s, which
// always begins with ESC. It consumes CSI and OSC sequences whole so their
// parameter bytes do not survive as visible text.
func escapeLen(s string) int {
	if len(s) < 2 {
		return len(s)
	}
	switch s[1] {
	case '[':
		for i := 2; i < len(s); i++ {
			if s[i] >= 0x40 && s[i] <= 0x7e {
				return i + 1
			}
		}
		return len(s)
	case ']':
		for i := 2; i < len(s); i++ {
			if s[i] == 0x07 {
				return i + 1
			}
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2
			}
		}
		return len(s)
	default:
		return 1
	}
}

func isDropped(r rune) bool {
	switch {
	case r < 0x20:
		// Tabs and line breaks are whitespace, not payload: Text collapses
		// them and Fenced keeps them.
		return r != '\t' && r != '\n' && r != '\v' && r != '\f' && r != '\r'
	case r == 0x7f, r >= 0x80 && r <= 0x9f:
		return true
	case r == 0x00ad:
		return true
	case r == 0x061c, r == 0x200e, r == 0x200f:
		return true
	case r >= 0x202a && r <= 0x202e:
		return true
	case r == 0x2028, r == 0x2029:
		return true
	case r == 0x200b, r == 0x200c, r == 0x200d, r == 0x2060, r == 0xfeff:
		return true
	case r >= 0x2066 && r <= 0x2069:
		return true
	case r >= 0xe0000 && r <= 0xe007f:
		return true
	}
	return false
}

func collapseSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = b.Len() > 0
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	keep := max - 3
	n := 0
	for i := range s {
		if n == keep {
			return s[:i] + "..."
		}
		n++
	}
	return s
}
