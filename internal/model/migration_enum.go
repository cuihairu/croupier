package model

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// enumColumnMigration converts a legacy varchar status column into the
// int-backed enum column. Dialect-neutral: plain ALTER/UPDATE with CASE WHEN.
type enumColumnMigration struct {
	table      string
	column     string
	mapping    map[string]int
	defaultInt int
}

// MigrateEnumColumns converts legacy varchar enum columns to int-backed enums
// (exported for the numbered goose migration; see internal/svc/migrations.go).
func MigrateEnumColumns(db *gorm.DB) error { return migrateEnumColumns(db) }

// migrateEnumColumns runs after AutoMigrate created the new int columns (the
// legacy string columns keep their names with a "_legacy" suffix created by
// gorm? No—) In practice: AutoMigrate alters the column type in place, which
// would destroy string values. To stay safe we snapshot legacy values into a
// shadow column first, then translate row by row.
func migrateEnumColumns(db *gorm.DB) error {
	migrations := []enumColumnMigration{
		{
			table:  "function_contracts",
			column: "capability",
			mapping: map[string]int{
				"collection_query": 2,
				"item_query":       3,
				"create":           4,
				"update":           5,
				"delete":           6,
				"action":           7,
				"task":             8,
				"report":           9,
			},
			defaultInt: 1, // unknown
		},
		{
			table:  "function_contracts",
			column: "risk",
			mapping: map[string]int{
				"safe":    1,
				"warning": 2,
				"high":    3,
				"danger":  4,
			},
			defaultInt: 1,
		},
		{
			table:  "page_proposals",
			column: "status",
			mapping: map[string]int{
				"pending":  1,
				"accepted": 2,
				"rejected": 3,
			},
			defaultInt: 1,
		},
		{
			table:  "tickets",
			column: "status",
			mapping: map[string]int{
				"open":        1,
				"in_progress": 2,
				"resolved":    3,
				"closed":      4,
			},
			defaultInt: 1,
		},
		{
			table:  "messages",
			column: "status",
			mapping: map[string]int{
				"unread": 1,
				"read":   2,
			},
			defaultInt: 1,
		},
		{
			table:  "feedbacks",
			column: "status",
			mapping: map[string]int{
				"open":    1,
				"triaged": 2,
				"closed":  3,
			},
			defaultInt: 1,
		},
	}

	for _, m := range migrations {
		if err := migrateOneEnumColumn(db, m); err != nil {
			return err
		}
	}
	// Backfill unknown capabilities from operation suffixes (legacy rows
	// registered before capability inference existed).
	if db != nil && db.Migrator().HasTable("function_contracts") {
		if err := db.Exec(`UPDATE function_contracts SET capability = CASE
			WHEN function_id LIKE '%.list' OR function_id LIKE '%.search' OR function_id LIKE '%.query' THEN 2
			WHEN function_id LIKE '%.get' OR function_id LIKE '%.detail' THEN 3
			WHEN function_id LIKE '%.create' OR function_id LIKE '%.add' THEN 4
			WHEN function_id LIKE '%.update' OR function_id LIKE '%.edit' OR function_id LIKE '%.set' THEN 5
			WHEN function_id LIKE '%.delete' OR function_id LIKE '%.remove' THEN 6
			WHEN capability = 1 THEN 7
			ELSE capability END
			WHERE capability = 1`).Error; err != nil {
			// Non-fatal: inference is best-effort.
			db.Logger.Warn(nil, "enum capability backfill failed: %v", err)
		}
	}
	return nil
}

// migrateOneEnumColumn: if the column is still a string type (legacy), copy
// values to a shadow int column and translate. Idempotent: skips when the
// column data type is already integer.
func migrateOneEnumColumn(db *gorm.DB, m enumColumnMigration) error {
	migrator := db.Migrator()
	if !migrator.HasTable(m.table) {
		return nil
	}

	// Drop the legacy default FIRST: PG refuses to alter a column whose
	// default needs a cast when the column type changes. Harmless on int
	// columns and on SQLite.
	if migrator.HasColumn(m.table, m.column) {
		_ = db.Exec(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT", m.table, m.column)).Error
	}

	// Skip when the column is already an integer type.
	if columnIsInteger(db, m.table, m.column) {
		return nil
	}

	var legacyRows int64
	shadow := m.column + "_enum_migrate"

	// Count legacy wire-string rows (0-row tables still need the type swap
	// because AutoMigrate would try to alter the varchar column otherwise).
	if err := db.Table(m.table).
		Where(fmt.Sprintf("%s IN ('collection_query','item_query','create','update','delete','action','task','report','safe','warning','high','danger','pending','accepted','rejected','expired','open','in_progress','resolved','closed','unread','read','triaged','new')", m.column)).
		Count(&legacyRows).Error; err != nil {
		// Comparison against an int column fails on PG: already migrated.
		return nil
	}

	if !migrator.HasColumn(m.table, shadow) {
		if err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s INTEGER", m.table, shadow)).Error; err != nil {
			return fmt.Errorf("add shadow column %s.%s: %w", m.table, shadow, err)
		}
	}

	// Translate legacy strings into the shadow int column.
	sql := fmt.Sprintf("UPDATE %s SET %s = CASE %s", m.table, shadow, m.column)
	for wire, value := range m.mapping {
		sql += fmt.Sprintf(" WHEN '%s' THEN %d", wire, value)
	}
	sql += fmt.Sprintf(" ELSE %d END", m.defaultInt)
	if err := db.Exec(sql).Error; err != nil {
		return fmt.Errorf("translate %s.%s: %w", m.table, m.column, err)
	}

	// Swap: drop string column, rename shadow into place.
	if err := db.Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", m.table, m.column)).Error; err != nil {
		return fmt.Errorf("drop legacy column %s.%s: %w", m.table, m.column, err)
	}
	if err := db.Exec(fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", m.table, shadow, m.column)).Error; err != nil {
		return fmt.Errorf("rename shadow column %s.%s: %w", m.table, shadow, err)
	}
	db.Logger.Info(context.Background(), "migrated enum column %s.%s from string to int (%d legacy rows)", m.table, m.column, legacyRows)
	return nil
}

// columnIsInteger reports whether the column already stores integers.
func columnIsInteger(db *gorm.DB, table, column string) bool {
	switch db.Dialector.Name() {
	case "postgres":
		var dataType string
		if err := db.Raw("SELECT data_type FROM information_schema.columns WHERE table_name = ? AND column_name = ?", table, column).Scan(&dataType).Error; err != nil {
			return true // assume migrated on probe failure
		}
		switch dataType {
		case "smallint", "integer", "bigint", "numeric":
			return true
		}
		return false
	default: // sqlite and others
		var declared string
		rows, err := db.Raw(fmt.Sprintf("SELECT type FROM pragma_table_info('%s') WHERE name = ?", table), column).Rows()
		if err != nil {
			return true
		}
		defer rows.Close()
		if rows.Next() {
			_ = rows.Scan(&declared)
		}
		switch declared {
		case "INTEGER", "INT", "BIGINT", "SMALLINT", "NUMERIC", "REAL":
			return true
		}
		return false
	}
}

// suppress unused warning for logger import when Info signature changes.
var _ = logger.Discard
