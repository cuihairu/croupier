// Package dbtype 提供方言感知的数据库列类型。叶子包：仅依赖 gorm，
// 禁止 import 任何 internal/ 业务包（model 与 platform/registry 之间
// 存在依赖环风险，两者都必须能用本类型）。
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

// JSON 是 datatypes.JSON 的方言感知替代：语义逐方法对齐（Value 空值返回
// nil、Scan nil 写 "null"、MarshalJSON 输出原文而非 base64），唯一差异是
// GormDBDataType 补齐 sqlserver 分支——datatypes.JSON 对未知方言返回空
// 字符串，gorm sqlserver 驱动随即回落到字面量 `json`，而 SQL Server 没有
// 原生 JSON 类型，AutoMigrate 建表直接报 Cannot find data type json。
// sqlserver 下 JSON 文档以 nvarchar(max) 存储（UTF-16，无损）。
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
			return fmt.Errorf("Failed to unmarshal JSONB value: %v", value)
		}
	}
	*j = JSON(json.RawMessage(bytes))
	return nil
}

// MarshalJSON outputs the raw document instead of base64.
func (j JSON) MarshalJSON() ([]byte, error) {
	return json.RawMessage(j).MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *JSON) UnmarshalJSON(b []byte) error {
	result := json.RawMessage{}
	if err := result.UnmarshalJSON(b); err != nil {
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
	switch db.Dialector.Name() {
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
	if v, ok := db.Dialector.(*gmysql.Dialector); ok && db.Dialector.Name() == "mysql" && !strings.Contains(v.ServerVersion, "MariaDB") {
		return gorm.Expr("CAST(? AS JSON)", string(data))
	}
	return gorm.Expr("?", string(data))
}
