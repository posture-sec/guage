package model

// AccountReport is the result for one requested account. Missing marks an
// account the endpoint reported as not found; Skipped explains why no state
// was read at all, as when the endpoint is on another network. Both are
// omitted from JSON when empty, so a normal report is unchanged.
type AccountReport struct {
	Snapshot AccountSnapshot `json:"snapshot"`
	Findings []Finding       `json:"findings"`
	Missing  bool            `json:"missing,omitempty"`
	Skipped  string          `json:"skipped,omitempty"`
}

type Report struct {
	ToolVersion string          `json:"tool_version"`
	Network     string          `json:"network"`
	Ledger      int64           `json:"ledger"`
	Accounts    []AccountReport `json:"accounts"`
	ExitCode    ExitCode        `json:"exit_code"`
}
