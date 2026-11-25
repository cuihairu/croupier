package migrate

import (
	"context"
	"errors"
	"io/fs"
	"regexp"
	"testing"
	"testing/fstest"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// ---- dialectOf variant coverage ----

func TestDialectOfVariants(t *testing.T) {
	assert.Equal(t, "mysql", dialectOf("mariadb"))
	assert.Equal(t, "postgres", dialectOf("PostgreSQL"))
	assert.Equal(t, "mssql", dialectOf("SQLServer"))
	assert.Equal(t, "sqlite3", dialectOf(" SQLite3 "))
	assert.Equal(t, "", dialectOf("oracle"))
}

// ---- EnsureUpToDate error branches ----

func TestEnsureUpToDateNilDB(t *testing.T) {
	_, err := EnsureUpToDate(context.Background(), nil, ScopeMeta, nil)
	assert.ErrorContains(t, err, "nil gorm DB")
}

// ---- ensureUpToDate failure paths (real sqlite database) ----

func TestEnsureUpToDateVersionBelowRequired(t *testing.T) {
	db := openTestDB(t)
	fsys := mapFS(map[string]string{
		"0001_a.sql": "-- +goose Up\nSELECT 1;\n\n-- +goose Down\nSELECT 1;\n",
		"0002_b.sql": "-- +goose Up\nSELECT 1;\n\n-- +goose Down\nSELECT 1;\n",
	})
	_, err := ensureUpToDate(context.Background(), db, fsys, ScopeSingle, nil)
	// Only two migrations → version 2 < MinimumRequiredVersion(15)
	assert.ErrorContains(t, err, "below required")
}

func TestEnsureUpToDateBaselineFailure(t *testing.T) {
	db := openTestDB(t)
	fsys := mapFS(map[string]string{
		"0001_a.sql": "-- +goose Up\nSELECT 1;\n\n-- +goose Down\nSELECT 1;\n",
	})
	_, err := ensureUpToDate(context.Background(), db, fsys, ScopeSingle,
		func(*gorm.DB) error { return errors.New("baseline exploded") })
	assert.ErrorContains(t, err, "baseline")
}

func TestEnsureUpToDateMigrationFailure(t *testing.T) {
	db := openTestDB(t)
	fsys := mapFS(map[string]string{
		"0001_bad.sql": "-- +goose Up\nTHIS IS NOT SQL;\n\n-- +goose Down\nSELECT 1;\n",
	})
	_, err := ensureUpToDate(context.Background(), db, fsys, ScopeSingle, nil)
	assert.ErrorContains(t, err, "up")
}

// ---- CurrentVersion branches ----

func TestCurrentVersionZeroRowMeansNeverMigrated(t *testing.T) {
	db := openTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE "+VersionTableName+" (version_id BIGINT)").Error)

	version, ok, err := CurrentVersion(context.Background(), sqlDB, "sqlite")
	require.NoError(t, err)
	assert.False(t, ok, "version 0 means never migrated")
	assert.Equal(t, int64(0), version)

	require.NoError(t, db.Exec("INSERT INTO "+VersionTableName+" (version_id) VALUES (7)").Error)
	require.NoError(t, err)
	version, ok, err = CurrentVersion(context.Background(), sqlDB, "sqlite")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, int64(7), version)
}

// ---- acquireSessionLock / versionTableExists mysql/pg/mssql branches (sqlmock) ----

