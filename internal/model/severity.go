package model

import (
	"encoding/json"
	"fmt"
)

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

// MarshalJSON writes the severity name, not its numeric value, so the report
// schema stays readable and stable if the constants are ever reordered.
func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON accepts only the three defined names.
func (s *Severity) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}
	switch name {
	case "info":
		*s = SevInfo
	case "warn":
		*s = SevWarn
	case "high":
		*s = SevHigh
	default:
		return fmt.Errorf("model: unknown severity %q", name)
	}
	return nil
}
