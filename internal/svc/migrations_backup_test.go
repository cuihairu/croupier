package svc

// 0060：backups.game_id/env/source_id/source_name 列（OPS-008 游戏库备份
// 归属经 capture 所见数据源登记）。回归用例复刻存量形态：手工 CREATE TABLE
// 只有旧列（无归属四列）→ AddColumn 补齐 + idx_backup_scope 复合索引；幂等；
// 缺表跳过；存量行三列空=平台级备份（行为与现状一致）。

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
)

func legacyBackupsTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`CREATE TABLE backups (
		id integer primary key autoincrement,
		created_at datetime, updated_at datetime, deleted_at datetime,
		backup_id text, name text, size integer, type text, status text,
		location text, checksum text, error_message text, completed_at datetime)`).Error)
}

func TestBackupScopeColumnsMigration_LegacyTableShape(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m60.db"), &gorm.Config{})
	require.NoError(t, err)
	legacyBackupsTable(t, db)
	require.NoError(t, db.Exec(`INSERT INTO backups
		(created_at, backup_id, name, type, status)
		VALUES (datetime(), 'legacy-meta-backup', 'meta-full', 'full', 'succeeded')`).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateBackupScopeColumns(context.Background(), sqlDB))

	require.True(t, db.Migrator().HasColumn(&model.Backup{}, "GameID"))
	require.True(t, db.Migrator().HasColumn(&model.Backup{}, "Env"))
	require.True(t, db.Migrator().HasColumn(&model.Backup{}, "SourceID"))
	require.True(t, db.Migrator().HasColumn(&model.Backup{}, "SourceName"))
	require.True(t, db.Migrator().HasIndex(&model.Backup{}, "idx_backup_scope"))

	// 存量行不回填：三列空=平台级备份。
	var gameID, env, sourceName string
	var sourceID uint
	row := db.Raw(
		`SELECT COALESCE(game_id,''), COALESCE(env,''), COALESCE(source_id,0), COALESCE(source_name,'')
		 FROM backups WHERE backup_id = 'legacy-meta-backup'`).Row()
	require.NoError(t, row.Scan(&gameID, &env, &sourceID, &sourceName))
	require.Empty(t, gameID, "legacy row keeps empty game_id")
	require.Empty(t, env, "legacy row keeps empty env")
	require.Zero(t, sourceID, "legacy row keeps zero source_id")
	require.Empty(t, sourceName, "legacy row keeps empty source_name")

	// 幂等。
	require.NoError(t, migrateBackupScopeColumns(context.Background(), sqlDB))
}

func TestBackupScopeColumnsMigration_MissingTableSkips(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m60skip.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateBackupScopeColumns(context.Background(), sqlDB))
	require.False(t, db.Migrator().HasTable(&model.Backup{}))
}

// 模型侧配套：迁移与模型列集不漂移（老库过迁移 → 模型字段可读写）。
func TestBackupScopeColumnsMigration_RoundTripsThroughModelColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m60round.db"), &gorm.Config{})
	require.NoError(t, err)
	legacyBackupsTable(t, db)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateBackupScopeColumns(context.Background(), sqlDB))
	require.NoError(t, db.Exec(`INSERT INTO backups
		(created_at, backup_id, name, type, status, game_id, env, source_id, source_name)
		VALUES (datetime(), 'game-backup-1', 'demo-prod-full', 'full', 'succeeded',
		        'demo', 'prod', 7, 'demo-game-db')`).Error)

	var b model.Backup
	require.NoError(t, db.Where("backup_id = ?", "game-backup-1").First(&b).Error)
	require.Equal(t, "demo", b.GameID)
	require.Equal(t, "prod", b.Env)
	require.Equal(t, uint(7), b.SourceID)
	require.Equal(t, "demo-game-db", b.SourceName)
}
