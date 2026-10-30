package sanitize

import "strings"

// Markdown escapes the characters that could start markup, after applying the
// same control-character stripping as Text.
func Markdown(s string) string {
	t := Text(s)
	var b strings.Builder
	b.Grow(len(t) + 8)
	for _, r := range t {
		switch r {
		case '\\', '*', '_', '[', ']', '<', '>', '#', '|', '~', '`', '&':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Fenced wraps s in a code fence longer than any backtick run inside it, so the
// content cannot close the block. Newlines are kept; control characters are not.
func Fenced(s string) string {
	body := stripControls(s)
	fence := strings.Repeat("`", fenceLen(body))
	return fence + "\n" + body + "\n" + fence
}

func fenceLen(body string) int {
	longest, run := 0, 0
	for _, r := range body {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
			continue
		}
		run = 0
	}
	if longest < 2 {
		return 3
	}
	return longest + 1
}
