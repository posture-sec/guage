package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/sanitize"
)

// DiffText writes human-readable baseline diff output.
func DiffText(w io.Writer, d model.DiffReport) error {
	var b strings.Builder
	banner := d.Banner
	if banner == "" {
		banner = model.BaselineBanner
	}
	b.WriteString("=== BASELINE REVIEW BANNER ===\n")
	b.WriteString(banner)
	b.WriteString("\n==============================\n\n")

	fmt.Fprintf(&b, "Gauge %s (baseline diff)\n", sanitize.Text(d.ToolVersion))
	fmt.Fprintf(&b, "network: %s\n", sanitize.Text(d.Network))
	fmt.Fprintf(&b, "baseline ledger: %d | current ledger: %d\n", d.BaselineLedger, d.CurrentLedger)

	if !d.HasDiff {
		b.WriteString("\nall accounts match baseline (no drift detected)\n")
		b.WriteString("\n")
		b.WriteString(Limits)
		b.WriteString("\n")
		_, err := io.WriteString(w, b.String())
		return err
	}

	b.WriteString("\ndrift detected from baseline:\n")
	for _, acct := range d.Accounts {
		acctID := sanitize.Text(acct.AccountID)
		if acct.Missing {
			fmt.Fprintf(&b, "\naccount %s: missing on chain\n", acctID)
			continue
		}
		if acct.Unbaselined {
			fmt.Fprintf(&b, "\naccount %s: not present in baseline\n", acctID)
			continue
		}
		hasAcctDiff := len(acct.Thresholds) > 0 || len(acct.Signers) > 0 || len(acct.Flags) > 0 || acct.HomeDomain != nil
		if !hasAcctDiff {
			fmt.Fprintf(&b, "\naccount %s: in sync\n", acctID)
			continue
		}
		fmt.Fprintf(&b, "\naccount %s:\n", acctID)
		if len(acct.Thresholds) > 0 {
			b.WriteString("  thresholds:\n")
			for _, th := range acct.Thresholds {
				fmt.Fprintf(&b, "    %s: changed from %d to %d\n", sanitize.Text(th.Tier), th.Old, th.New)
			}
		}
		if len(acct.Signers) > 0 {
			b.WriteString("  signers:\n")
			for _, s := range acct.Signers {
				switch s.Kind {
				case "added":
					fmt.Fprintf(&b, "    + added: %s (weight: %d, type: %s)\n",
						sanitize.Text(s.Key), s.Weight, sanitize.Text(string(s.Type)))
				case "removed":
					fmt.Fprintf(&b, "    - removed: %s (weight: %d, type: %s)\n",
						sanitize.Text(s.Key), s.Weight, sanitize.Text(string(s.Type)))
				case "weight_changed":
					fmt.Fprintf(&b, "    ~ weight changed: %s from %d to %d (type: %s)\n",
						sanitize.Text(s.Key), s.OldWeight, s.Weight, sanitize.Text(string(s.Type)))
				}
			}
		}
		if len(acct.Flags) > 0 {
			b.WriteString("  flags:\n")
			for _, fl := range acct.Flags {
				fmt.Fprintf(&b, "    %s: changed from %v to %v\n", sanitize.Text(fl.Flag), fl.Old, fl.New)
			}
		}
		if acct.HomeDomain != nil {
			oldDom, _ := sanitize.Domain(acct.HomeDomain.Old)
			newDom, _ := sanitize.Domain(acct.HomeDomain.New)
			if oldDom == "" {
				oldDom = "(none)"
			}
			if newDom == "" {
				newDom = "(none)"
			}
			fmt.Fprintf(&b, "  home domain: changed from %s to %s\n", oldDom, newDom)
		}
	}

	b.WriteString("\n")
	b.WriteString(Limits)
	b.WriteString("\n")
	_, err := io.WriteString(w, b.String())
	return err
}

