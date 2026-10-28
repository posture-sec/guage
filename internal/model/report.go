package model

type AccountReport struct {
	Snapshot AccountSnapshot `json:"snapshot"`
	Findings []Finding       `json:"findings"`
}

type Report struct {
	ToolVersion string          `json:"tool_version"`
	Network     string          `json:"network"`
	Ledger      int64           `json:"ledger"`
	Accounts    []AccountReport `json:"accounts"`
	ExitCode    ExitCode        `json:"exit_code"`
}
