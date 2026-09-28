package svc

// D 批覆盖补齐（migrations.go 0032-0035 四个迁移的错误分支）。完全复用
// C 批两类确定性注入（coverage_c_migrations_test.go）：
//  1. 全查询失败连接 → wrapGorm err 透传分支；
//  2. PRAGMA query_only 拒写连接 → CreateTable / AddColumn 失败分支。
// 0032/0033/0035 迁移体是 NewGoMigration 内联闭包，经导出字段
// UpFnNoTxContext（goose 对 RunDB 的映射）直调；0034 体本身抽出为
// migrateProviderMetadataTable 可直调。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

// 探针失败连接：四个迁移体的 wrapGorm err 透传分支。
func TestCoverageD_Migrations_WrapGormProbeError(t *testing.T) {
	db := probeFailingDBC()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	mig32 := adminOtpRecoveryCodesMigration()
	require.NotNil(t, mig32.UpFnNoTxContext, "0032 应带 RunDB 闭包")
	require.Error(t, mig32.UpFnNoTxContext(ctx, db))

	mig33 := adminPasswordPolicyMigration()
	require.NotNil(t, mig33.UpFnNoTxContext, "0033 应带 RunDB 闭包")
	require.Error(t, mig33.UpFnNoTxContext(ctx, db))

	require.Error(t, migrateProviderMetadataTable(ctx, db))

	mig35 := bugTicketLinkMigration()
	require.NotNil(t, mig35.UpFnNoTxContext, "0035 应带 RunDB 闭包")
	require.Error(t, mig35.UpFnNoTxContext(ctx, db))
}

// 0032：admin_otp_recovery_codes 缺表 → CreateTable 失败（query_only 拒写）。
func TestCoverageD_Mig0032_CreateTableError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	sqlDB := writeRefusingSQLiteDBC(t, db)

	mig := adminOtpRecoveryCodesMigration()
	require.NotNil(t, mig.UpFnNoTxContext)
	err := mig.UpFnNoTxContext(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0032")
}

// 0033：admins 表存在但缺密码策略两列 → AddColumn 失败。
func TestCoverageD_Mig0033_AddColumnError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	require.NoError(t, db.AutoMigrate(&model.Admin{}))
	require.NoError(t, db.Migrator().DropColumn(&model.Admin{}, "MustChangePassword"))
	require.NoError(t, db.Migrator().DropColumn(&model.Admin{}, "PasswordExpiresAt"))
	sqlDB := writeRefusingSQLiteDBC(t, db)

	mig := adminPasswordPolicyMigration()
	require.NotNil(t, mig.UpFnNoTxContext)
	err := mig.UpFnNoTxContext(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0033")
}

// 0034：provider_metadata 缺表 → CreateTable 失败。
func TestCoverageD_Mig0034_CreateTableError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	sqlDB := writeRefusingSQLiteDBC(t, db)

	err := migrateProviderMetadataTable(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0034")
}

// 0035：bug_ticket_links 缺表 → CreateTable 失败。
func TestCoverageD_Mig0035_CreateTableError(t *testing.T) {
	db := openSharedMemSQLiteC(t)
	sqlDB := writeRefusingSQLiteDBC(t, db)

	mig := bugTicketLinkMigration()
	require.NotNil(t, mig.UpFnNoTxContext)
	err := mig.UpFnNoTxContext(context.Background(), sqlDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0035")
}
