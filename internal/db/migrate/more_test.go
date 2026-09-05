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

// ---- acquireSessionLock：各方言加锁查询失败 ----

func TestAcquireSessionLockQueryErrors(t *testing.T) {
	// mysql：GET_LOCK 查询失败
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	mock.ExpectQuery("SELECT GET_LOCK").WillReturnError(errors.New("boom"))
	_, err = acquireSessionLock(context.Background(), sqlDB, "mysql")
	assert.ErrorContains(t, err, "acquire session lock")
	assert.NoError(t, mock.ExpectationsWereMet())

	// postgres：advisory lock 查询失败
	sqlDB2, mock2, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB2.Close() })
	mock2.ExpectQuery("SELECT pg_try_advisory_lock").WillReturnError(errors.New("boom"))
	_, err = acquireSessionLock(context.Background(), sqlDB2, "postgres")
	assert.ErrorContains(t, err, "acquire session lock")

	// mssql：sp_getapplock 失败
	sqlDB3, mock3, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB3.Close() })
	mock3.ExpectQuery("sp_getapplock").WillReturnError(errors.New("boom"))
	_, err = acquireSessionLock(context.Background(), sqlDB3, "mssql")
	assert.ErrorContains(t, err, "acquire session lock")

	// 未知方言：直接空释放（no-op）
	release, err := acquireSessionLock(context.Background(), sqlDB3, "unknown")
	require.NoError(t, err)
	release()
}

// ---- EnsureUpToDate：不支持方言 / 底层 unwrap 失败 / 锁失败 ----

func TestEnsureUpToDateUnsupportedDialect(t *testing.T) {
	db := &gorm.DB{Config: &gorm.Config{Dialector: fakeDialector{name: "oracle"}}}
	_, err := EnsureUpToDate(context.Background(), db, ScopeMeta, nil)
	assert.ErrorContains(t, err, "unsupported dialect")
}

func TestEnsureUpToDateUnwrapFailure(t *testing.T) {
	// 未 open 的 gorm.DB：db.DB() 失败
	db := &gorm.DB{Config: &gorm.Config{Dialector: fakeDialector{name: "sqlite"}}}
	_, err := ensureUpToDate(context.Background(), db, nil, ScopeSingle, nil)
	assert.Error(t, err)
}

func TestEnsureUpToDateSessionLockFailure(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)

	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	_ = sqlDB.Close() // open 成功后再关闭：Conn 失败 → 锁获取失败

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
	// 版本表探测失败
	mock.ExpectQuery("information_schema.tables").WillReturnError(errors.New("probe boom"))
	mock.ExpectExec("SELECT RELEASE_LOCK").WillReturnResult(sqlmock.NewResult(0, 0))

	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)

	_, err = EnsureUpToDate(context.Background(), db, ScopeMeta, nil)
	assert.ErrorContains(t, err, "probe version table")
}

// fakeDialector 只服务于 Dialector.Name() 探测。
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

// ---- CurrentVersion：读取版本失败 ----

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

// ---- ensureUpToDate：goose provider 与版本读取边界 ----

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

	// 目录内没有任何迁移：goose provider 直接拒绝，不静默通过
	fsys := fstest.MapFS{"readme.txt": &fstest.MapFile{Data: []byte("not a migration")}}
	_, err = ensureUpToDate(context.Background(), db, fsys, ScopeSingle, nil)
	assert.ErrorContains(t, err, "no migrations found")
}
