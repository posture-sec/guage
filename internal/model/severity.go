package model

type Severity int

const (
	SevInfo Severity = iota
	SevWarn
	SevHigh
)

// String returns the stable lowercase name used in output and sorting.
func (s Severity) String() string {
	switch s {
	case SevInfo:
		return "info"
	case SevWarn:
		return "warn"
	case SevHigh:
		return "high"
	default:
		return "unknown"
	}
}
