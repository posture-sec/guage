package model

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
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
			return a.Severity > b.Severity
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

func normalize(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
