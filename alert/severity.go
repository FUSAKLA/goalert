package alert

import (
	"database/sql/driver"
	"fmt"
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

func (s Severity) Value() (driver.Value, error) {
	str := string(s)
	if str == "" {
		str = string(SeverityInfo)
	}
	return str, nil
}

func (s *Severity) Scan(value interface{}) error {
	switch t := value.(type) {
	case []byte:
		*s = Severity(t)
	case string:
		*s = Severity(t)
	case nil:
		*s = SeverityInfo
	default:
		return fmt.Errorf("could not process unknown type for Severity(%T)", t)
	}
	return nil
}
