package svc

// 0050：approvals 补 claim/dispatch 列回归（ARCH-002）。存量库的 approvals
// 表没有 claim_status/claim_token/claimed_at/payload_digest 列，迁移必须幂等
// 补列且不破坏既有行；全新库则直接建表（fanout 到无 approvals 表的库）。

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/approvals"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestApprovalDispatchClaimMigration_AddsColumnsToExistingTable(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "m50.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	// 老形态：手工建 approvals 表（无 claim 列），带一行存量审批。
	require.NoError(t, db.Exec(`CREATE TABLE approvals (
		id varchar(255) primary key,
		state varchar(50) NOT NULL,
		function_id varchar(255) NOT NULL,
		game_id varchar(255) NOT NULL,
		env varchar(100) NOT NULL,
		actor varchar(255) NOT NULL,
		approver varchar(255),
		reviewed_at datetime,
		mode varchar(50),
		idempotency_key varchar(255),
		route varchar(500),
		target_service_id varchar(255),
		hash_key varchar(255),
		payload blob,
		metadata_json json,
		reason text,
		result_kind varchar(50),
		task_id varchar(255),
		result blob,
		created_at datetime NOT NULL,
		updated_at datetime NOT NULL,
		deleted_at datetime
	)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO approvals
		(id, state, function_id, game_id, env, actor, created_at, updated_at)
		VALUES ('legacy-ap', 'approved', 'fn.x', 'game', 'prod', 'alice', datetime(), datetime())`).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateApprovalDispatchClaimColumns(context.Background(), sqlDB))

	for _, col := range []string{"claim_status", "claim_token", "claimed_at", "payload_digest"} {
		require.True(t, db.Migrator().HasColumn(&approvals.ApprovalModel{}, col), "missing column %s", col)
	}

	// 既有行保持终态且默认空 claim 字段（仍可领取一次）。
	var row struct {
		State         string
		ClaimStatus   string
		ClaimToken    string
		PayloadDigest string
	}
	require.NoError(t, db.Raw(`SELECT state, claim_status, claim_token, payload_digest FROM approvals WHERE id = 'legacy-ap'`).Scan(&row).Error)
	require.Equal(t, "approved", row.State)
	require.Empty(t, row.ClaimStatus)
	require.Empty(t, row.ClaimToken)
	require.Empty(t, row.PayloadDigest)

	// 幂等：重复执行不报错、不重复加列。
	require.NoError(t, migrateApprovalDispatchClaimColumns(context.Background(), sqlDB))
}

func TestApprovalDispatchClaimMigration_CreatesMissingTable(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "m50-empty.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// 无 approvals 表（multiGame fanout 到无该表的库）：建表并含 claim 列。
	require.NoError(t, migrateApprovalDispatchClaimColumns(context.Background(), sqlDB))
	require.True(t, db.Migrator().HasTable("approvals"))
	require.True(t, db.Migrator().HasColumn(&approvals.ApprovalModel{}, "claim_status"))
}
