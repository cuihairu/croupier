package svc

// 0049：audit_records.chain_version 补列回归（ARCH-008）。存量库的
// audit_records 没有 chain_version 列（过 0001 baseline 的老形态），
// 迁移必须幂等补列并让既有行落 DEFAULT 1（旧算法版本），既有哈希不改写。

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAuditChainVersionMigration_LegacyTableShape(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "m49.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	// 老形态：手工 CREATE TABLE，无 chain_version 列，带一行存量哈希。
	require.NoError(t, db.Exec(`CREATE TABLE audit_records (
		id integer primary key autoincrement,
		audit_id text NOT NULL UNIQUE,
		timestamp datetime NOT NULL,
		event_type text NOT NULL,
		category text NOT NULL,
		severity text NOT NULL,
		actor_json json,
		action text,
		resource_json json,
		details_json json,
		changes_json json,
		context_json json,
		outcome text NOT NULL,
		error_message text,
		chain_hash varchar(64) NOT NULL,
		chain_prev_hash varchar(64),
		chain_sequence integer NOT NULL,
		chain_signer_id varchar(255),
		chain_signature text,
		game_id varchar(100),
		env varchar(50),
		function_id varchar(255),
		duration_ms integer NOT NULL DEFAULT 0,
		ip varchar(64),
		actor_id varchar(255),
		created_at datetime NOT NULL
	)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO audit_records
		(audit_id, timestamp, event_type, category, severity, outcome, chain_hash, chain_sequence, created_at)
		VALUES ('legacy-1', datetime(), 'auth.login', 'security', 'info', 'success', 'abc123', 1, datetime())`).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateAuditChainVersionColumn(context.Background(), sqlDB))

	require.True(t, db.Migrator().HasColumn(&audit.AuditModel{}, "ChainVersion"))

	// 既有行回填为版本 1（旧算法），哈希值原样保留（绝不改写）。
	var row struct {
		ChainVersion int
		ChainHash    string
	}
	require.NoError(t, db.Raw(`SELECT chain_version, chain_hash FROM audit_records WHERE audit_id = 'legacy-1'`).Scan(&row).Error)
	require.Equal(t, 1, row.ChainVersion, "existing rows must backfill to the legacy chain version")
	require.Equal(t, "abc123", row.ChainHash, "existing hashes must never be rewritten")

	// 补列后表可正常写入（迁移未破坏表结构）。
	require.NoError(t, db.Exec(`INSERT INTO audit_records
		(audit_id, timestamp, event_type, category, severity, outcome, chain_hash, chain_sequence, chain_version, created_at)
		VALUES ('v2-1', datetime(), 'auth.login', 'security', 'info', 'success', 'def456', 2, 2, datetime())`).Error)

	// 幂等：重复执行不报错、不重复加列。
	require.NoError(t, migrateAuditChainVersionColumn(context.Background(), sqlDB))
}

func TestAuditChainVersionMigration_SkipsAbsentTable(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "m49-empty.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// 无 audit_records 表（multiGame fanout 到无该表的库）：跳过不报错、不建空壳。
	require.NoError(t, migrateAuditChainVersionColumn(context.Background(), sqlDB))
	require.False(t, db.Migrator().HasTable("audit_records"))
}
