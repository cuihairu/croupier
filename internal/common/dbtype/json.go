// Package dbtype provides dialect-aware database column types. Leaf package:
// depends only on gorm; importing any internal/ business package is forbidden
// (there is a dependency-cycle risk between model and platform/registry, and
// both must be able to use these types).
package dbtype

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"

	gmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// JSON is a dialect-aware replacement for datatypes.JSON: the semantics are
// aligned method by method (Value returns nil for empty values, Scan of nil
// writes "null", MarshalJSON outputs the raw document instead of base64). The
// only difference is that GormDBDataType fills in the sqlserver branch —
// datatypes.JSON returns an empty string for unknown dialects, after which the
// gorm sqlserver driver falls back to the literal `json`; SQL Server has no
// native JSON type, so AutoMigrate table creation fails outright with Cannot
// find data type json. Under sqlserver, JSON documents are stored as
// nvarchar(max) (UTF-16, lossless).
type JSON json.RawMessage

// Value implements driver.Valuer.
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

// Scan implements sql.Scanner.
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = JSON("null")
		return nil
	}
	var bytes []byte
	if s, ok := value.(fmt.Stringer); ok {
		bytes = []byte(s.String())
	} else {
		switch v := value.(type) {
		case []byte:
			if len(v) > 0 {
				bytes = make([]byte, len(v))
				copy(bytes, v)
			}
		case string:
			bytes = []byte(v)
		default:
			return fmt.Errorf("failed to unmarshal JSONB value: %v", value)
		}
	}
	*j = JSON(json.RawMessage(bytes))
	return nil
}

// MarshalJSON outputs the raw document instead of base64.
func (j JSON) MarshalJSON() ([]byte, error) {
	return json.RawMessage(j).MarshalJSON()
}

// unmarshalRawMessage is the test seam for json.RawMessage.UnmarshalJSON:
// a non-nil receiver always returns a nil error, so the error branch of
// UnmarshalJSON is unreachable through real calls; only tests can inject a
// failure via this variable to verify error propagation.
var unmarshalRawMessage = func(m *json.RawMessage, b []byte) error {
	return m.UnmarshalJSON(b)
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *JSON) UnmarshalJSON(b []byte) error {
	result := json.RawMessage{}
	if err := unmarshalRawMessage(&result, b); err != nil {
		return err
	}
	*j = JSON(result)
	return nil
}

// String returns the raw document text.
func (j JSON) String() string {
	return string(j)
}

// GormDataType reports the logical gorm data type.
func (JSON) GormDataType() string {
	return "json"
}

// GormDBDataType maps the column type per dialect. Unlike datatypes.JSON the
// default branch is populated: sqlserver (and any unknown dialect) gets
// nvarchar(max) instead of the unsupported literal `json`.
func (JSON) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Name() {
	case "sqlite", "mysql":
		return "JSON"
	case "postgres":
		return "JSONB"
	default:
		return "nvarchar(max)"
	}
}

// GormValue builds the write expression; mysql CAST mirrors datatypes.JSON so
// JSON-typed columns keep server-side validation.
func (j JSON) GormValue(_ context.Context, db *gorm.DB) clause.Expr {
	if len(j) == 0 {
		return gorm.Expr("NULL")
	}
	data, _ := j.MarshalJSON()
	if v, ok := db.Dialector.(*gmysql.Dialector); ok && db.Name() == "mysql" && !strings.Contains(v.ServerVersion, "MariaDB") {
		return gorm.Expr("CAST(? AS JSON)", string(data))
	}
	return gorm.Expr("?", string(data))
}
