// Package migrate implements the versioned database migration executor
// described in docs/architecture/database-migration-strategy.md.
//
// Contract:
//   - The first boot against a database that has no version table runs the
//     scoped legacy GORM AutoMigrate bridge exactly once (baseline), then
//     records the baseline migration version. This is the only path that may
//     execute AutoMigrate, and it is reserved for fresh/legacy databases.
//   - Every subsequent schema change MUST be a numbered SQL file under
//     migrations/ (or a registered Go migration) and is applied in order on
//     the next boot (catch-up).
//   - A cross-process session lock guards the whole sequence — version-table
//     probe, baseline bridge and catch-up — on MySQL/Postgres so concurrent
//     server processes can never run AutoMigrate or DDL in parallel against
//     the same database (SQLite is single-writer already).
package migrate

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// Scope identifies which logical database is being migrated. It exists for
// diagnostics; meta and game databases share the same execution contract.
type Scope string

const (
	ScopeMeta   Scope = "meta"
	ScopeGame   Scope = "game"
	ScopeSingle Scope = "single"
)

// VersionTableName is the table goose uses to record applied versions.
const VersionTableName = "goose_db_version"

// MinimumRequiredVersion is the lowest schema version required by this build.
// Bump it together with new migration files once the baseline era ends.
//
// 0001 baseline marker; 0002 openapi backfill; 0003 legacy cleanup;
// 0004 enum columns; 0005 game-support context columns; 0006 bug tracker; 0007 tool registry; 0008 game release; 0009 config namespace; 0010 ticket CSAT; 0011 hotpatch; 0012 db source registry; 0013 platform settings; 0015 agent sessions addr
// (Go migrations registered in internal/svc/migrations.go).
const MinimumRequiredVersion int64 = 15

func dialectOf(gormDialect string) string {
	switch strings.ToLower(strings.TrimSpace(gormDialect)) {
	case "mysql", "mariadb":
		return "mysql"
	case "postgres", "postgresql":
		return "postgres"
	case "sqlserver", "mssql":
		return "mssql"
	case "sqlite", "sqlite3":
		return "sqlite3"
	default:
		return ""
	}
}

// sessionLockDeadline bounds how long we wait for a competing process to
// finish its own migration pass before failing startup.
const sessionLockDeadline = 60 * time.Second

const mysqlMigrationLockName = "croupier_schema_migration"

// Application-lock resource name for SQL Server (sp_getapplock).
const sqlServerMigrationLockName = "croupier_schema_migration"

// Advisory lock keys for Postgres (two int32 components). Chosen so they spell
// "crou"+"pier" in ASCII; any stable constant pair works as long as every
// process uses the same one.
const (
	pgAdvisoryLockKey1 = 0x63726f75 // "crou"
	pgAdvisoryLockKey2 = 0x70696572 // "pier"
)

// acquireSessionLock takes a cross-process lock bound to a dedicated pooled
// connection. The returned release func must be called when migration work is
// done. SQLite returns a no-op release because it is single-writer.
func acquireSessionLock(ctx context.Context, sqlDB *sql.DB, gooseDialect string) (func(), error) {
	switch gooseDialect {
	case "mysql", "postgres", "mssql":
	default:
		return func() {}, nil
	}

	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("migrate: dial lock connection: %w", err)
	}

	acquire := func() (bool, error) {
		switch gooseDialect {
		case "mysql":
			var result sql.NullInt64
			if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", mysqlMigrationLockName).Scan(&result); err != nil {
				return false, err
			}
			return result.Valid && result.Int64 == 1, nil
		case "postgres":
			var ok bool
			query := fmt.Sprintf("SELECT pg_try_advisory_lock(%d, %d)", pgAdvisoryLockKey1, pgAdvisoryLockKey2)
			if err := conn.QueryRowContext(ctx, query).Scan(&ok); err != nil {
				return false, err
			}
			return ok, nil
		case "mssql":
			// sp_getapplock: session-scoped exclusive application lock on
			// the bound connection. Return codes: 0 = granted, 1 = granted
			// after wait; <0 = timeout/cancel/error. LockTimeout 0 →
			// immediate, -1 when held elsewhere. The resource name is a
			// compile-time constant — inlined because go-mssqldb does not
			// rewrite '?' placeholders inside multi-statement batches.
			var code sql.NullInt32
			query := fmt.Sprintf("DECLARE @r int; EXEC @r = sp_getapplock @Resource = N'%s', @LockMode = 'Exclusive', @LockOwner = 'Session', @LockTimeout = 0; SELECT @r", sqlServerMigrationLockName)
			if err := conn.QueryRowContext(ctx, query).Scan(&code); err != nil {
				return false, err
			}
			return code.Valid && code.Int32 >= 0, nil
		}
		return false, nil
	}

	deadline := time.Now().Add(sessionLockDeadline)
	for {
		ok, err := acquire()
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("migrate: acquire session lock: %w", err)
		}
		if ok {
			break
		}
		if time.Now().After(deadline) {
			conn.Close()
			return nil, errors.New("migrate: could not acquire migration lock (another process may be migrating)")
		}
		select {
		case <-ctx.Done():
			conn.Close()
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	release := func() {
		switch gooseDialect {
		case "mysql":
			_, _ = conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", mysqlMigrationLockName)
		case "postgres":
			query := fmt.Sprintf("SELECT pg_advisory_unlock(%d, %d)", pgAdvisoryLockKey1, pgAdvisoryLockKey2)
			_, _ = conn.ExecContext(context.Background(), query)
		case "mssql":
			query := fmt.Sprintf("EXEC sp_releaseapplock @Resource = N'%s', @LockOwner = 'Session'", sqlServerMigrationLockName)
			_, _ = conn.ExecContext(context.Background(), query)
		}
		conn.Close()
	}
	return release, nil
}

