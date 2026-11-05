package report

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/sanitize"
)

// Limits is the footer every report carries. It names what a posture report
// cannot see, so a clean result is not mistaken for a guarantee.
const Limits = "Gauge reads public ledger state only. It cannot see who holds a key, whether signers are separate people, or off-chain controls. A clean report is a posture result, not proof that the account cannot be compromised."

// The three ways a printed value can be missing, kept apart because the report is
// where an operator learns which of them applied: a policy line nobody wrote, a
// declared block that names one field of several, and a document that parsed
// without naming a key SEP-1 defines.
const (
	notDeclared = "(not declared)"
	notStated   = "(not stated)"
	notNamed    = "(not named)"
)

// Text writes the human-readable report. Findings are ordered by severity,
// then account, then rule; chain-derived strings are sanitized first.
func Text(w io.Writer, r model.Report) error {
	prepared := prepare(r)
	var b strings.Builder

	fmt.Fprintf(&b, "Gauge %s\n", sanitize.Text(prepared.ToolVersion))
	fmt.Fprintf(&b, "network: %s\n", sanitize.Text(prepared.Network))
	fmt.Fprintf(&b, "ledger: %d\n", prepared.Ledger)

	if len(prepared.Accounts) == 0 {
		b.WriteString("\nno accounts were checked\n")
	}
	for _, ar := range prepared.Accounts {
		writeAccount(&b, ar)
	}

	high, warn, info, accepted := countFindings(prepared)
	fmt.Fprintf(&b, "\nresult: %d findings (%d high, %d warn, %d info), %d accepted\n",
		high+warn+info, high, warn, info, accepted)
	fmt.Fprintf(&b, "exit code: %d\n", prepared.ExitCode)
	fmt.Fprintf(&b, "limits: %s\n", Limits)

	_, err := io.WriteString(w, b.String())
	return err
}

func writeAccount(b *strings.Builder, ar model.AccountReport) {
	s := ar.Snapshot
	fmt.Fprintf(b, "\naccount %s\n", s.ID)
	fmt.Fprintf(b, "  endpoint: %s\n", emptyAs(s.Endpoint, "(none)"))
	switch {
	case ar.Missing:
		b.WriteString("  not found on this endpoint (never created, or merged)\n")
	case ar.Skipped != "":
		fmt.Fprintf(b, "  no state read: %s\n", ar.Skipped)
	default:
		fmt.Fprintf(b, "  as of ledger %d, sequence %s\n", s.Ledger, emptyAs(s.Sequence, "(none)"))
		fmt.Fprintf(b, "  thresholds: low %d, medium %d, high %d\n", s.Thresholds.Low, s.Thresholds.Medium, s.Thresholds.High)
		writeReachability(b, ar.Facts)
		fmt.Fprintf(b, "  flags: auth_required=%t auth_revocable=%t auth_immutable=%t clawback=%t\n",
			s.Flags.AuthRequired, s.Flags.AuthRevocable, s.Flags.AuthImmutable, s.Flags.ClawbackEnabled)
		fmt.Fprintf(b, "  home domain: %s\n", describedDomain(s.HomeDomain))
		fmt.Fprintf(b, "  sponsor: %s\n", emptyAs(s.Sponsor, "(none)"))
		fmt.Fprintf(b, "  signers: %d\n", len(s.Signers))
		for _, signer := range s.Signers {
			fmt.Fprintf(b, "    %s %s weight %d", signer.Key, signer.Type, signer.Weight)
			if signer.Sponsor != "" {
				fmt.Fprintf(b, " sponsored by %s", signer.Sponsor)
			}
			b.WriteString("\n")
		}
	}

	writeDeclared(b, ar.Facts)
	writeIdentity(b, s, ar.Facts)

	accepted := 0
	for _, f := range ar.Findings {
		if f.Accepted {
			accepted++
		}
	}
	if len(ar.Findings) == 0 {
		b.WriteString("  findings: none\n")
		return
	}
	fmt.Fprintf(b, "  findings: %d (%d accepted)\n", len(ar.Findings), accepted)
	for _, f := range ar.Findings {
		marker := ""
		if f.Accepted {
			marker = " (accepted)"
		}
		fmt.Fprintf(b, "    [%s] %s%s\n", f.Severity, f.RuleID, marker)
		fmt.Fprintf(b, "      %s\n", f.Message)
		if f.Evidence != "" {
			fmt.Fprintf(b, "      evidence: %s\n", f.Evidence)
		}
		fmt.Fprintf(b, "      fingerprint: %s\n", f.Fingerprint)
	}
}

func countFindings(r model.Report) (high, warn, info, accepted int) {
	for _, ar := range r.Accounts {
		for _, f := range ar.Findings {
			if f.Accepted {
				accepted++
			}
			switch f.Severity {
			case model.SevHigh:
				high++
			case model.SevWarn:
				warn++
			case model.SevInfo:
				info++
			}
		}
	}
	return high, warn, info, accepted
}

func describedDomain(domain string) string {
	if domain == "" {
		return "(none)"
	}
	if _, idn := sanitize.Domain(domain); idn {
		return domain + " (IDN)"
	}
	return domain
}

// writeReachability prints how many of the account's own signers a tier needs.
// The line sits beside the thresholds it is computed from, and a tier no set can
// reach is printed because a report that named only the working tiers would
// describe an account as working.
func writeReachability(b *strings.Builder, facts *model.AccountFacts) {
	if facts == nil || facts.Reachability == nil {
		return
	}
	r := facts.Reachability
	fmt.Fprintf(b, "  minimum signers to authorize: low %s, medium %s, high %s\n",
		minimumText(r.Low), minimumText(r.Medium), minimumText(r.High))
}

func minimumText(minimum *int) string {
	if minimum == nil {
		return "(unreachable)"
	}
	return strconv.Itoa(*minimum)
}

