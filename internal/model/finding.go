package model

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"unicode"
)

type Finding struct {
	RuleID      string   `json:"rule_id"`
	Severity    Severity `json:"severity"`
	Account     string   `json:"account"`
	Message     string   `json:"message"`
	Evidence    string   `json:"evidence"`
	Fingerprint string   `json:"fingerprint"`
	Accepted    bool     `json:"accepted"`
}

// NewFinding builds a finding and computes its fingerprint from the rule, the
// account and the evidence with all whitespace runs collapsed. The fingerprint
// is what accepted-risk entries key on, so it must not move when only spacing
// changes.
func NewFinding(ruleID string, severity Severity, account, message, evidence string) Finding {
	return Finding{
		RuleID:      ruleID,
		Severity:    severity,
		Account:     account,
		Message:     message,
		Evidence:    evidence,
		Fingerprint: fingerprint(ruleID, account, evidence),
	}
}

// SortFindings orders findings by severity, highest first, then account, then
// rule ID. Equal keys keep the order they were found in so output is stable.
func SortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Severity != b.Severity {
			return a.Severity.rank() > b.Severity.rank()
		}
		if a.Account != b.Account {
			return a.Account < b.Account
		}
		return a.RuleID < b.RuleID
	})
}

func fingerprint(ruleID, account, evidence string) string {
	sum := sha256.Sum256([]byte(ruleID + "\x00" + account + "\x00" + normalize(evidence)))
	return hex.EncodeToString(sum[:])
}

// normalize collapses whitespace and drops Unicode format characters (category
// Cf). NewFinding hashes this form, and sanitize.Text removes the same class
// before any renderer prints evidence, so evidence that prints the same hashes
// the same. sanitize.Text drops more than Cf, including whole escape sequences,
// C0 and C1 controls and the line and paragraph separators; that wider set is
// not repeated here, because model may not import sanitize and a second copy of
// its drop list would be one more rule to keep in step.
func normalize(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}