// DiffJSON writes indented JSON for the diff report.
func DiffJSON(w io.Writer, d model.DiffReport) error {
	document := struct {
		model.DiffReport
		Limits string `json:"limits"`
	}{
		DiffReport: d,
		Limits:     Limits,
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(document)
}

// DiffMarkdown writes GitHub-flavored Markdown for the diff report.
func DiffMarkdown(w io.Writer, d model.DiffReport) error {
	var b strings.Builder
	banner := d.Banner
	if banner == "" {
		banner = model.BaselineBanner
	}
	b.WriteString("### Baseline Comparison Report\n\n")
	b.WriteString("> **Warning (Baseline Review):** ")
	b.WriteString(sanitize.Markdown(banner))
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "- **Network:** `%s`\n", sanitize.Markdown(d.Network))
	fmt.Fprintf(&b, "- **Baseline Ledger:** `%d`\n", d.BaselineLedger)
	fmt.Fprintf(&b, "- **Current Ledger:** `%d`\n", d.CurrentLedger)

	if !d.HasDiff {
		b.WriteString("\n**Status:** All accounts match baseline (no drift detected).\n\n")
		b.WriteString("---\n\n*")
		b.WriteString(sanitize.Markdown(Limits))
		b.WriteString("*\n")
		_, err := io.WriteString(w, b.String())
		return err
	}

	b.WriteString("\n#### Drift Summary\n\n")
	for _, acct := range d.Accounts {
		acctID := sanitize.Markdown(acct.AccountID)
		if acct.Missing {
			fmt.Fprintf(&b, "##### Account `%s`\n\n- **Status:** Missing on chain\n\n", acctID)
			continue
		}
		if acct.Unbaselined {
			fmt.Fprintf(&b, "##### Account `%s`\n\n- **Status:** Not present in baseline\n\n", acctID)
			continue
		}
		hasAcctDiff := len(acct.Thresholds) > 0 || len(acct.Signers) > 0 || len(acct.Flags) > 0 || acct.HomeDomain != nil
		if !hasAcctDiff {
			fmt.Fprintf(&b, "##### Account `%s`\n\n- **Status:** In sync with baseline\n\n", acctID)
			continue
		}
		fmt.Fprintf(&b, "##### Account `%s`\n\n", acctID)
		if len(acct.Thresholds) > 0 {
			b.WriteString("- **Threshold Changes:**\n")
			for _, th := range acct.Thresholds {
				fmt.Fprintf(&b, "  - `%s`: changed from `%d` to `%d`\n", sanitize.Markdown(th.Tier), th.Old, th.New)
			}
		}
		if len(acct.Signers) > 0 {
			b.WriteString("- **Signer Changes:**\n")
			for _, s := range acct.Signers {
				switch s.Kind {
				case "added":
					fmt.Fprintf(&b, "  - **+ Added:** `%s` (weight: %d, type: `%s`)\n",
						sanitize.Markdown(s.Key), s.Weight, sanitize.Markdown(string(s.Type)))
				case "removed":
					fmt.Fprintf(&b, "  - **- Removed:** `%s` (weight: %d, type: `%s`)\n",
						sanitize.Markdown(s.Key), s.Weight, sanitize.Markdown(string(s.Type)))
				case "weight_changed":
					fmt.Fprintf(&b, "  - **~ Weight Changed:** `%s` from `%d` to `%d` (type: `%s`)\n",
						sanitize.Markdown(s.Key), s.OldWeight, s.Weight, sanitize.Markdown(string(s.Type)))
				}
			}
		}
		if len(acct.Flags) > 0 {
			b.WriteString("- **Flag Changes:**\n")
			for _, fl := range acct.Flags {
				fmt.Fprintf(&b, "  - `%s`: changed from `%v` to `%v`\n", sanitize.Markdown(fl.Flag), fl.Old, fl.New)
			}
		}
		if acct.HomeDomain != nil {
			oldDom, _ := sanitize.Domain(acct.HomeDomain.Old)
			newDom, _ := sanitize.Domain(acct.HomeDomain.New)
			fmt.Fprintf(&b, "- **Home Domain:** changed from `%s` to `%s`\n",
				sanitize.Markdown(oldDom), sanitize.Markdown(newDom))
		}
		b.WriteString("\n")
	}

	b.WriteString("---\n\n*")
	b.WriteString(sanitize.Markdown(Limits))
	b.WriteString("*\n")
	_, err := io.WriteString(w, b.String())
	return err
}
