package model

import (
	"fmt"

	"gorm.io/gorm"
)

// legacyUITables lists old page/UI tables that no longer have model definitions.
// These may exist in production databases from before the Dashboard vNext refactor.
var legacyUITables = []string{
	"function_ui_schemas",
	"function_ui_schema_versions",
	"function_ui_schema_documents",
}

// legacyUIColumns lists old UI-related columns that may exist on the functions table.
// These were part of the registration-side UI fields that have been removed.
var legacyUIColumns = []string{
	"category_display",
	"entity_display",
	"operation_display",
	"operation_kind",
	"page_hint",
	"x_labels",
	"ui_schema",
	"ui_config",
}

// CleanupLegacyPageTables drops old page/UI tables that no longer have model
// definitions. Safe to call multiple times — uses HasTable checks before
// dropping. This is the H-005-1 cleanup from the legacy deletion inventory.
func CleanupLegacyPageTables(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	migrator := db.Migrator()
	for _, table := range legacyUITables {
		if migrator.HasTable(table) {
			if err := migrator.DropTable(table); err != nil {
				return fmt.Errorf("drop legacy table %s: %w", table, err)
			}
		}
	}
	return nil
}

// tableHasColumn checks whether a table has a specific column using raw
// SQL information_schema query. Works reliably across all database drivers
// unlike GORM's HasColumn which has issues with columns not in the Go struct.
func tableHasColumn(db *gorm.DB, tableName, columnName string) bool {
	var count int64
	switch db.Dialector.Name() {
	case "sqlite":
		// SQLite PRAGMA returns column info for a table.
		rows, err := db.Raw("PRAGMA table_info(" + tableName + ")").Rows()
		if err != nil {
			return false
		}
		defer rows.Close()
		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull int
			var dfltValue interface{}
			var pk int
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
				continue
			}
			if name == columnName {
				return true
			}
		}
		return false
	case "postgres":
		err := db.Raw(
			"SELECT COUNT(*) FROM information_schema.columns WHERE table_name = ? AND column_name = ?",
			tableName, columnName,
		).Scan(&count).Error
		return err == nil && count > 0
	case "mysql":
		err := db.Raw(
			"SELECT COUNT(*) FROM information_schema.columns WHERE table_name = ? AND column_name = ? AND table_schema = DATABASE()",
			tableName, columnName,
		).Scan(&count).Error
		return err == nil && count > 0
	default:
		// Fallback to GORM migrator.
		return db.Migrator().HasColumn(&Function{}, columnName)
	}
}

// dropColumnFromTable drops a column from a table using raw SQL.
// Handles the dialect differences for DROP COLUMN.
func dropColumnFromTable(db *gorm.DB, tableName, columnName string) error {
	switch db.Dialector.Name() {
	case "sqlite":
		// SQLite >= 3.35.0 supports ALTER TABLE DROP COLUMN.
		return db.Exec("ALTER TABLE " + tableName + " DROP COLUMN " + columnName).Error
	case "postgres":
		return db.Exec("ALTER TABLE " + tableName + " DROP COLUMN IF EXISTS " + columnName).Error
	case "mysql":
		return db.Exec("ALTER TABLE " + tableName + " DROP COLUMN " + columnName).Error
	default:
		return db.Migrator().DropColumn(&Function{}, columnName)
	}
}

// CleanupLegacyUIColumns drops old UI-related columns from the functions table.
// These columns were part of the registration-side UI fields removed in H-004.
// Safe to call multiple times — checks column existence before dropping.
func CleanupLegacyUIColumns(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(&Function{}) {
		return nil
	}
	for _, col := range legacyUIColumns {
		if tableHasColumn(db, "functions", col) {
			if err := dropColumnFromTable(db, "functions", col); err != nil {
				return fmt.Errorf("drop legacy column %s from functions: %w", col, err)
			}
		}
	}
	return nil
}

// CleanupAllLegacy performs all legacy cleanup operations from H-005.
// Safe to call on every startup — all operations are idempotent.
func CleanupAllLegacy(db *gorm.DB) error {
	if err := CleanupLegacyPageTables(db); err != nil {
		return err
	}
	return CleanupLegacyUIColumns(db)
}

// LegacyCleanupReport returns a summary of what cleanup would do, without
// modifying anything. Useful for dry-run verification.
func LegacyCleanupReport(db *gorm.DB) (tables []string, columns []string) {
	if db == nil {
		return nil, nil
	}
	migrator := db.Migrator()
	for _, table := range legacyUITables {
		if migrator.HasTable(table) {
			tables = append(tables, table)
		}
	}
	if migrator.HasTable(&Function{}) {
		for _, col := range legacyUIColumns {
			if tableHasColumn(db, "functions", col) {
				columns = append(columns, col)
			}
		}
	}
	return tables, columns
}