// writeDeclared prints the expectations the policy file carried, under the names
// the file itself uses so a report line maps to a config line. An undeclared entry
// is printed as undeclared rather than left out, because the drift rules are silent
// either way and only this block tells an operator which half of the file Gauge read.
func writeDeclared(b *strings.Builder, facts *model.AccountFacts) {
	// A report with no fact block at all says nothing about declarations: the run
	// that built it never assembled one, so there is no answer to print.
	if facts == nil {
		return
	}
	declared := facts.Declared
	if declared == nil {
		b.WriteString("  declared: nothing; this run read no policy file, so only the universal rules ran\n")
		return
	}
	header := "  declared: role " + declared.Role
	if name := declared.Name; name != "" {
		header += fmt.Sprintf(", name %q", name)
	}
	b.WriteString(header + "\n")

	fmt.Fprintf(b, "    expected_signers: %s\n", signersText(declared.ExpectedSigners))
	fmt.Fprintf(b, "    expected_thresholds: %s\n", thresholdsText(declared.ExpectedThresholds))
	fmt.Fprintf(b, "    expected_flags: %s\n", flagsText(declared.ExpectedFlags))
	fmt.Fprintf(b, "    home_domain: %s\n", quotedText(declared.HomeDomain, notDeclared))
	if declared.ExpectLocked == nil {
		fmt.Fprintf(b, "    expect_locked: %s\n", notDeclared)
		return
	}
	if declared.ExpectLockedIgnored {
		fmt.Fprintf(b, "    expect_locked: %t, which role %q is not allowed to declare; it is read for issuer alone\n",
			*declared.ExpectLocked, declared.Role)
		return
	}
	fmt.Fprintf(b, "    expect_locked: %t\n", *declared.ExpectLocked)
}

func signersText(count *int) string {
	if count == nil {
		return notDeclared
	}
	if *count == 1 {
		return "1 key declared"
	}
	return strconv.Itoa(*count) + " keys declared"
}

func thresholdsText(expected *model.DeclaredThresholds) string {
	if expected == nil {
		return notDeclared
	}
	return fmt.Sprintf("low %s, medium %s, high %s",
		numberText(expected.Low), numberText(expected.Medium), numberText(expected.High))
}

func flagsText(expected *model.DeclaredFlags) string {
	if expected == nil {
		return notDeclared
	}
	return fmt.Sprintf("auth_required=%s auth_revocable=%s auth_immutable=%s clawback=%s",
		booleanText(expected.AuthRequired), booleanText(expected.AuthRevocable),
		booleanText(expected.AuthImmutable), booleanText(expected.ClawbackEnabled))
}

func numberText(value *int) string {
	if value == nil {
		return notStated
	}
	return strconv.Itoa(*value)
}

func booleanText(value *bool) string {
	if value == nil {
		return notStated
	}
	return strconv.FormatBool(*value)
}

// writeIdentity prints what the guarded fetcher made of the account's home domain.
// A document that never parsed reports the step that decided it and nothing the
// parser would have filled in, so an unread file is never mistaken for a file that
// stated no keys.
func writeIdentity(b *strings.Builder, s model.AccountSnapshot, facts *model.AccountFacts) {
	if facts == nil {
		return
	}
	identity := facts.Identity
	switch {
	case identity == nil && !s.HasHomeDomain():
		// The home domain line above already says the account publishes nothing,
		// so there is no identity claim for this run to report on.
	case identity == nil:
		b.WriteString("  identity document: never fetched, so the published home domain is unverified\n")
	case identity.Reachable:
		fmt.Fprintf(b, "  identity document: %s served a readable file\n", identity.HomeDomain)
	default:
		fmt.Fprintf(b, "  identity document: %s did not serve a readable file\n", identity.HomeDomain)
	}
	if identity == nil {
		return
	}
	fmt.Fprintf(b, "    step: %s\n", identity.Step)
	if identity.Detail != "" {
		fmt.Fprintf(b, "    detail: %s\n", identity.Detail)
	}
	if identity.Status == 0 {
		b.WriteString("    fetch: no response received\n")
	} else {
		cors := "absent"
		if identity.CORSPresent {
			cors = "present"
		}
		fmt.Fprintf(b, "    fetch: status %d, %d redirect hops, %d bytes, CORS %s\n",
			identity.Status, identity.Hops, identity.Bytes, cors)
	}
	if !identity.Reachable {
		return
	}
	network := quotedText(identity.NetworkPassphrase, notNamed)
	if matches := identity.NetworkMatches; matches != nil {
		agreement := " (not this network)"
		if *matches {
			agreement = " (this network)"
		}
		network += agreement
	}
	fmt.Fprintf(b, "    network_passphrase: %s\n", network)
	fmt.Fprintf(b, "    signing_key: %s\n", quotedText(identity.SigningKey, notNamed))
	fmt.Fprintf(b, "    horizon_url: %s\n", quotedText(identity.HorizonURL, notNamed))
	fmt.Fprintf(b, "    version: %s\n", quotedText(identity.Version, notNamed))
	if malformed := identity.MalformedAccounts; malformed > 0 {
		fmt.Fprintf(b, "    accounts: %d listed, %d rejected as not an address\n",
			len(identity.Accounts), malformed)
		return
	}
	fmt.Fprintf(b, "    accounts: %d listed\n", len(identity.Accounts))
}

// quotedText prints a value an operator or a document supplied, or why it is
// missing. An absent pointer is not the same as an empty string, and the wording
// says which kind of absence this is: a policy line nobody wrote, or a key the
// document does not name.
func quotedText(value *string, whenMissing string) string {
	if value == nil {
		return whenMissing
	}
	return strconv.Quote(*value)
}

func emptyAs(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
