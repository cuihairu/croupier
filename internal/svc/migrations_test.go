package svc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/db/migrate"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "migrate.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

// TestGoMigrationsRegistered pins the numbered Go migrations so the provider
// sees exactly versions 1..4 on the production path.
func TestGoMigrationsRegistered(t *testing.T) {
	db := openMigrationTestDB(t)
	version, err := migrate.EnsureUpToDate(context.Background(), db, migrate.ScopeSingle, autoMigrate)
	if err != nil {
		t.Fatalf("EnsureUpToDate: %v", err)
	}
	if version != migrate.MinimumRequiredVersion {
		t.Fatalf("version = %d, want %d (Go migrations not collected by provider?)", version, migrate.MinimumRequiredVersion)
	}

	// Second boot is a no-op catch-up and must not re-run the baseline.
	calls := 0
	wrapped := func(db *gorm.DB) error {
		calls++
		return autoMigrate(db)
	}
	if _, err := migrate.EnsureUpToDate(context.Background(), db, migrate.ScopeSingle, wrapped); err != nil {
		t.Fatalf("second EnsureUpToDate: %v", err)
	}
	if calls != 0 {
		t.Fatalf("baseline ran %d times on up-to-date database, want 0", calls)
	}
}

// TestEnsureUpToDate_ConvertsLegacyVarcharEnum simulates a pre-versioning
// database whose enum column is still varchar: the baseline bridge must
// convert values (not destroy them) and the Go migrations must complete.
func TestEnsureUpToDate_ConvertsLegacyVarcharEnum(t *testing.T) {
	db := openMigrationTestDB(t)
	if err := db.Exec("CREATE TABLE function_contracts (id INTEGER PRIMARY KEY, capability VARCHAR(16))").Error; err != nil {
		t.Fatalf("seed legacy table: %v", err)
	}
	if err := db.Exec("INSERT INTO function_contracts (id, capability) VALUES (1, 'delete'), (2, 'create')").Error; err != nil {
		t.Fatalf("seed legacy rows: %v", err)
	}

	if _, err := migrate.EnsureUpToDate(context.Background(), db, migrate.ScopeSingle, autoMigrate); err != nil {
		t.Fatalf("EnsureUpToDate: %v", err)
	}

	var caps []int
	if err := db.Raw("SELECT capability FROM function_contracts ORDER BY id").Scan(&caps).Error; err != nil {
		t.Fatalf("read back capability: %v", err)
	}
	if len(caps) != 2 || caps[0] != 6 || caps[1] != 4 {
		t.Fatalf("capability values = %v, want [6 4] ('delete'->6, 'create'->4)", caps)
	}
}

// TestProbeDialect_Sqlite covers the dialect probe used by Go migrations.
func TestProbeDialect_Sqlite(t *testing.T) {
	db := openMigrationTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	dialect, err := probeDialect(sqlDB)
	if err != nil {
		t.Fatalf("probeDialect: %v", err)
	}
	if dialect != "sqlite" {
		t.Fatalf("dialect = %q, want sqlite", dialect)
	}
	wrapped, err := wrapGorm(sqlDB)
	if err != nil {
		t.Fatalf("wrapGorm: %v", err)
	}
	if wrapped.Dialector == nil || wrapped.Dialector.Name() != "sqlite" {
		t.Fatalf("wrapped dialector = %+v, want sqlite", wrapped.Dialector)
	}
}
