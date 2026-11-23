package svc

// 0067 建 admin_refresh_tokens（AUD-052 刷新令牌轮换的持久状态）：新表无
// 存量约束名漂移（0023 教训），HasTable 检查后 CreateTable；重复运行幂等；
// 模型列集与表一致（round-trip：兑换/重放判定列全部可读写）。
import (
	"context"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
)

func TestAdminRefreshTokenTableMigration_CreatesAndIdempotent(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	ctx := context.Background()

	// 首次：建表。
	require.NoError(t, migrateAdminRefreshTokenTable(ctx, sqlDB))
	assert.True(t, db.Migrator().HasTable(&model.AdminRefreshToken{}))

	// 幂等：再跑不报错、表仍在。
	require.NoError(t, migrateAdminRefreshTokenTable(ctx, sqlDB))
	assert.True(t, db.Migrator().HasTable(&model.AdminRefreshToken{}))

	// 模型 round-trip：兑换/重放判定列（token_hash 唯一、used_at、revoked、
	// rotated_from_id）写入读回一致。
	m := model.NewAdminRefreshTokenModel(db)
	now := time.Now().UTC()
	row := &model.AdminRefreshToken{
		AdminID:       7,
		TokenHash:     model.HashRefreshToken("aud052-migration-token"),
		RotatedFromID: 0,
		ExpiresAt:     now.Add(7 * 24 * time.Hour),
		ClientIP:      "127.0.0.1",
		UserAgent:     "jest",
	}
	require.NoError(t, m.Create(ctx, row))
	require.NoError(t, m.MarkUsed(ctx, row.ID, now))

	got, err := m.FindByHash(ctx, model.HashRefreshToken("aud052-migration-token"))
	require.NoError(t, err)
	assert.Equal(t, uint(7), got.AdminID)
	assert.Equal(t, uint(0), got.RotatedFromID)
	require.NotNil(t, got.UsedAt)
	assert.False(t, got.Revoked)

	require.NoError(t, m.RevokeAllForAdmin(ctx, 7))
	revoked, err := m.FindByHash(ctx, model.HashRefreshToken("aud052-migration-token"))
	require.NoError(t, err)
	assert.True(t, revoked.Revoked)
}

// 迁移注册表必须含 0067（防 registerSvcMigrations 漏配）：重置全局注册后
// 重跑 registerSvcMigrations，再单独注册 0067 必须命中 already registered
// ——证明 67 已在全局集（与 0052 同一恢复现场模式；顺序执行）。
func TestRegisterSvcMigrations_ContainsAdminRefreshTokens(t *testing.T) {
	goose.ResetGlobalMigrations()
	registerSvcMigrations()
	err := goose.SetGlobalMigrations(adminRefreshTokenTableMigration())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version 67 already registered")
}
