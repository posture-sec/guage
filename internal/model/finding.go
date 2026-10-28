package model

type Finding struct {
	RuleID      string   `json:"rule_id"`
	Severity    Severity `json:"severity"`
	Account     string   `json:"account"`
	Message     string   `json:"message"`
	Evidence    string   `json:"evidence"`
	Fingerprint string   `json:"fingerprint"`
	Accepted    bool     `json:"accepted"`
}
