package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

func TestMarkdownGolden(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Markdown(&buf, sampleReport()); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	checkGolden(t, "sample.markdown.golden", buf.Bytes())
}

// hostileMarkup builds markup payloads from fragments, so the test file never
// carries a ready-made link, image or script tag as one literal. Each payload is
// one a chain-derived label could carry into a GitHub step summary.
func hostileMarkup() (link, image, script, fence string) {
	open, close := "[", "]"
	link = open + "click" + close + "(" + "https" + "://evil.invalid/x)"
	image = "!" + open + "x" + close + "(" + "https" + "://evil.invalid/p.png)"
	script = "<" + "script" + ">alert(1)<" + "/script" + ">"
	tick := "`"
	fence = strings.Repeat(tick, 3) + "\n# heading\n" + strings.Repeat(tick, 5)
	return link, image, script, fence
}

func hostileMarkdownReport() model.Report {
	link, image, script, fence := hostileMarkup()
	r := sampleReport()
	ar := &r.Accounts[0]
	ar.Facts.Declared.Name = link
	ar.Facts.Identity.Version = value(image)
	ar.Snapshot.Sponsor = script
	ar.Snapshot.HomeDomain = "a|b" + script
	ar.Findings[0].Message = "- " + link
	ar.Findings[0].Evidence = fence
	return r
}

func renderMarkdown(t *testing.T, r model.Report) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Markdown(&buf, r); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	return buf.String()
}

func TestMarkdownRendersMarkupAsLiteralText(t *testing.T) {
	t.Parallel()
	out := renderMarkdown(t, hostileMarkdownReport())
	link, image, script, _ := hostileMarkup()

	// None of the payloads may survive as markup.
	for name, raw := range map[string]string{"link": link, "image": image, "script": script} {
		if strings.Contains(out, raw) {
			t.Errorf("markdown output kept the %s payload unescaped", name)
		}
	}
	opener := "<" + "script"
	if strings.Count(out, opener) != strings.Count(out, `\`+opener) {
		t.Error("markdown output kept a raw tag opener")
	}
	// Each payload is still there, as escaped text a reader can see.
	for _, want := range []string{
		`\[click\](https://evil.invalid/x)`,
		`!\[x\](https://evil.invalid/p.png)`,
		`\<script\>alert(1)\</script\>`,
		`a\|b`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown output does not carry escaped %q", want)
		}
	}
	// A message that starts like a list item must not start a line.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "- \\[click") {
			t.Errorf("a chain-derived message opened a line: %q", line)
		}
	}
}

func TestMarkdownFencesEvidenceVerbatim(t *testing.T) {
	t.Parallel()
	out := renderMarkdown(t, hostileMarkdownReport())
	tick := "`"
	// The evidence holds a run of five backticks, so its fence must be six long
	// and the evidence must sit inside it unescaped and whole. The sanitizer
	// collapses the newlines first, which is the same evidence the text report
	// prints.
	fence := strings.Repeat(tick, 6)
	body := strings.Repeat(tick, 3) + " # heading " + strings.Repeat(tick, 5)
	want := "\n" + fence + "\n" + body + "\n" + fence + "\n"
	if !strings.Contains(out, want) {
		t.Fatalf("evidence is not fenced verbatim; want block %q in:\n%s", want, out)
	}
	if strings.Contains(out, "\n# heading") {
		t.Fatal("evidence escaped its fence and opened a heading")
	}
}

func TestMarkdownStripsHostileBytes(t *testing.T) {
	t.Parallel()
	out := renderMarkdown(t, sampleReport())
	for _, r := range []rune{0x1b, 0x07, 0x202e, 0x200b} {
		if strings.ContainsRune(out, r) {
			t.Fatalf("markdown output kept U+%04X", r)
		}
	}
	if strings.Contains(out, "title") {
		t.Fatal("markdown output kept an OSC payload")
	}
	if !strings.Contains(out, "(IDN)") {
		t.Fatal("markdown output does not flag the IDN home domain")
	}
}

func TestMarkdownSaysPostureAndEndsWithTheLimits(t *testing.T) {
	t.Parallel()
	for name, r := range map[string]model.Report{
		"sample": sampleReport(),
		"empty":  {ToolVersion: "0.0.0-test", Network: "Test SDF Network ; September 2015"},
	} {
		out := renderMarkdown(t, r)
		lower := strings.ToLower(out)
		if strings.Contains(lower, "secure") || strings.Contains(lower, "safe") {
			t.Errorf("%s: markdown output uses forbidden wording", name)
		}
		if !strings.Contains(out, "posture") {
			t.Errorf("%s: markdown output does not say posture", name)
		}
		if !strings.HasSuffix(out, Limits+"\n") {
			t.Errorf("%s: markdown output does not end with the limits footer", name)
		}
	}
}

func TestMarkdownEmptyReportSaysNothingWasChecked(t *testing.T) {
	t.Parallel()
	out := renderMarkdown(t, model.Report{ToolVersion: "0.0.0-test", Network: "testnet"})
	if !strings.Contains(out, "No accounts were checked.") {
		t.Fatalf("empty markdown report does not say so:\n%s", out)
	}
}

func TestMarkdownIsDeterministicAndDoesNotMutate(t *testing.T) {
	t.Parallel()
	input := hostileMarkdownReport()
	first := renderMarkdown(t, input)
	second := renderMarkdown(t, input)
	if first != second {
		t.Fatal("markdown output changed between identical runs")
	}
	fresh := hostileMarkdownReport()
	if input.Accounts[0].Snapshot.Sponsor != fresh.Accounts[0].Snapshot.Sponsor ||
		input.Accounts[0].Findings[0].Evidence != fresh.Accounts[0].Findings[0].Evidence ||
		input.Accounts[0].Findings[0].RuleID != fresh.Accounts[0].Findings[0].RuleID {
		t.Fatal("Markdown changed the report it was given")
	}
}
