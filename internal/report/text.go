package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/sanitize"
)

// Limits is the footer every report carries. It names what a posture report
// cannot see, so a clean result is not mistaken for a guarantee.
const Limits = "Gauge reads public ledger state only. It cannot see who holds a key, whether signers are separate people, or off-chain controls. A clean report is a posture result, not proof that the account cannot be compromised."

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
	fmt.Fprintf(b, "  as of ledger %d, sequence %s\n", s.Ledger, emptyAs(s.Sequence, "(none)"))
	fmt.Fprintf(b, "  thresholds: low %d, medium %d, high %d\n", s.Thresholds.Low, s.Thresholds.Medium, s.Thresholds.High)
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

func emptyAs(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
