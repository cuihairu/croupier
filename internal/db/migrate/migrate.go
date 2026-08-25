// Package migrate implements the versioned database migration executor
// described in docs/architecture/database-migration-strategy.md.
//
// Contract:
//   - The first boot against a database that has no version table runs the
//     scoped legacy GORM AutoMigrate bridge exactly once (baseline), then
//     records the baseline migration version. This is the only path that may
//     execute AutoMigrate, and it is reserved for fresh/legacy databases.
//   - Every subsequent schema change MUST be a numbered SQL file under
//     migrations/ and is applied in order on the next boot (catch-up).
//   - A session-level database lock serializes concurrent server processes
//     during migration on MySQL/Postgres (SQLite is single-writer already).
package migrate

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// Scope identifies which logical database is being migrated. It exists for
// diagnostics; meta and game databases share the same execution contract.
type Scope string

const (
	ScopeMeta Scope = "meta"
	ScopeGame Scope = "game"
)

// VersionTableName is the table goose uses to record applied versions.
const VersionTableName = "goose_db_version"

// MinimumRequiredVersion is the lowest schema version required by this build.
// Bump it together with new migration files once the baseline era ends.
const MinimumRequiredVersion int64 = 1

func dialectOf(gormDialect string) string {
	switch strings.ToLower(strings.TrimSpace(gormDialect)) {
	case "mysql", "mariadb":
		return "mysql"
	case "postgres", "postgresql":
		return "postgres"
	case "sqlite", "sqlite3":
		return "sqlite3"
	default:
		return ""
	}
}

func sessionLockSupported(gooseDialect string) bool {
	return gooseDialect == "mysql" || gooseDialect == "postgres"
}

// mysqlSessionLocker implements goose's SessionLocker with MySQL named locks
// (GET_LOCK/RELEASE_LOCK), serializing concurrent server processes during
// migration. The lock is bound to the dedicated migration connection.
type mysqlSessionLocker struct{}

const mysqlMigrationLockName = "croupier_schema_migration"

func (l *mysqlSessionLocker) SessionLock(ctx context.Context, conn *sql.Conn) error {
	var result sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 60)", mysqlMigrationLockName).Scan(&result); err != nil {
		return fmt.Errorf("migrate: GET_LOCK: %w", err)
	}
	if !result.Valid || result.Int64 != 1 {
		return errors.New("migrate: could not acquire MySQL migration lock (another process may be migrating)")
	}
	return nil
}

func (l *mysqlSessionLocker) SessionUnlock(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, "SELECT RELEASE_LOCK(?)", mysqlMigrationLockName)
	if err != nil {
		return fmt.Errorf("migrate: RELEASE_LOCK: %w", err)
	}
	return nil
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

	opts := []goose.ProviderOption{}
	switch gooseDialect {
	case "postgres":
		if locker, err := lock.NewPostgresSessionLocker(); err == nil {
			opts = append(opts, goose.WithSessionLocker(locker))
		}
	case "mysql":
		opts = append(opts, goose.WithSessionLocker(&mysqlSessionLocker{}))
	}
	provider, err := goose.NewProvider(goose.Dialect(gooseDialect), sqlDB, fsys, opts...)
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
	default:
		query = "SELECT COUNT(1) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = '" + VersionTableName + "'"
	}
	var count int
	if err := sqlDB.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return false, fmt.Errorf("migrate: probe version table: %w", err)
	}
	return count > 0, nil
}
