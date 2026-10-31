package svc

// 0051：approvals 领取列（ARCH-002 D7）。回归复刻存量形态：手工建旧列集
// 的 approvals 表 → AddColumn 补齐五列；幂等；缺表跳过；模型列可读写。

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/platform/approvals"
)

func openApprovalClaimDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m51.db"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestApprovalClaimColumnsMigration_LegacyTableShape(t *testing.T) {
	db := openApprovalClaimDB(t)
	// 存量老库形态（过 baseline 的 approvals 表）：只有旧列，无领取列。
	require.NoError(t, db.Exec(`CREATE TABLE approvals (
		id varchar(255) primary key, state varchar(50) not null,
		function_id varchar(255) not null, game_id varchar(255) not null,
		env varchar(100) not null, actor varchar(255) not null,
		approver varchar(255), reviewed_at datetime, mode varchar(50) default 'invoke',
		idempotency_key varchar(255), route varchar(500), target_service_id varchar(255),
		hash_key varchar(255), payload blob, metadata_json json, reason text,
		result_kind varchar(50), task_id varchar(255), result blob,
		created_at datetime not null, updated_at datetime not null, deleted_at datetime)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO approvals
		(id, state, function_id, game_id, env, actor, created_at, updated_at)
		VALUES ('legacy-1', 'pending', 'f.x', 'g', 'prod', 'u', datetime(), datetime())`).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateApprovalClaimColumns(context.Background(), sqlDB))

	for _, col := range []string{"ClaimStatus", "ClaimToken", "ClaimedAt", "DispatchPayloadFingerprint", "DispatchedAt"} {
		require.True(t, db.Migrator().HasColumn(&approvals.ApprovalModel{}, col), "缺列 %s", col)
	}
	// 存量行不回填（pending 尚未领取）。
	var legacy struct {
		Status string
		FP     string
		Token  int64
	}
	require.NoError(t, db.Raw(`SELECT COALESCE(claim_status,'') AS status,
		COALESCE(dispatch_payload_fingerprint,'') AS fp,
		COALESCE(claim_token,0) AS token FROM approvals WHERE id='legacy-1'`).
		Scan(&legacy).Error)
	require.Empty(t, legacy.Status)
	require.Empty(t, legacy.FP)
	require.Zero(t, legacy.Token)

	// 幂等：再跑一次不报错。
	require.NoError(t, migrateApprovalClaimColumns(context.Background(), sqlDB))
}

func TestApprovalClaimColumnsMigration_MissingTableSkips(t *testing.T) {
	db := openApprovalClaimDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateApprovalClaimColumns(context.Background(), sqlDB))
	require.False(t, db.Migrator().HasTable(&approvals.ApprovalModel{}))
}

func TestApprovalClaimColumnsMigration_RoundTripsThroughModel(t *testing.T) {
	db := openApprovalClaimDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE approvals (
		id varchar(255) primary key, state varchar(50) not null,
		function_id varchar(255) not null, game_id varchar(255) not null,
		env varchar(100) not null, actor varchar(255) not null,
		approver varchar(255), reviewed_at datetime, mode varchar(50) default 'invoke',
		idempotency_key varchar(255), route varchar(500), target_service_id varchar(255),
		hash_key varchar(255), payload blob, metadata_json json, reason text,
		result_kind varchar(50), task_id varchar(255), result blob,
		created_at datetime not null, updated_at datetime not null, deleted_at datetime)`).Error)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateApprovalClaimColumns(context.Background(), sqlDB))

	rec := &approvals.Approval{ID: "rt-1", State: "pending", FunctionID: "f.x",
		GameID: "g", Env: "dev", Actor: "u",
		ClaimStatus: "claimed", ClaimToken: 7, DispatchPayloadFingerprint: "fp"}
	store, err := approvals.NewSQLStore(db)
	require.NoError(t, err)
	_, err = store.Create(rec)
	require.NoError(t, err)
	got, err := store.Get("rt-1")
	require.NoError(t, err)
	require.Equal(t, "claimed", got.ClaimStatus)
	require.Equal(t, int64(7), got.ClaimToken)
	require.Equal(t, "fp", got.DispatchPayloadFingerprint)
}
