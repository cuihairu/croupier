package svc

import (
	"context"
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
)

// 0052 建 provision_runs：新表无存量约束名漂移（0023 教训），HasTable 检查后
// CreateTable；重复运行幂等；模型列集与表一致（round-trip）。
func TestProvisionRunsTableMigration_CreatesAndIdempotent(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	ctx := context.Background()

	// 首次：建表。
	require.NoError(t, migrateProvisionRunsTable(ctx, sqlDB))
	assert.True(t, db.Migrator().HasTable(&model.ProvisionRun{}))

	// 幂等：再跑不报错、表仍在。
	require.NoError(t, migrateProvisionRunsTable(ctx, sqlDB))
	assert.True(t, db.Migrator().HasTable(&model.ProvisionRun{}))

	// 模型 round-trip：写入读回字段一致。
	m := model.NewProvisionRunModel(db)
	run := &model.ProvisionRun{
		RunID:       "prov-test1",
		Provider:    "aws",
		Region:      "us-east-1",
		GameID:      "demo",
		Env:         "prod",
		Policy:      "direct",
		Count:       1,
		State:       "adopted",
		ApprovalID:  "",
		RequestedBy: "admin",
	}
	require.NoError(t, m.Create(ctx, run))
	got, err := m.FindByRunID(ctx, "prov-test1")
	require.NoError(t, err)
	assert.Equal(t, "aws", got.Provider)
	assert.Equal(t, "us-east-1", got.Region)
	assert.Equal(t, "adopted", got.State)
}

// 迁移注册表必须含 0052（防 registerSvcMigrations 漏配）：重置全局注册后
// 重跑 registerSvcMigrations，再单独注册 0052 必须命中 already registered
// ——证明 52 已在全局集（与 TestRegisterSvcMigrations_PanicsOnDuplicateRegistration
// 同一恢复现场模式；顺序执行）。
func TestRegisterSvcMigrations_ContainsProvisionRuns(t *testing.T) {
	goose.ResetGlobalMigrations()
	registerSvcMigrations()
	err := goose.SetGlobalMigrations(provisionRunsTableMigration())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version 52 already registered")
}

// 0053 加 destroy_protected 列（OPS-002 数据盘护栏）：补列 0025 模式、
// 幂等、缺表跳过（fanout 重放到无该表的库不报错、不建空壳）；存量行默认
// false=未护栏。
func TestProvisionRunDestroyProtectionMigration_AddsColumnIdempotent(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	ctx := context.Background()

	// 缺表跳过：无该表时不报错、不建空壳。
	require.NoError(t, migrateProvisionRunDestroyProtection(ctx, sqlDB))
	assert.False(t, db.Migrator().HasTable(&model.ProvisionRun{}))

	// 先跑 0052 建表，再跑 0053 补列。0052 的 CreateTable 用当前模型（已含
	// destroy_protected 字段）会一并建出该列；为复刻「0052 时代老表（无该
	// 列）」，先 DROP 掉该列再跑 0053——这才是存量库真实形态（迁移契约：
	// 回归用例须复刻存量形态，非 AutoMigrate 新表）。
	require.NoError(t, migrateProvisionRunsTable(ctx, sqlDB))
	assert.True(t, db.Migrator().HasColumn(&model.ProvisionRun{}, "destroy_protected"))
	require.NoError(t, db.Migrator().DropColumn(&model.ProvisionRun{}, "DestroyProtected"))
	assert.False(t, db.Migrator().HasColumn(&model.ProvisionRun{}, "destroy_protected"))
	require.NoError(t, migrateProvisionRunDestroyProtection(ctx, sqlDB))
	assert.True(t, db.Migrator().HasColumn(&model.ProvisionRun{}, "destroy_protected"))

	// 幂等：再跑不报错。
	require.NoError(t, migrateProvisionRunDestroyProtection(ctx, sqlDB))
	assert.True(t, db.Migrator().HasColumn(&model.ProvisionRun{}, "destroy_protected"))

	// 存量行默认 false：插入不含 DestroyProtected 显式值 → 读回 false。
	m := model.NewProvisionRunModel(db)
	run := &model.ProvisionRun{
		RunID:    "prov-prot",
		Provider: "aws",
		Region:   "us-east-1",
		GameID:   "demo",
		Env:      "prod",
		Policy:   "direct",
		Count:    1,
		State:    "adopted",
	}
	require.NoError(t, m.Create(ctx, run))
	got, err := m.FindByRunID(ctx, "prov-prot")
	require.NoError(t, err)
	assert.False(t, got.DestroyProtected)

	// 写 true 回读 true（round-trip）。
	got.DestroyProtected = true
	require.NoError(t, m.Update(ctx, got))
	got2, err := m.FindByRunID(ctx, "prov-prot")
	require.NoError(t, err)
	assert.True(t, got2.DestroyProtected)
}

// 迁移注册表必须含 0053（防 registerSvcMigrations 漏配）。
func TestRegisterSvcMigrations_ContainsProvisionDestroyProtection(t *testing.T) {
	goose.ResetGlobalMigrations()
	registerSvcMigrations()
	err := goose.SetGlobalMigrations(provisionRunDestroyProtectionMigration())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version 53 already registered")
}

