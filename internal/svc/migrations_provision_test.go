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
