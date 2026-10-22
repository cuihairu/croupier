package svc

// 0045：db_sources.owner_agent_id 列（DB 源周期探测归属 agent，
// agent-health-recovery-review.md §12 R4）。回归用例复刻存量形态：
// 手工 CREATE TABLE 只有旧列（无 owner_agent_id）→ AddColumn 补齐；幂等；
// 缺表跳过；存量行不回填（历史源无归属=未归属）。

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
)

func TestDBSourceOwnerMigration_LegacyTableShape(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m45.db"), &gorm.Config{})
	require.NoError(t, err)
	// 存量老库形态（过 0012 baseline 的表）：只有注册与阈值旧列。
	require.NoError(t, db.Exec(`CREATE TABLE db_sources (
		id integer primary key autoincrement,
		created_at datetime, updated_at datetime, deleted_at datetime,
		name text, driver text, kind text, dsn text,
		game_id text, env text,
		enabled integer default 1, sort integer default 0, created_by text,
		lock_wait_warn integer default 0, conn_warn_ratio integer default 0)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO db_sources
		(created_at, name, driver, kind, dsn, enabled)
		VALUES (datetime(), 'legacy-game-db', 'postgres', 'self', 'postgres://ro@h/db', 1)`).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateDBSourceOwnerColumn(context.Background(), sqlDB))
	require.True(t, db.Migrator().HasColumn(&model.DBSource{}, "OwnerAgentID"))

	// 存量行不回填：owner_agent_id 留空（未归属=server 诊断端点兜底）。
	var owner string
	require.NoError(t, db.Raw(
		`SELECT COALESCE(owner_agent_id, '') FROM db_sources WHERE name = 'legacy-game-db'`).
		Scan(&owner).Error)
	require.Empty(t, owner, "legacy row keeps empty owner_agent_id")

	// 幂等。
	require.NoError(t, migrateDBSourceOwnerColumn(context.Background(), sqlDB))
}

func TestDBSourceOwnerMigration_MissingTableSkips(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m45skip.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateDBSourceOwnerColumn(context.Background(), sqlDB))
	require.False(t, db.Migrator().HasTable(&model.DBSource{}))
}

// 模型侧配套：迁移与模型列集不漂移（老库过迁移 → 模型字段可读写）。
func TestDBSourceOwnerMigration_RoundTripsThroughModelColumn(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m45round.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE db_sources (
		id integer primary key autoincrement,
		created_at datetime, updated_at datetime, deleted_at datetime,
		name text, driver text, kind text, dsn text,
		game_id text, env text,
		enabled integer default 1, sort integer default 0, created_by text,
		lock_wait_warn integer default 0, conn_warn_ratio integer default 0)`).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateDBSourceOwnerColumn(context.Background(), sqlDB))
	require.NoError(t, db.Exec(`INSERT INTO db_sources
		(created_at, name, driver, kind, dsn, owner_agent_id)
		VALUES (datetime(), 'owned-game-db', 'mysql', 'self', 'root:pass@tcp(h)/g', 'agent-sidecar-1')`).Error)

	var src model.DBSource
	require.NoError(t, db.Where("name = ?", "owned-game-db").First(&src).Error)
	require.Equal(t, "agent-sidecar-1", src.OwnerAgentID)

	// 空归属 = 未归属（server 诊断端点兜底路径）。
	require.NoError(t, db.Model(&model.DBSource{}).
		Where("name = ?", "owned-game-db").Update("owner_agent_id", "").Error)
	require.NoError(t, db.Where("name = ?", "owned-game-db").First(&src).Error)
	require.Empty(t, src.OwnerAgentID)
}
