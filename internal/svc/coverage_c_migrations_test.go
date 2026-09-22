package svc

// C 批覆盖补齐（migrations.go 六个迁移体的错误分支）。两类确定性注入：
//  1. 全查询失败连接（Prepare 即错）→ probeDialect 四连探针全部落空 →
//     wrapGorm 返回错误（各迁移体入口的 wrapGorm err 透传分支）；
//  2. PRAGMA query_only=ON 的单连接内存 sqlite：读（sqlite_master 探测/
//     HasTable/HasColumn/HasIndex）全部成功、写（CREATE/ALTER/CREATE INDEX）
//     全部被拒绝，用于驱动 CreateTable / AddColumn / CreateIndex 的失败
//     返回分支。两种注入均为确定性错误，不依赖时序，也不触碰产品代码。

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// --- 注入工具 1：探针全失败的 *sql.DB ---

type refusePrepareConnC struct{}

func (refusePrepareConnC) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("coverage: prepare refused")
}
func (refusePrepareConnC) Close() error { return nil }
func (refusePrepareConnC) Begin() (driver.Tx, error) {
	return nil, errors.New("coverage: tx refused")
}

type refusePrepareConnectorC struct{}

func (refusePrepareConnectorC) Connect(context.Context) (driver.Conn, error) {
	return refusePrepareConnC{}, nil
}
func (refusePrepareConnectorC) Driver() driver.Driver { return nil }

// probeFailingDBC 返回所有查询都会失败的 *sql.DB（conn 未实现 Queryer，
// database/sql 走 Prepare 路径即错）。
func probeFailingDBC() *sql.DB {
	return sql.OpenDB(refusePrepareConnectorC{})
}

// wrapGorm 探针失败：六个迁移体共用同一入口分支（各自 return err）。
func TestCoverageC_Migrations_WrapGormProbeError(t *testing.T) {
	db := probeFailingDBC()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	require.Error(t, addContractExecutionStateColumn(ctx, db))
	require.Error(t, migrateSDKVersionHighwatermark(ctx, db))
	require.Error(t, migrateFunctionContractVersionTable(ctx, db))
	require.Error(t, migrateFunctionVersionFloorTable(ctx, db))
	require.Error(t, migrateContractRemovalPendingColumn(ctx, db))
	require.Error(t, migrateMenuItemTables(ctx, db))
}

// --- 注入工具 2：拒绝写入的内存 sqlite ---

var roDBSeqC atomic.Uint64

// writeRefusingSQLiteDBC 把已打开的 gorm sqlite 变成「读成功、写全拒」：
// 单连接保证 PRAGMA query_only 作用在迁移函数实际使用的连接上。
func writeRefusingSQLiteDBC(t *testing.T, db *gorm.DB) *sql.DB {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	_, err = sqlDB.Exec("PRAGMA query_only = ON")
	require.NoError(t, err)
	return sqlDB
}

// openSharedMemSQLiteC 打开一个独占命名的共享缓存内存库（各测试互不可见）。
func openSharedMemSQLiteC(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:cov_c_ro_%d?mode=memory&cache=shared", roDBSeqC.Add(1))), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	return db
}

// 0025：function_contracts 存在但缺 execution_state 列 → AddColumn 失败。
func TestCoverageC_Mig0025_AddColumnError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	require.NoError(t, db.AutoMigrate(&model.FunctionContract{}))
	require.NoError(t, db.Migrator().DropColumn(&model.FunctionContract{}, "ExecutionState"))
	sqlDB := writeRefusingSQLiteDBC(t, db)

	err := addContractExecutionStateColumn(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0025")
}

// 0028：sdk_version_highwatermarks 缺表 → CreateTable 失败。
func TestCoverageC_Mig0028_CreateTableError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	sqlDB := writeRefusingSQLiteDBC(t, db)

	err := migrateSDKVersionHighwatermark(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0028")
}

// 0029：function_contract_versions 缺表 → CreateTable 失败。
func TestCoverageC_Mig0029_CreateTableError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	sqlDB := writeRefusingSQLiteDBC(t, db)

	err := migrateFunctionContractVersionTable(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0029")
}

// 0030：function_version_floors 缺表 → CreateTable 失败。
func TestCoverageC_Mig0030_CreateTableError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	sqlDB := writeRefusingSQLiteDBC(t, db)

	err := migrateFunctionVersionFloorTable(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0030")
}

// 0031：removal_pending_at 列缺失 → AddColumn 失败。
func TestCoverageC_Mig0031_AddColumnError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	require.NoError(t, db.AutoMigrate(&model.FunctionContract{}))
	require.NoError(t, db.Migrator().DropColumn(&model.FunctionContract{}, "RemovalPendingAt"))
	sqlDB := writeRefusingSQLiteDBC(t, db)

	err := migrateContractRemovalPendingColumn(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0031")
}

// 0031：列已存在但清扫索引缺失 → CreateIndex 失败。
func TestCoverageC_Mig0031_CreateIndexError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	require.NoError(t, db.AutoMigrate(&model.FunctionContract{}))
	require.NoError(t, db.Migrator().DropIndex(&model.FunctionContract{}, "idx_function_contracts_removal_pending"))
	sqlDB := writeRefusingSQLiteDBC(t, db)

	err := migrateContractRemovalPendingColumn(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0031")
}

// 0027：menu_items 缺表 → CreateTable 失败。
func TestCoverageC_Mig0027_CreateTableError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	sqlDB := writeRefusingSQLiteDBC(t, db)

	err := migrateMenuItemTables(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0027")
}

// 0027：menu_items 已存在、page_specs 存在但缺 menu_id 列 → AddColumn 失败。
func TestCoverageC_Mig0027_AddColumnError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	require.NoError(t, db.AutoMigrate(&model.MenuItem{}, &model.PageSpec{}))
	require.NoError(t, db.Migrator().DropColumn(&model.PageSpec{}, "MenuID"))
	sqlDB := writeRefusingSQLiteDBC(t, db)

	err := migrateMenuItemTables(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0027")
}
