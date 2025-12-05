package alert

import (
	"database/sql/driver"
	"fmt"
	"slices"
)

// Severity is the severity level of an Alert.
type Severity string

// Alert severity types
const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

var (
	KnownSeverities = []Severity{
		SeverityInfo,
		SeverityWarning,
		SeverityHigh,
		SeverityCritical,
	}
)

func (s Severity) Value() (driver.Value, error) {
	str := string(s)
	if str == "" {
		str = string(SeverityInfo)
	}
	return str, nil
}

func (s Severity) Valid() bool {
	return slices.Contains(KnownSeverities, s)
}

func (s *Severity) Scan(value interface{}) error {
	switch t := value.(type) {
	case []byte:
		*s = Severity(t)
	case string:
		*s = Severity(t)
	case nil:
		*s = SeverityCritical
	default:
		return fmt.Errorf("could not process unknown type for Severity(%T)", t)
	}
	if !s.Valid() {
		return fmt.Errorf("invalid severity: %s", *s)
	}
	return nil
}
