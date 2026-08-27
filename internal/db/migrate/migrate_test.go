package migrate

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/test.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func mapFS(files map[string]string) fs.FS {
	out := fstest.MapFS{}
	for name, content := range files {
		out[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return out
}

// baselineProbe creates a probe table so tests can verify the baseline bridge
// executed exactly when no version table existed.
func baselineProbe(db *gorm.DB) error {
	return db.Exec("CREATE TABLE IF NOT EXISTS probe (id INTEGER PRIMARY KEY)").Error
}

func probeExists(t *testing.T, db *gorm.DB) bool {
	t.Helper()
	var count int
	if err := db.Raw("SELECT COUNT(1) FROM sqlite_master WHERE type='table' AND name='probe'").Scan(&count).Error; err != nil {
		t.Fatalf("probe check: %v", err)
	}
	return count > 0
}

func TestEnsureUpToDate_FreshDatabaseRunsBaselineAndMigrations(t *testing.T) {
	db := openTestDB(t)
	fsys := mapFS(map[string]string{
		"0001_baseline.sql":    "-- +goose Up\nSELECT 1;\n\n-- +goose Down\nSELECT 1;\n",
		"0002_add_probe2.sql":  "-- +goose Up\nCREATE TABLE probe2 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe2;\n",
		"0003_add_probe3.sql":  "-- +goose Up\nCREATE TABLE probe3 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe3;\n",
		"0004_add_probe4.sql":  "-- +goose Up\nCREATE TABLE probe4 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe4;\n",
		"0005_add_probe5.sql":  "-- +goose Up\nCREATE TABLE probe5 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe5;\n",
		"0006_add_probe6.sql":  "-- +goose Up\nCREATE TABLE probe6 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe6;\n",
		"0007_add_probe7.sql":  "-- +goose Up\nCREATE TABLE probe7 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe7;\n",
		"0008_add_probe8.sql":  "-- +goose Up\nCREATE TABLE probe8 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe8;\n",
		"0009_add_probe9.sql":  "-- +goose Up\nCREATE TABLE probe9 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe9;\n",
		"0010_add_probe10.sql": "-- +goose Up\nCREATE TABLE probe10 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe10;\n",
		"0011_add_probe11.sql": "-- +goose Up\nCREATE TABLE probe11 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe11;\n",
		"0012_add_probe12.sql": "-- +goose Up\nCREATE TABLE probe12 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe12;\n",
		"0013_add_probe13.sql": "-- +goose Up\nCREATE TABLE probe13 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe13;\n",
		"0014_add_probe14.sql": "-- +goose Up\nCREATE TABLE probe14 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe14;\n",
	})

	version, err := ensureUpToDate(context.Background(), db, fsys, ScopeMeta, baselineProbe)
	if err != nil {
		t.Fatalf("ensureUpToDate: %v", err)
	}
	if version < 14 {
		t.Fatalf("version = %d, want >= 14", version)
	}
	if !probeExists(t, db) {
		t.Fatal("baseline did not run")
	}
	for _, table := range []string{"probe2", "probe3", "probe4", "probe5", "probe6", "probe7", "probe8", "probe9", "probe10", "probe11", "probe12", "probe13", "probe14"} {
		if !tableExists(t, db, table) {
			t.Fatalf("migration table %s missing", table)
		}
	}
	if !versionTableExistsFor(t, db) {
		t.Fatal("version table missing")
	}
}

func TestEnsureUpToDate_LegacyDatabaseBridgesOnceThenCatchesUp(t *testing.T) {
	db := openTestDB(t)
	// Simulate a pre-versioning database: tables exist, no version table.
	if err := db.Exec("CREATE TABLE probe (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatalf("seed legacy table: %v", err)
	}

	fsys := mapFS(map[string]string{
		"0001_baseline.sql":    "-- +goose Up\nSELECT 1;\n\n-- +goose Down\nSELECT 1;\n",
		"0002_add_probe2.sql":  "-- +goose Up\nCREATE TABLE probe2 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe2;\n",
		"0003_add_probe3.sql":  "-- +goose Up\nCREATE TABLE probe3 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe3;\n",
		"0004_add_probe4.sql":  "-- +goose Up\nCREATE TABLE probe4 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe4;\n",
		"0005_add_probe5.sql":  "-- +goose Up\nCREATE TABLE probe5 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe5;\n",
		"0006_add_probe6.sql":  "-- +goose Up\nCREATE TABLE probe6 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe6;\n",
		"0007_add_probe7.sql":  "-- +goose Up\nCREATE TABLE probe7 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe7;\n",
		"0008_add_probe8.sql":  "-- +goose Up\nCREATE TABLE probe8 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe8;\n",
		"0009_add_probe9.sql":  "-- +goose Up\nCREATE TABLE probe9 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe9;\n",
		"0010_add_probe10.sql": "-- +goose Up\nCREATE TABLE probe10 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe10;\n",
		"0011_add_probe11.sql": "-- +goose Up\nCREATE TABLE probe11 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe11;\n",
		"0012_add_probe12.sql": "-- +goose Up\nCREATE TABLE probe12 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe12;\n",
		"0013_add_probe13.sql": "-- +goose Up\nCREATE TABLE probe13 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe13;\n",
		"0014_add_probe14.sql": "-- +goose Up\nCREATE TABLE probe14 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe14;\n",
	})
	if _, err := ensureUpToDate(context.Background(), db, fsys, ScopeGame, baselineProbe); err != nil {
		t.Fatalf("ensureUpToDate: %v", err)
	}
	if !tableExists(t, db, "probe2") {
		t.Fatal("catch-up migration not applied on legacy database")
	}

	// Second boot must be a no-op catch-up (idempotent).
	fsys2 := mapFS(map[string]string{
		"0001_baseline.sql":    "-- +goose Up\nSELECT 1;\n\n-- +goose Down\nSELECT 1;\n",
		"0002_add_probe2.sql":  "-- +goose Up\nCREATE TABLE probe2 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe2;\n",
		"0003_add_probe3.sql":  "-- +goose Up\nCREATE TABLE probe3 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe3;\n",
		"0004_add_probe4.sql":  "-- +goose Up\nCREATE TABLE probe4 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe4;\n",
		"0005_add_probe5.sql":  "-- +goose Up\nCREATE TABLE probe5 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe5;\n",
		"0006_add_probe6.sql":  "-- +goose Up\nCREATE TABLE probe6 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe6;\n",
		"0007_add_probe7.sql":  "-- +goose Up\nCREATE TABLE probe7 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe7;\n",
		"0008_add_probe8.sql":  "-- +goose Up\nCREATE TABLE probe8 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe8;\n",
		"0009_add_probe9.sql":  "-- +goose Up\nCREATE TABLE probe9 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe9;\n",
		"0010_add_probe10.sql": "-- +goose Up\nCREATE TABLE probe10 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe10;\n",
		"0011_add_probe11.sql": "-- +goose Up\nCREATE TABLE probe11 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe11;\n",
		"0012_add_probe12.sql": "-- +goose Up\nCREATE TABLE probe12 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe12;\n",
		"0013_add_probe13.sql": "-- +goose Up\nCREATE TABLE probe13 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe13;\n",
		"0014_add_probe14.sql": "-- +goose Up\nCREATE TABLE probe14 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe14;\n",
	})
	version, err := ensureUpToDate(context.Background(), db, fsys2, ScopeGame, nil)
	if err != nil {
		t.Fatalf("second ensureUpToDate: %v", err)
	}
	if version != 14 {
		t.Fatalf("version = %d, want 14", version)
	}
}

func TestEnsureUpToDate_UpToDateDatabaseSkipsBaseline(t *testing.T) {
	db := openTestDB(t)
	fsys := mapFS(map[string]string{
		"0001_baseline.sql":    "-- +goose Up\nSELECT 1;\n\n-- +goose Down\nSELECT 1;\n",
		"0002_add_probe2.sql":  "-- +goose Up\nCREATE TABLE probe2 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe2;\n",
		"0003_add_probe3.sql":  "-- +goose Up\nCREATE TABLE probe3 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe3;\n",
		"0004_add_probe4.sql":  "-- +goose Up\nCREATE TABLE probe4 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe4;\n",
		"0005_add_probe5.sql":  "-- +goose Up\nCREATE TABLE probe5 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe5;\n",
		"0006_add_probe6.sql":  "-- +goose Up\nCREATE TABLE probe6 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe6;\n",
		"0007_add_probe7.sql":  "-- +goose Up\nCREATE TABLE probe7 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe7;\n",
		"0008_add_probe8.sql":  "-- +goose Up\nCREATE TABLE probe8 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe8;\n",
		"0009_add_probe9.sql":  "-- +goose Up\nCREATE TABLE probe9 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe9;\n",
		"0010_add_probe10.sql": "-- +goose Up\nCREATE TABLE probe10 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe10;\n",
		"0011_add_probe11.sql": "-- +goose Up\nCREATE TABLE probe11 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe11;\n",
		"0012_add_probe12.sql": "-- +goose Up\nCREATE TABLE probe12 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe12;\n",
		"0013_add_probe13.sql": "-- +goose Up\nCREATE TABLE probe13 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe13;\n",
		"0014_add_probe14.sql": "-- +goose Up\nCREATE TABLE probe14 (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE probe14;\n",
	})
	if _, err := ensureUpToDate(context.Background(), db, fsys, ScopeMeta, baselineProbe); err != nil {
		t.Fatalf("first ensureUpToDate: %v", err)
	}

	// Drop the probe table; an up-to-date database must NOT re-run baseline.
	if err := db.Exec("DROP TABLE probe").Error; err != nil {
		t.Fatalf("drop probe: %v", err)
	}
	calls := 0
	wrapped := func(db *gorm.DB) error {
		calls++
		return baselineProbe(db)
	}
	if _, err := ensureUpToDate(context.Background(), db, fsys, ScopeMeta, wrapped); err != nil {
		t.Fatalf("second ensureUpToDate: %v", err)
	}
	if calls != 0 {
		t.Fatalf("baseline ran %d times on up-to-date database, want 0", calls)
	}
	if probeExists(t, db) {
		t.Fatal("baseline re-ran on already-migrated database")
	}
}

func TestCurrentVersion_NoTable(t *testing.T) {
	db := openTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	version, ok, err := CurrentVersion(context.Background(), sqlDB, "sqlite")
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	if ok || version != 0 {
		t.Fatalf("version=%d ok=%v, want zero/false on unmigrated database", version, ok)
	}
}

func TestDialectOf(t *testing.T) {
	cases := map[string]string{
		"mysql":      "mysql",
		"MariaDB":    "mysql",
		"postgres":   "postgres",
		"sqlite":     "sqlite3",
		"sqlite3":    "sqlite3",
		"clickhouse": "",
		"":           "",
	}
	for in, want := range cases {
		if got := dialectOf(in); got != want {
			t.Errorf("dialectOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEmbeddedMigrationsIncludeBaseline(t *testing.T) {
	sub, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		t.Fatalf("sub fs: %v", err)
	}
	entries, err := fs.ReadDir(sub, ".")
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no embedded migrations found")
	}
	found := false
	for _, e := range entries {
		if filepath.Base(e.Name()) == "0001_baseline.sql" {
			found = true
		}
	}
	if !found {
		t.Fatal("0001_baseline.sql missing from embedded migrations")
	}
}

func tableExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var count int
	if err := db.Raw("SELECT COUNT(1) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&count).Error; err != nil {
		t.Fatalf("table exists check: %v", err)
	}
	return count > 0
}

func versionTableExistsFor(t *testing.T, db *gorm.DB) bool {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	exists, err := versionTableExists(context.Background(), sqlDB, "sqlite3")
	if err != nil {
		t.Fatalf("versionTableExists: %v", err)
	}
	_ = sqlDB

	return exists
}