// EnsureUpToDate brings the given database to the latest embedded migration
// version. When the database has never been migrated (no version table), the
// baseline callback runs first — pass the scoped legacy AutoMigrate function
// here. It returns the schema version after catch-up.
func EnsureUpToDate(ctx context.Context, db *gorm.DB, scope Scope, baseline func(*gorm.DB) error) (int64, error) {
	if db == nil {
		return 0, errors.New("migrate: nil gorm DB")
	}
	gormDialect := ""
	if db.Dialector != nil {
		gormDialect = db.Dialector.Name()
	}
	gooseDialect := dialectOf(gormDialect)
	if gooseDialect == "" {
		return 0, fmt.Errorf("migrate: unsupported dialect %q", gormDialect)
	}
	sub, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		return 0, fmt.Errorf("migrate: embedded fs: %w", err)
	}
	return ensureUpToDate(ctx, db, sub, scope, baseline)
}

func ensureUpToDate(ctx context.Context, db *gorm.DB, fsys fs.FS, scope Scope, baseline func(*gorm.DB) error) (int64, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return 0, fmt.Errorf("migrate: unwrap sql.DB: %w", err)
	}
	gormDialect := ""
	if db.Dialector != nil {
		gormDialect = db.Dialector.Name()
	}
	gooseDialect := dialectOf(gormDialect)

	// Hold the cross-process lock across the version-table probe, the baseline
	// bridge and the catch-up run. Without it two processes racing on the same
	// fresh database would both fail the probe and both run AutoMigrate.
	// goose's own SessionLocker is intentionally not configured here: this
	// outer lock already serializes every process, and a second lock on a
	// different connection would self-deadlock.
	release, err := acquireSessionLock(ctx, sqlDB, gooseDialect)
	if err != nil {
		return 0, err
	}
	defer release()

	hasVersions, err := versionTableExists(ctx, sqlDB, gooseDialect)
	if err != nil {
		return 0, err
	}
	if !hasVersions && baseline != nil {
		// One-time bridge: fresh database or pre-versioning legacy database.
		// This is the only sanctioned AutoMigrate path (see strategy doc).
		if err := baseline(db); err != nil {
			return 0, fmt.Errorf("migrate: %s baseline: %w", scope, err)
		}
	}

	provider, err := goose.NewProvider(goose.Dialect(gooseDialect), sqlDB, fsys)
	if err != nil {
		return 0, fmt.Errorf("migrate: provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return 0, fmt.Errorf("migrate: %s up: %w", scope, err)
	}
	version, ok, err := CurrentVersion(ctx, sqlDB, gooseDialect)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, errors.New("migrate: no recorded version after Up")
	}
	if version < MinimumRequiredVersion {
		return 0, fmt.Errorf("migrate: schema version %d below required %d", version, MinimumRequiredVersion)
	}
	return version, nil
}

// CurrentVersion reports the recorded schema version. ok is false when the
// database has never been migrated.
func CurrentVersion(ctx context.Context, sqlDB *sql.DB, gormDialect string) (int64, bool, error) {
	gooseDialect := dialectOf(gormDialect)
	exists, err := versionTableExists(ctx, sqlDB, gooseDialect)
	if err != nil || !exists {
		return 0, false, err
	}
	var maxVersion sql.NullInt64
	query := "SELECT COALESCE(MAX(version_id), 0) FROM " + VersionTableName
	if err := sqlDB.QueryRowContext(ctx, query).Scan(&maxVersion); err != nil {
		return 0, false, fmt.Errorf("migrate: read version: %w", err)
	}
	if !maxVersion.Valid || maxVersion.Int64 == 0 {
		return 0, false, nil
	}
	return maxVersion.Int64, true, nil
}

func versionTableExists(ctx context.Context, sqlDB *sql.DB, gooseDialect string) (bool, error) {
	var query string
	switch gooseDialect {
	case "sqlite3":
		query = "SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = '" + VersionTableName + "'"
	case "postgres":
		query = "SELECT COUNT(1) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = '" + VersionTableName + "'"
	case "mssql":
		query = "SELECT COUNT(1) FROM sys.objects WHERE type = 'U' AND name = '" + VersionTableName + "'"
	default:
		query = "SELECT COUNT(1) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = '" + VersionTableName + "'"
	}
	var count int
	if err := sqlDB.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return false, fmt.Errorf("migrate: probe version table: %w", err)
	}
	return count > 0, nil
}
