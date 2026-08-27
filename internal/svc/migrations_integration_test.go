//go:build integration

// Migration matrix integration tests: exercise the versioned migration
// executor (baseline bridge + numbered Go migrations + cross-process session
// lock) against real MySQL and PostgreSQL servers. SQLite is covered by the
// unit tests in migrations_test.go.
//
// Enable with -tags=integration and provide DSNs:
//
//	TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/"
//	TEST_POSTGRES_DSN="postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable"
//	TEST_SQLSERVER_DSN="sqlserver://sa:YourStr0ngPass@127.0.0.1:1433"
//
// Each run creates (and finally drops) a uniquely named throwaway database.
package svc

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/db/migrate"
	gmysql "gorm.io/driver/mysql"
	gpostgres "gorm.io/driver/postgres"
	gsqlserver "gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMigrationMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	t.Run("MySQL", func(t *testing.T) {
		dsn := strings.TrimSpace(strings.TrimRight(strings.TrimPrefix(strings.TrimSpace(envOr("TEST_MYSQL_DSN", "")), "\"'"), "\"'"))
		if dsn == "" {
			t.Skip("TEST_MYSQL_DSN not set")
		}
		runMigrationMatrixCase(t, "mysql", dsn)
	})
	t.Run("Postgres", func(t *testing.T) {
		dsn := envOr("TEST_POSTGRES_DSN", "")
		if dsn == "" {
			t.Skip("TEST_POSTGRES_DSN not set")
		}
		runMigrationMatrixCase(t, "postgres", dsn)
	})
	t.Run("SQLServer", func(t *testing.T) {
		dsn := envOr("TEST_SQLSERVER_DSN", "")
		if dsn == "" {
			t.Skip("TEST_SQLSERVER_DSN not set")
		}
		runMigrationMatrixCase(t, "sqlserver", dsn)
	})
}

func runMigrationMatrixCase(t *testing.T, dialect, adminDSN string) {
	t.Helper()
	dbName := fmt.Sprintf("migrate_matrix_%d", time.Now().UnixNano()%1_000_000_000)

	var open func(dsn string) (*gorm.DB, error)
	switch dialect {
	case "mysql":
		open = func(dsn string) (*gorm.DB, error) {
			return gorm.Open(gmysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		}
	case "postgres":
		open = func(dsn string) (*gorm.DB, error) {
			return gorm.Open(gpostgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		}
	case "sqlserver":
		open = func(dsn string) (*gorm.DB, error) {
			return gorm.Open(gsqlserver.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		}
	}

	admin, err := open(adminDSN)
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	create := map[string]string{
		"mysql":     fmt.Sprintf("CREATE DATABASE %s", dbName),
		"postgres":  fmt.Sprintf("CREATE DATABASE %s", dbName),
		"sqlserver": fmt.Sprintf("CREATE DATABASE [%s]", dbName),
	}[dialect]
	if err := admin.Exec(create).Error; err != nil {
		t.Fatalf("create throwaway database: %v", err)
	}
	t.Cleanup(func() {
		drop := map[string]string{
			"mysql":     fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName),
			"postgres":  fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName),
			"sqlserver": fmt.Sprintf("IF DB_ID('%s') IS NOT NULL ALTER DATABASE [%s] SET SINGLE_USER WITH ROLLBACK IMMEDIATE; DROP DATABASE IF EXISTS [%s]", dbName, dbName, dbName),
		}[dialect]
		if err := admin.Exec(drop).Error; err != nil {
			t.Logf("cleanup drop database: %v", err)
		}
	})

	targetDSN := map[string]func(string, string) string{
		"mysql": func(dsn, name string) string {
			if strings.HasSuffix(dsn, "/") {
				return dsn + name
			}
			return dsn + "/" + name
		},
		"postgres": func(dsn, name string) string {
			u, err := url.Parse(dsn)
			if err != nil {
				t.Fatalf("parse postgres DSN: %v", err)
			}
			u.Path = "/" + name
			return u.String()
		},
		"sqlserver": func(dsn, name string) string {
			u, err := url.Parse(dsn)
			if err != nil {
				t.Fatalf("parse sqlserver DSN: %v", err)
			}
			q := u.Query()
			q.Set("database", name)
			u.RawQuery = q.Encode()
			return u.String()
		},
	}[dialect](adminDSN, dbName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// First boot: baseline bridge + full catch-up, concurrently from two
	// connections to prove the session lock serializes the AutoMigrate bridge.
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			db, err := open(targetDSN)
			if err != nil {
				results[slot] = err
				return
			}
			calls := 0
			baseline := func(db *gorm.DB) error {
				calls++
				return autoMigrate(db)
			}
			if _, err := migrate.EnsureUpToDate(ctx, db, migrate.ScopeSingle, baseline); err != nil {
				results[slot] = fmt.Errorf("slot %d: %w", slot, err)
			}
		}(i)
	}
	wg.Wait()
	for i, err := range results {
		if err != nil {
			t.Fatalf("concurrent first boot slot %d: %v", i, err)
		}
	}

	// Verify schema state.
	db, err := open(targetDSN)
	if err != nil {
		t.Fatalf("open migrated database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	gormDialect := map[string]string{"mysql": "mysql", "postgres": "postgres", "sqlserver": "sqlserver"}[dialect]
	version, ok, err := migrate.CurrentVersion(ctx, sqlDB, gormDialect)
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	if !ok || version != migrate.MinimumRequiredVersion {
		t.Fatalf("version = %d ok=%v, want %d", version, ok, migrate.MinimumRequiredVersion)
	}

	// Spot-check a baseline table exists (admins is a meta model).
	var count int
	adminTable := map[string]string{
		"mysql":     "SELECT COUNT(1) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'admins'",
		"postgres":  "SELECT COUNT(1) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'admins'",
		"sqlserver": "SELECT COUNT(1) FROM sys.objects WHERE type = 'U' AND name = 'admins'",
	}[dialect]
	if err := db.Raw(adminTable).Scan(&count).Error; err != nil {
		t.Fatalf("probe admins table: %v", err)
	}
	if count != 1 {
		t.Fatalf("admins table missing after migration")
	}

	// Second boot: idempotent catch-up, baseline must not re-run.
	calls := 0
	baseline := func(db *gorm.DB) error {
		calls++
		return autoMigrate(db)
	}
	if _, err := migrate.EnsureUpToDate(ctx, db, migrate.ScopeSingle, baseline); err != nil {
		t.Fatalf("second boot: %v", err)
	}
	if calls != 0 {
		t.Fatalf("baseline ran %d times on up-to-date database, want 0", calls)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