func TestAcquireSessionLockMySQL(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectQuery("SELECT GET_LOCK").
		WithArgs(mysqlMigrationLockName).
		WillReturnRows(sqlmock.NewRows([]string{"l"}).AddRow(int64(1)))
	mock.ExpectExec("SELECT RELEASE_LOCK").
		WithArgs(mysqlMigrationLockName).
		WillReturnResult(sqlmock.NewResult(0, 0))

	release, err := acquireSessionLock(context.Background(), sqlDB, "mysql")
	require.NoError(t, err)
	release()
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAcquireSessionLockPostgresAndMSSQL(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectQuery(regexp.QuoteMeta("SELECT pg_try_advisory_lock(1668444021, 1885955442)")).
		WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_unlock(1668444021, 1885955442)")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("DECLARE @r int; EXEC @r = sp_getapplock @Resource = N'croupier_schema_migration', @LockMode = 'Exclusive', @LockOwner = 'Session', @LockTimeout = 0; SELECT @r")).
		WillReturnRows(sqlmock.NewRows([]string{"r"}).AddRow(int64(0)))
	mock.ExpectExec(regexp.QuoteMeta("EXEC sp_releaseapplock @Resource = N'croupier_schema_migration', @LockOwner = 'Session'")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	release, err := acquireSessionLock(context.Background(), sqlDB, "postgres")
	require.NoError(t, err)
	release()
	release2, err := acquireSessionLock(context.Background(), sqlDB, "mssql")
	require.NoError(t, err)
	release2()
}

func TestAcquireSessionLockDialFailure(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	_ = sqlDB.Close() // close early so Conn(ctx) fails

	_, err = acquireSessionLock(context.Background(), sqlDB, "postgres")
	assert.ErrorContains(t, err, "dial lock connection")
}

func TestVersionTableExistsDialects(t *testing.T) {
	ctx := context.Background()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT COUNT(1) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'goose_db_version'")).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	exists, err := versionTableExists(ctx, sqlDB, "mysql")
	require.NoError(t, err)
	assert.True(t, exists)

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT COUNT(1) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'goose_db_version'")).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
	exists, err = versionTableExists(ctx, sqlDB, "postgres")
	require.NoError(t, err)
	assert.False(t, exists)

	// Probe failure
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT COUNT(1) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'goose_db_version'")).
		WillReturnError(errors.New("boom"))
	_, err = versionTableExists(ctx, sqlDB, "mysql")
	assert.ErrorContains(t, err, "probe version table")
}

// EnsureUpToDate public entry point: sqlite dialect + embedded migrations subdirectory.
func TestEnsureUpToDateEmbeddedBelowRequired(t *testing.T) {
	db := openTestDB(t)
	_, err := EnsureUpToDate(context.Background(), db, ScopeSingle,
		func(*gorm.DB) error { return db.Exec("SELECT 1").Error })
	// 0001 is the only embedded SQL migration → version 1 < 15 (Go migrations are registered at the svc layer)
	assert.ErrorContains(t, err, "below required")
}

// ctx cancellation branch: GET_LOCK never grants → the loop exits on ctx timeout.
func TestAcquireSessionLockContextCancelled(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	for i := 0; i < 3; i++ {
		mock.ExpectQuery("SELECT GET_LOCK").
			WithArgs(mysqlMigrationLockName).
			WillReturnRows(sqlmock.NewRows([]string{"l"}).AddRow(int64(0)))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	_, err = acquireSessionLock(ctx, sqlDB, "mysql")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// Contention timeout branch: the lock is never acquired within 60s → failure (covered equivalently by cancelling ctx early).
func TestVersionTableExistsMSSQL(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT COUNT(1) FROM sys.objects WHERE type = 'U' AND name = 'goose_db_version'")).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	exists, err := versionTableExists(context.Background(), sqlDB, "mssql")
	require.NoError(t, err)
	assert.True(t, exists)
}

// ---- acquireSessionLock: per-dialect lock query failures ----

func TestAcquireSessionLockQueryErrors(t *testing.T) {
	// mysql: GET_LOCK query failure
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	mock.ExpectQuery("SELECT GET_LOCK").WillReturnError(errors.New("boom"))
	_, err = acquireSessionLock(context.Background(), sqlDB, "mysql")
	assert.ErrorContains(t, err, "acquire session lock")
	assert.NoError(t, mock.ExpectationsWereMet())

	// postgres: advisory lock query failure
	sqlDB2, mock2, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB2.Close() })
	mock2.ExpectQuery("SELECT pg_try_advisory_lock").WillReturnError(errors.New("boom"))
	_, err = acquireSessionLock(context.Background(), sqlDB2, "postgres")
	assert.ErrorContains(t, err, "acquire session lock")

	// mssql: sp_getapplock failure
	sqlDB3, mock3, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB3.Close() })
	mock3.ExpectQuery("sp_getapplock").WillReturnError(errors.New("boom"))
	_, err = acquireSessionLock(context.Background(), sqlDB3, "mssql")
	assert.ErrorContains(t, err, "acquire session lock")

	// unknown dialect: return an empty release (no-op)
	release, err := acquireSessionLock(context.Background(), sqlDB3, "unknown")
	require.NoError(t, err)
	release()
}

