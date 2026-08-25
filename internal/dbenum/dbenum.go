// Package dbenum provides int-backed enumeration types for GORM models.
//
// Design rules (see docs/development/repository-guidelines.md):
//   - Go layer uses typed int enums (iota) for compile-time safety.
//   - Database columns store small ints (fast to compare, cheap to index).
//   - REST/JSON layer keeps human-readable strings: enums implement
//     json.Marshaler/json.Unmarshaler so wire contracts stay unchanged.
//   - Zero value is always the default/initial state (e.g. pending/open).
//   - This package covers PLATFORM state machines only. Enums owned by game
//     contracts stay inside their JSON Schema ("enum": [...]) and must never
//     be converted to Go enums or DB constraints.
package dbenum

import (
	"database/sql/driver"
	"fmt"
)

// Enum is the constraint implemented by all dbenum types.
type Enum interface {
	Value() (driver.Value, error)
	Scan(src any) error
	String() string
}

// ScanInt scans a driver value into *int. Supported sources: int64, int,
// float64 (SQLite drivers may deliver ints as float), []byte and string
// (legacy rows before enum-ification), NULL and empty strings (map to 0 so
// empty legacy cells scan as the zero enum value).
func ScanInt(dst *int, src any) error {
	switch v := src.(type) {
	case nil:
		*dst = 0
	case int64:
		*dst = int(v)
	case int:
		*dst = v
	case float64:
		*dst = int(v)
	case []byte:
		if len(v) == 0 {
			*dst = 0
			return nil
		}
		var parsed int
		if _, err := fmt.Sscanf(string(v), "%d", &parsed); err != nil {
			return fmt.Errorf("dbenum: invalid int bytes %q: %w", string(v), err)
		}
		*dst = parsed
	case string:
		if v == "" {
			*dst = 0
			return nil
		}
		var parsed int
		if _, err := fmt.Sscanf(v, "%d", &parsed); err != nil {
			return fmt.Errorf("dbenum: invalid int string %q: %w", v, err)
		}
		*dst = parsed
	default:
		return fmt.Errorf("dbenum: unsupported scan source %T", src)
	}
	return nil
}

// ValueInt returns the int as a driver value.
func ValueInt(v int) (driver.Value, error) {
	return int64(v), nil
}
