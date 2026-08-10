package model

import (
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCleanupLegacyPageTables_NoopWhenEmpty(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := CleanupLegacyPageTables(db); err != nil {
		t.Fatalf("CleanupLegacyPageTables should succeed on empty db: %v", err)
	}
}

func TestCleanupLegacyPageTables_DropsExistingTables(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	for _, table := range legacyUITables {
		if err := db.Exec("CREATE TABLE " + table + " (id INTEGER PRIMARY KEY)").Error; err != nil {
			t.Fatalf("create simulated table %s: %v", table, err)
		}
	}
	migrator := db.Migrator()
	for _, table := range legacyUITables {
		if !migrator.HasTable(table) {
			t.Fatalf("table %s should exist before cleanup", table)
		}
	}
	if err := CleanupLegacyPageTables(db); err != nil {
		t.Fatalf("CleanupLegacyPageTables failed: %v", err)
	}
	for _, table := range legacyUITables {
		if migrator.HasTable(table) {
			t.Errorf("table %s should be dropped after cleanup", table)
		}
	}
}

func TestCleanupLegacyPageTables_Idempotent(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := CleanupLegacyPageTables(db); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if err := CleanupLegacyPageTables(db); err != nil {
		t.Fatalf("second call failed: %v", err)
	}
}

func TestCleanupLegacyUIColumns_NoopWhenNoTable(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := CleanupLegacyUIColumns(db); err != nil {
		t.Fatalf("CleanupLegacyUIColumns should succeed when functions table missing: %v", err)
	}
}

func TestCleanupLegacyUIColumns_NoopWhenClean(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&Function{}); err != nil {
		t.Fatalf("auto migrate Function: %v", err)
	}
	if err := CleanupLegacyUIColumns(db); err != nil {
		t.Fatalf("CleanupLegacyUIColumns should succeed on clean table: %v", err)
	}
}

func TestCleanupLegacyUIColumns_DropsExistingColumns(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// Create a functions table with legacy columns.
	if err := db.Exec(`
		CREATE TABLE functions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			function_id TEXT,
			name TEXT,
			category_display TEXT,
			entity_display TEXT,
			operation_display TEXT,
			operation_kind TEXT,
			page_hint TEXT,
			x_labels TEXT,
			ui_schema TEXT,
			ui_config TEXT
		)
	`).Error; err != nil {
		t.Fatalf("create simulated functions table: %v", err)
	}
	// Verify legacy columns exist using raw SQL.
	for _, col := range legacyUIColumns {
		if !tableHasColumn(db, "functions", col) {
			t.Fatalf("column %s should exist before cleanup", col)
		}
	}
	// Run cleanup.
	if err := CleanupLegacyUIColumns(db); err != nil {
		t.Fatalf("CleanupLegacyUIColumns failed: %v", err)
	}
	// Verify legacy columns are gone.
	for _, col := range legacyUIColumns {
		if tableHasColumn(db, "functions", col) {
			t.Errorf("column %s should be dropped after cleanup", col)
		}
	}
}

func TestCleanupLegacyUIColumns_Idempotent(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE functions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			function_id TEXT,
			name TEXT,
			category_display TEXT,
			entity_display TEXT
		)
	`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := CleanupLegacyUIColumns(db); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if err := CleanupLegacyUIColumns(db); err != nil {
		t.Fatalf("second call failed: %v", err)
	}
}

func TestCleanupAllLegacy_Integration(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	for _, table := range legacyUITables {
		if err := db.Exec("CREATE TABLE " + table + " (id INTEGER PRIMARY KEY)").Error; err != nil {
			t.Fatalf("create table %s: %v", table, err)
		}
	}
	if err := db.Exec(`
		CREATE TABLE functions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			function_id TEXT,
			name TEXT,
			category_display TEXT,
			entity_display TEXT,
			operation_display TEXT
		)
	`).Error; err != nil {
		t.Fatalf("create functions: %v", err)
	}
	if err := CleanupAllLegacy(db); err != nil {
		t.Fatalf("CleanupAllLegacy failed: %v", err)
	}
	migrator := db.Migrator()
	for _, table := range legacyUITables {
		if migrator.HasTable(table) {
			t.Errorf("table %s should be dropped", table)
		}
	}
	for _, col := range legacyUIColumns {
		if tableHasColumn(db, "functions", col) {
			t.Errorf("column %s should be dropped", col)
		}
	}
}

func TestLegacyCleanupReport(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// Empty db — nothing to report.
	tables, columns := LegacyCleanupReport(db)
	if len(tables) != 0 || len(columns) != 0 {
		t.Fatalf("expected empty report, got tables=%v columns=%v", tables, columns)
	}
	// Add legacy table and column.
	if err := db.Exec("CREATE TABLE function_ui_schemas (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := db.Exec("CREATE TABLE functions (id INTEGER PRIMARY KEY, function_id TEXT, category_display TEXT)").Error; err != nil {
		t.Fatalf("create functions: %v", err)
	}
	tables, columns = LegacyCleanupReport(db)
	if len(tables) != 1 || tables[0] != "function_ui_schemas" {
		t.Errorf("expected [function_ui_schemas], got %v", tables)
	}
	if len(columns) != 1 || columns[0] != "category_display" {
		t.Errorf("expected [category_display], got %v", columns)
	}
}
