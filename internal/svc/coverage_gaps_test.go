package svc

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/platform/registry"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- truncate（此前 0%） ----

func TestTruncateShortAndOverflow(t *testing.T) {
	assert.Equal(t, "short", truncate("short", 10))
	assert.Equal(t, "0123456789...", truncate("0123456789X", 10))
}

// ---- StartScheduler / StopScheduler（此前 0%） ----

func TestSchedulerNilGuards(t *testing.T) {
	var nilCtx *ServiceContext
	assert.Nil(t, nilCtx.StartScheduler())
	nilCtx.StopScheduler() // 不 panic

	ctx := &ServiceContext{}
	assert.Nil(t, ctx.StartScheduler()) // 依赖缺失
	ctx.StopScheduler()
}

func TestSchedulerStartStopWithDeps(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/sched.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.TaskSchedule{}, &model.TaskRun{}, &model.TaskScheduleRunLog{}))

	ctx := &ServiceContext{
		TaskScheduleModel: model.NewTaskScheduleModel(db),
		Dispatcher:        dispatch.NewDispatcher(registry.NewStore()),
	}
	mgr := ctx.StartScheduler()
	require.NotNil(t, mgr)
	// 幂等：重复启动返回同一实例
	assert.Same(t, mgr, ctx.StartScheduler())
	ctx.StopScheduler()
}

// ---- openGorm sqlite 分支（44.9%） ----

func TestOpenGormSQLiteVariants(t *testing.T) {
	db, err := openGorm("sqlite", ":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Exec("SELECT 1").Error)

	db2, err := openGorm("sqlite3", t.TempDir()+"/x.db")
	require.NoError(t, err)
	require.NoError(t, db2.Exec("SELECT 1").Error)

	_, err = openGorm("oracle", "whatever")
	assert.Error(t, err)
}

// ---- probeDialect（27.3%）：mysql 命中与全部失败 ----

func TestProbeDialectMySQLViaMock(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectQuery("SELECT COUNT").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT CURRENT_SETTING").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT @@version_comment").WillReturnRows(
		sqlmock.NewRows([]string{"v"}).AddRow("MySQL Community"))

	dialect, err := probeDialect(sqlDB)
	require.NoError(t, err)
	assert.Equal(t, "mysql", dialect)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProbeDialectUnknown(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectQuery("SELECT COUNT").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT CURRENT_SETTING").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT @@version_comment").WillReturnError(assert.AnError)

	dialect, err := probeDialect(sqlDB)
	assert.Error(t, err)
	assert.Empty(t, dialect)
}

// ---- wrapGorm（61.5%）：mysql 连接包装与探测失败 ----

func TestWrapGormMySQLConnAndProbeFailure(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectQuery("SELECT COUNT").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT CURRENT_SETTING").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT @@version_comment").WillReturnRows(
		sqlmock.NewRows([]string{"v"}).AddRow("MySQL"))

	gdb, err := wrapGorm(sqlDB)
	require.NoError(t, err)
	require.NotNil(t, gdb)

	sqlDB2, mock2, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB2.Close() })
	mock2.ExpectQuery("SELECT COUNT").WillReturnError(assert.AnError)
	mock2.ExpectQuery("SELECT CURRENT_SETTING").WillReturnError(assert.AnError)
	mock2.ExpectQuery("SELECT @@version_comment").WillReturnError(assert.AnError)
	_, err = wrapGorm(sqlDB2)
	assert.Error(t, err)
}

// ---- sqliteFileDSN（66.7%） ----

func TestSQLiteFileDSNPragmaInjection(t *testing.T) {
	assert.Contains(t, sqliteFileDSN("/tmp/a.db"), "_pragma=busy_timeout")
	assert.Contains(t, sqliteFileDSN("file:x.db?cache=shared"), "_pragma=busy_timeout")
}
