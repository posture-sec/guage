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

// Valid reports whether the value is one of the three defined severities.
func (s Severity) Valid() bool {
	return s == SevInfo || s == SevWarn || s == SevHigh
}

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
// schema stays readable and stable if the constants are ever reordered. An
// undefined value is refused rather than written as a name.
func (s Severity) MarshalJSON() ([]byte, error) {
	if !s.Valid() {
		return nil, fmt.Errorf("model: severity %d is outside the defined range", int(s))
	}
	return json.Marshal(s.String())
}

func (s Severity) rank() int {
	if !s.Valid() {
		return -1
	}
	return int(s)
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
