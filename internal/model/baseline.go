package model

// BaselineBanner is the review banner included in every baseline file and diff,
// warning operators that recorded state is an observation, not an approval of security.
const BaselineBanner = "A baseline is a record of state, not an approval of it. Review every threshold and signer before adopting this baseline."

// Baseline represents a saved state snapshot for one or more accounts.
type Baseline struct {
	Version     int               `json:"version"`
	Banner      string            `json:"banner"`
	ToolVersion string            `json:"tool_version,omitempty"`
	Network     string            `json:"network"`
	Ledger      int64             `json:"ledger"`
	Accounts    []AccountSnapshot `json:"accounts"`
}

// SignerDiff captures a change in signers between a baseline and live state.
type SignerDiff struct {
	Key       string     `json:"key"`
	Type      SignerType `json:"type"`
	Weight    int        `json:"weight"`
	OldWeight int        `json:"old_weight,omitempty"`
	Kind      string     `json:"kind"` // "added", "removed", "weight_changed"
}

// ThresholdDiff captures a change in account thresholds.
type ThresholdDiff struct {
	Tier string `json:"tier"` // "low", "medium", "high"
	Old  int    `json:"old"`
	New  int    `json:"new"`
}

// FlagDiff captures a change in account flags.
type FlagDiff struct {
	Flag string `json:"flag"`
	Old  bool   `json:"old"`
	New  bool   `json:"new"`
}

// DomainDiff captures a change in home domain.
type DomainDiff struct {
	Old string `json:"old"`
	New string `json:"new"`
}

// AccountDiff records all differences found for a single account.
type AccountDiff struct {
	AccountID   string          `json:"account_id"`
	Missing     bool            `json:"missing,omitempty"`
	Unbaselined bool            `json:"unbaselined,omitempty"`
	Thresholds  []ThresholdDiff `json:"thresholds,omitempty"`
	Signers     []SignerDiff    `json:"signers,omitempty"`
	Flags       []FlagDiff      `json:"flags,omitempty"`
	HomeDomain  *DomainDiff     `json:"home_domain,omitempty"`
}

// DiffReport summarizes comparison of live state against a baseline.
type DiffReport struct {
	Banner         string        `json:"banner"`
	ToolVersion    string        `json:"tool_version"`
	Network        string        `json:"network"`
	BaselineLedger int64         `json:"baseline_ledger"`
	CurrentLedger  int64         `json:"current_ledger"`
	Accounts       []AccountDiff `json:"accounts"`
	HasDiff        bool          `json:"has_diff"`
	ExitCode       ExitCode      `json:"exit_code"`
}
