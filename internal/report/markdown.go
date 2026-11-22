package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/sanitize"
)

// Markdown writes the report as GitHub-flavoured markdown, for a step summary or
// an issue body. It carries the same facts, findings and limits footer as Text,
// in the same order. Every chain-derived string goes through sanitize.Markdown,
// so a label carrying a link, an image or a tag reaches the page as the text it
// is, and evidence goes into a fence sized by sanitize.Fenced so it cannot close
// its own block. No chain-derived string ever starts a line: each one follows a
// label this file wrote, because a value at the start of a line could open a
// list, a heading or a quote even with every inline marker escaped.
func Markdown(w io.Writer, r model.Report) error {
	prepared := prepare(r)
	var b strings.Builder

	b.WriteString("# Gauge posture report\n\n")
	fmt.Fprintf(&b, "- tool: Gauge %s\n", md(prepared.ToolVersion))
	fmt.Fprintf(&b, "- network: %s\n", md(prepared.Network))
	fmt.Fprintf(&b, "- ledger: %d\n", prepared.Ledger)

	if len(prepared.Accounts) == 0 {
		b.WriteString("\nNo accounts were checked.\n")
	}
	for _, ar := range prepared.Accounts {
		writeMarkdownAccount(&b, ar)
	}

	high, warn, info, accepted := countFindings(prepared)
	b.WriteString("\n## Result\n\n")
	fmt.Fprintf(&b, "- findings: %d (%d high, %d warn, %d info), %d accepted\n",
		high+warn+info, high, warn, info, accepted)
	fmt.Fprintf(&b, "- exit code: %d\n", prepared.ExitCode)
	fmt.Fprintf(&b, "\n**Limits.** %s\n", Limits)

	_, err := io.WriteString(w, b.String())
	return err
}

// md escapes one already-prepared value. prepare has run sanitize.Text on it, so
// the truncation inside sanitize.Markdown is a no-op and a suffix this file adds
// afterwards, such as the IDN flag, is never cut off.
func md(s string) string {
	return sanitize.Markdown(s)
}

func mdOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return md(s)
}

func writeMarkdownAccount(b *strings.Builder, ar model.AccountReport) {
	s := ar.Snapshot
	fmt.Fprintf(b, "\n## Account %s\n\n", md(s.ID))
	fmt.Fprintf(b, "- endpoint: %s\n", mdOr(s.Endpoint, "(none)"))
	switch {
	case ar.Missing:
		b.WriteString("- state: not found on this endpoint (never created, or merged)\n")
	case ar.Skipped != "":
		fmt.Fprintf(b, "- state: no state read: %s\n", md(ar.Skipped))
	default:
		fmt.Fprintf(b, "- as of ledger %d, sequence %s\n", s.Ledger, mdOr(s.Sequence, "(none)"))
		fmt.Fprintf(b, "- thresholds: low %d, medium %d, high %d\n", s.Thresholds.Low, s.Thresholds.Medium, s.Thresholds.High)
		if facts := ar.Facts; facts != nil && facts.Reachability != nil {
			r := facts.Reachability
			fmt.Fprintf(b, "- minimum signers to authorize: low %s, medium %s, high %s\n",
				minimumText(r.Low), minimumText(r.Medium), minimumText(r.High))
		}
		fmt.Fprintf(b, "- flags: auth\\_required=%t auth\\_revocable=%t auth\\_immutable=%t clawback=%t\n",
			s.Flags.AuthRequired, s.Flags.AuthRevocable, s.Flags.AuthImmutable, s.Flags.ClawbackEnabled)
		fmt.Fprintf(b, "- home domain: %s\n", markdownDomain(s.HomeDomain))
		fmt.Fprintf(b, "- sponsor: %s\n", mdOr(s.Sponsor, "(none)"))
		writeMarkdownSigners(b, s.Signers)
	}

	writeMarkdownDeclared(b, ar.Facts)
	writeMarkdownIdentity(b, s, ar.Facts)
	writeMarkdownFindings(b, ar.Findings)
}

func markdownDomain(domain string) string {
	if domain == "" {
		return "(none)"
	}
	if _, idn := sanitize.Domain(domain); idn {
		return md(domain) + " (IDN)"
	}
	return md(domain)
}

func writeMarkdownSigners(b *strings.Builder, signers []model.Signer) {
	fmt.Fprintf(b, "\n### Signers (%d)\n\n", len(signers))
	if len(signers) == 0 {
		b.WriteString("None.\n")
		return
	}
	b.WriteString("| key | type | weight | sponsor |\n")
	b.WriteString("| --- | --- | ---: | --- |\n")
	for _, signer := range signers {
		fmt.Fprintf(b, "| %s | %s | %d | %s |\n",
			md(signer.Key), md(string(signer.Type)), signer.Weight, mdOr(signer.Sponsor, "(none)"))
	}
}

