package migrate

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- dialectOf 变体补全 ----

func TestDialectOfVariants(t *testing.T) {
	assert.Equal(t, "mysql", dialectOf("mariadb"))
	assert.Equal(t, "postgres", dialectOf("PostgreSQL"))
	assert.Equal(t, "mssql", dialectOf("SQLServer"))
	assert.Equal(t, "sqlite3", dialectOf(" SQLite3 "))
	assert.Equal(t, "", dialectOf("oracle"))
}

// ---- EnsureUpToDate 错误分支 ----

func TestEnsureUpToDateNilDB(t *testing.T) {
	_, err := EnsureUpToDate(context.Background(), nil, ScopeMeta, nil)
	assert.ErrorContains(t, err, "nil gorm DB")
}

// ---- ensureUpToDate 失败路径（sqlite 真库） ----

func TestEnsureUpToDateVersionBelowRequired(t *testing.T) {
	db := openTestDB(t)
	fsys := mapFS(map[string]string{
		"0001_a.sql": "-- +goose Up\nSELECT 1;\n\n-- +goose Down\nSELECT 1;\n",
		"0002_b.sql": "-- +goose Up\nSELECT 1;\n\n-- +goose Down\nSELECT 1;\n",
	})
	_, err := ensureUpToDate(context.Background(), db, fsys, ScopeSingle, nil)
	// 仅两个迁移 → 版本 2 < MinimumRequiredVersion(15)
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

// ---- CurrentVersion 分支 ----

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

// ---- acquireSessionLock / versionTableExists 的 mysql/pg/mssql 分支（sqlmock） ----

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
	_ = sqlDB.Close() // 提前关闭让 Conn(ctx) 失败

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

	// 探测失败
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT COUNT(1) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'goose_db_version'")).
		WillReturnError(errors.New("boom"))
	_, err = versionTableExists(ctx, sqlDB, "mysql")
	assert.ErrorContains(t, err, "probe version table")
}

// EnsureUpToDate 公共入口：sqlite 方言 + 内嵌 migrations 子目录。
func TestEnsureUpToDateEmbeddedBelowRequired(t *testing.T) {
	db := openTestDB(t)
	_, err := EnsureUpToDate(context.Background(), db, ScopeSingle,
		func(*gorm.DB) error { return db.Exec("SELECT 1").Error })
	// 0001 是唯一内嵌 SQL 迁移 → 版本 1 < 15（Go 迁移由 svc 层注册）
	assert.ErrorContains(t, err, "below required")
}

// ctx 取消分支：GET_LOCK 永不授权 → 循环内 ctx 超时退出。
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

// 竞争超时分支：60s 内始终拿不到锁 → 失败（用 ctx 提前取消等价覆盖）。
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