// ---- EnsureUpToDate: unsupported dialect / underlying unwrap failure / lock failure ----

func TestEnsureUpToDateUnsupportedDialect(t *testing.T) {
	db := &gorm.DB{Config: &gorm.Config{Dialector: fakeDialector{name: "oracle"}}}
	_, err := EnsureUpToDate(context.Background(), db, ScopeMeta, nil)
	assert.ErrorContains(t, err, "unsupported dialect")
}

func TestEnsureUpToDateUnwrapFailure(t *testing.T) {
	// gorm.DB that was never opened: db.DB() fails
	db := &gorm.DB{Config: &gorm.Config{Dialector: fakeDialector{name: "sqlite"}}}
	_, err := ensureUpToDate(context.Background(), db, nil, ScopeSingle, nil)
	assert.Error(t, err)
}

func TestEnsureUpToDateSessionLockFailure(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)

	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	_ = sqlDB.Close() // close after open succeeds: Conn fails → lock acquisition fails

	_, err = EnsureUpToDate(context.Background(), db, ScopeMeta, nil)
	assert.ErrorContains(t, err, "dial lock connection")
}

func TestEnsureUpToDateVersionProbeFailure(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectQuery("SELECT GET_LOCK").
		WithArgs(mysqlMigrationLockName).
		WillReturnRows(sqlmock.NewRows([]string{"l"}).AddRow(int64(1)))
	// version table probe failure
	mock.ExpectQuery("information_schema.tables").WillReturnError(errors.New("probe boom"))
	mock.ExpectExec("SELECT RELEASE_LOCK").WillReturnResult(sqlmock.NewResult(0, 0))

	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)

	_, err = EnsureUpToDate(context.Background(), db, ScopeMeta, nil)
	assert.ErrorContains(t, err, "probe version table")
}

// fakeDialector only serves Dialector.Name() probing.
type fakeDialector struct{ name string }

func (f fakeDialector) Name() string { return f.name }

func (fakeDialector) Initialize(*gorm.DB) error { return errors.New("not implemented") }
func (fakeDialector) Migrator(*gorm.DB) gorm.Migrator {
	return nil
}
func (fakeDialector) DataTypeOf(*schema.Field) string { return "" }
func (fakeDialector) DefaultValueOf(*schema.Field) clause.Expression {
	return clause.Expr{}
}
func (fakeDialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, v interface{}) {}
func (fakeDialector) QuoteTo(writer clause.Writer, str string)                            {}
func (fakeDialector) Explain(sql string, vars ...interface{}) string                      { return "" }

// ---- CurrentVersion: version read failure ----

func TestCurrentVersionReadError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectQuery("sqlite_master").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version")).
		WillReturnError(errors.New("read boom"))

	_, ok, err := CurrentVersion(context.Background(), sqlDB, "sqlite")
	assert.Error(t, err)
	assert.False(t, ok)
}

// ---- ensureUpToDate: goose provider and version-read boundaries ----

type failingFS struct{}

func (failingFS) Open(string) (fs.File, error) { return nil, errors.New("fs boom") }

func TestEnsureUpToDateProviderFailure(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	_, err = ensureUpToDate(context.Background(), db, failingFS{}, ScopeMeta, nil)
	assert.ErrorContains(t, err, "provider")
}

func TestEnsureUpToDateNoMigrationsFound(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	// no migrations at all in the directory: the goose provider rejects outright instead of passing silently
	fsys := fstest.MapFS{"readme.txt": &fstest.MapFile{Data: []byte("not a migration")}}
	_, err = ensureUpToDate(context.Background(), db, fsys, ScopeSingle, nil)
	assert.ErrorContains(t, err, "no migrations found")
}