// 0054 加 destroy_approval_id 列（OPS-003 审批双模式 workflow）：补列 0025
// 模式、幂等、缺表跳过（fanout 重放到无该表的库不报错、不建空壳）；存量行
// 空=未申请销毁审批。
func TestProvisionRunDestroyApprovalMigration_AddsColumnIdempotent(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	ctx := context.Background()

	// 缺表跳过：无该表时不报错、不建空壳。
	require.NoError(t, migrateProvisionRunDestroyApproval(ctx, sqlDB))
	assert.False(t, db.Migrator().HasTable(&model.ProvisionRun{}))

	// 先跑 0052 建表，再 DROP 掉 0054 列复刻存量形态，然后跑 0054 补列。
	require.NoError(t, migrateProvisionRunsTable(ctx, sqlDB))
	assert.True(t, db.Migrator().HasColumn(&model.ProvisionRun{}, "destroy_approval_id"))
	require.NoError(t, db.Migrator().DropColumn(&model.ProvisionRun{}, "DestroyApprovalID"))
	assert.False(t, db.Migrator().HasColumn(&model.ProvisionRun{}, "destroy_approval_id"))
	require.NoError(t, migrateProvisionRunDestroyApproval(ctx, sqlDB))
	assert.True(t, db.Migrator().HasColumn(&model.ProvisionRun{}, "destroy_approval_id"))

	// 幂等：再跑不报错。
	require.NoError(t, migrateProvisionRunDestroyApproval(ctx, sqlDB))
	assert.True(t, db.Migrator().HasColumn(&model.ProvisionRun{}, "destroy_approval_id"))

	// 存量行空：插入不显式赋值 → 读回空。
	m := model.NewProvisionRunModel(db)
	run := &model.ProvisionRun{
		RunID:    "prov-appr",
		Provider: "aws",
		Region:   "us-east-1",
		GameID:   "demo",
		Env:      "prod",
		Policy:   "ticket",
		Count:    1,
		State:    "destroy_pending",
	}
	require.NoError(t, m.Create(ctx, run))
	got, err := m.FindByRunID(ctx, "prov-appr")
	require.NoError(t, err)
	assert.Empty(t, got.DestroyApprovalID)

	// 写审批 id 回读一致（round-trip）。
	got.DestroyApprovalID = "appr-1"
	require.NoError(t, m.Update(ctx, got))
	got2, err := m.FindByRunID(ctx, "prov-appr")
	require.NoError(t, err)
	assert.Equal(t, "appr-1", got2.DestroyApprovalID)
}

// 迁移注册表必须含 0054（防 registerSvcMigrations 漏配）。
func TestRegisterSvcMigrations_ContainsProvisionDestroyApproval(t *testing.T) {
	goose.ResetGlobalMigrations()
	registerSvcMigrations()
	err := goose.SetGlobalMigrations(provisionRunDestroyApprovalMigration())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version 54 already registered")
}

// 0059 加 cloud_init_extra 列（#657 ticket 续行保留非敏感自定义片段）：
// 补列 0025 模式、幂等、缺表跳过；存量行空=未提供；round-trip 一致。
func TestProvisionRunCloudInitExtraMigration_AddsColumnIdempotent(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	ctx := context.Background()

	// 缺表跳过：无该表时不报错、不建空壳。
	require.NoError(t, migrateProvisionRunCloudInitExtra(ctx, sqlDB))
	assert.False(t, db.Migrator().HasTable(&model.ProvisionRun{}))

	// 先跑 0052 建表，再 DROP 掉 0059 列复刻存量形态，然后跑 0059 补列。
	require.NoError(t, migrateProvisionRunsTable(ctx, sqlDB))
	assert.True(t, db.Migrator().HasColumn(&model.ProvisionRun{}, "cloud_init_extra"))
	require.NoError(t, db.Migrator().DropColumn(&model.ProvisionRun{}, "CloudInitExtra"))
	assert.False(t, db.Migrator().HasColumn(&model.ProvisionRun{}, "cloud_init_extra"))
	require.NoError(t, migrateProvisionRunCloudInitExtra(ctx, sqlDB))
	assert.True(t, db.Migrator().HasColumn(&model.ProvisionRun{}, "cloud_init_extra"))

	// 幂等：再跑不报错。
	require.NoError(t, migrateProvisionRunCloudInitExtra(ctx, sqlDB))
	assert.True(t, db.Migrator().HasColumn(&model.ProvisionRun{}, "cloud_init_extra"))

	// round-trip：写片段读回一致；存量行空=未提供。
	m := model.NewProvisionRunModel(db)
	run := &model.ProvisionRun{
		RunID:          "prov-cix",
		Provider:       "aws",
		Region:         "us-east-1",
		GameID:         "demo",
		Env:            "prod",
		Policy:         "ticket",
		Count:          1,
		State:          "pending",
		CloudInitExtra: "[ systemctl, enable, --now, croupier-agent ]",
	}
	require.NoError(t, m.Create(ctx, run))
	got, err := m.FindByRunID(ctx, "prov-cix")
	require.NoError(t, err)
	assert.Equal(t, "[ systemctl, enable, --now, croupier-agent ]", got.CloudInitExtra)
}

// 迁移注册表必须含 0059（防 registerSvcMigrations 漏配）。
func TestRegisterSvcMigrations_ContainsProvisionCloudInitExtra(t *testing.T) {
	goose.ResetGlobalMigrations()
	registerSvcMigrations()
	err := goose.SetGlobalMigrations(provisionRunCloudInitExtraMigration())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version 59 already registered")
}