func writeMarkdownDeclared(b *strings.Builder, facts *model.AccountFacts) {
	if facts == nil {
		return
	}
	b.WriteString("\n### Declared\n\n")
	declared := facts.Declared
	if declared == nil {
		b.WriteString("Nothing: this run read no policy file, so only the universal rules ran.\n")
		return
	}
	roleStr := string(declared.Role)
	fmt.Fprintf(b, "- role: %s\n", md(roleStr))
	if declared.Name != "" {
		fmt.Fprintf(b, "- name: %s\n", markdownQuoted(&declared.Name, ""))
	}
	fmt.Fprintf(b, "- expected\\_signers: %s\n", signersText(declared.ExpectedSigners))
	fmt.Fprintf(b, "- expected\\_thresholds: %s\n", thresholdsText(declared.ExpectedThresholds))
	fmt.Fprintf(b, "- expected\\_flags: %s\n", md(flagsText(declared.ExpectedFlags)))
	fmt.Fprintf(b, "- home\\_domain: %s\n", markdownQuoted(declared.HomeDomain, notDeclared))
	switch {
	case declared.ExpectLocked == nil:
		fmt.Fprintf(b, "- expect\\_locked: %s\n", notDeclared)
	case declared.ExpectLockedIgnored:
		fmt.Fprintf(b, "- expect\\_locked: %t, which role %s is not allowed to declare; it is read for issuer alone\n",
			*declared.ExpectLocked, markdownQuoted(&roleStr, ""))
	default:
		fmt.Fprintf(b, "- expect\\_locked: %t\n", *declared.ExpectLocked)
	}
}

func writeMarkdownIdentity(b *strings.Builder, s model.AccountSnapshot, facts *model.AccountFacts) {
	if facts == nil {
		return
	}
	identity := facts.Identity
	if identity == nil && !s.HasHomeDomain() {
		return
	}
	b.WriteString("\n### Identity document\n\n")
	switch {
	case identity == nil:
		b.WriteString("Never fetched, so the published home domain is unverified.\n")
		return
	case identity.Reachable:
		fmt.Fprintf(b, "- result: %s served a readable file\n", md(identity.HomeDomain))
	default:
		fmt.Fprintf(b, "- result: %s did not serve a readable file\n", md(identity.HomeDomain))
	}
	fmt.Fprintf(b, "- step: %s\n", md(identity.Step))
	if identity.Detail != "" {
		fmt.Fprintf(b, "- detail: %s\n", md(identity.Detail))
	}
	if identity.Status == 0 {
		b.WriteString("- fetch: no response received\n")
	} else {
		cors := "absent"
		if identity.CORSPresent {
			cors = "present"
		}
		fmt.Fprintf(b, "- fetch: status %d, %d redirect hops, %d bytes, CORS %s\n",
			identity.Status, identity.Hops, identity.Bytes, cors)
	}
	if !identity.Reachable {
		return
	}
	network := markdownQuoted(identity.NetworkPassphrase, notNamed)
	if matches := identity.NetworkMatches; matches != nil {
		if *matches {
			network += " (this network)"
		} else {
			network += " (not this network)"
		}
	}
	fmt.Fprintf(b, "- network\\_passphrase: %s\n", network)
	fmt.Fprintf(b, "- signing\\_key: %s\n", markdownQuoted(identity.SigningKey, notNamed))
	fmt.Fprintf(b, "- horizon\\_url: %s\n", markdownQuoted(identity.HorizonURL, notNamed))
	fmt.Fprintf(b, "- version: %s\n", markdownQuoted(identity.Version, notNamed))
	if malformed := identity.MalformedAccounts; malformed > 0 {
		fmt.Fprintf(b, "- accounts: %d listed, %d rejected as not an address\n", len(identity.Accounts), malformed)
		return
	}
	fmt.Fprintf(b, "- accounts: %d listed\n", len(identity.Accounts))
}

// markdownQuoted is quotedText for markdown. The quotes go around the escaped
// value rather than through strconv.Quote, because a quoted string near the field
// limit would be truncated by sanitize.Markdown and lose its closing quote.
func markdownQuoted(value *string, whenMissing string) string {
	if value == nil {
		return whenMissing
	}
	return `"` + md(*value) + `"`
}

func writeMarkdownFindings(b *strings.Builder, findings []model.Finding) {
	accepted := 0
	for _, f := range findings {
		if f.Accepted {
			accepted++
		}
	}
	if len(findings) == 0 {
		b.WriteString("\n### Findings\n\nNone.\n")
		return
	}
	fmt.Fprintf(b, "\n### Findings: %d (%d accepted)\n", len(findings), accepted)
	for _, f := range findings {
		marker := ""
		if f.Accepted {
			marker = " (accepted)"
		}
		// The rule ID and the severity are this program's own names, but they pass
		// through the escaper anyway: a renderer does not get to decide which
		// fields are trusted.
		fmt.Fprintf(b, "\n#### \\[%s\\] %s%s\n\n", md(f.Severity.String()), md(f.RuleID), marker)
		fmt.Fprintf(b, "- message: %s\n", md(f.Message))
		fmt.Fprintf(b, "- fingerprint: %s\n", md(f.Fingerprint))
		if f.Evidence != "" {
			b.WriteString("\nEvidence:\n\n")
			b.WriteString(sanitize.Fenced(f.Evidence))
			b.WriteString("\n")
		}
	}
}
