package svc

// 0066：external_tokens.game_allowlist/env_allowlist 两列（AUD-036 token 绑定
// game/env 允许面）。回归用例复刻存量形态：过 0041 的库只有旧列（无允许面）
// → AddColumn 逐列补齐；存量行两列 NULL = 不限，行为与现状一致；幂等；
// 缺表跳过（fanout 重放到无该表的库不报错、不建空壳）。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestExternalTokenScopeColumnsMigration_LegacyTableShape(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m66.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE external_tokens (
		id integer primary key autoincrement,
		created_at datetime, updated_at datetime, deleted_at datetime,
		name text NOT NULL,
		token_hash text NOT NULL,
		scope json,
		enabled numeric DEFAULT 1,
		last_used_at datetime,
		created_by text)`).Error)
	// 存量行：迁移后允许面留空（NULL），不回填
	require.NoError(t, db.Exec(
		`INSERT INTO external_tokens (created_at, updated_at, name, token_hash, enabled, created_by)
		VALUES (datetime(), datetime(), 'legacy-bot', 'abc123', 1, 'tester')`).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateExternalTokenScopeColumns(context.Background(), sqlDB))
	for _, col := range []string{"GameAllowlist", "EnvAllowlist"} {
		require.True(t, db.Migrator().HasColumn(&model.ExternalToken{}, col), col)
	}
	// 存量行允许面留空（NULL 或空串），不回填
	var nullCount int64
	require.NoError(t, db.Raw(
		`SELECT count(*) FROM external_tokens WHERE game_allowlist IS NULL AND env_allowlist IS NULL`).Scan(&nullCount).Error)
	require.EqualValues(t, 1, nullCount, "legacy rows keep empty allowlist")
	var name string
	require.NoError(t, db.Raw(`SELECT name FROM external_tokens WHERE token_hash = 'abc123'`).Scan(&name).Error)
	require.Equal(t, "legacy-bot", name, "legacy row must be untouched")
	// 幂等
	require.NoError(t, migrateExternalTokenScopeColumns(context.Background(), sqlDB))
}

func TestExternalTokenScopeColumnsMigration_MissingTableSkips(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m66skip.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateExternalTokenScopeColumns(context.Background(), sqlDB))
	require.False(t, db.Migrator().HasTable("external_tokens"))
}
