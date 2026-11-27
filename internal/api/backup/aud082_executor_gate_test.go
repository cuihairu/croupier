// AUD-082 regression: the generic backup create entry used to write a pending
// record BEFORE inspecting the executor, so with no object store configured (or
// an unsupported database driver) the caller got a pending status that nothing
// ever advanced. Covers:
//   - create with an executor available -> the normal async execution path is
//     unchanged (pending acceptance, then a real succeeded run);
//   - create with missing storage / unsupported driver -> rejected up-front with
//     400 + backup_executor_unavailable and a machine-readable reason, and no
//     forever-pending row is written;
//   - legacy pending rows created by the old bug converge on read (database rows
//     and the extension-mirror snapshot) and on same-key retry (zombie dedupe
//     reuse), instead of staying pending forever.
package backup

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aud082RowCount counts the backups rows, for the "no side effects" assertions.
func aud082RowCount(t *testing.T, env *backupFlowEnv) int64 {
	t.Helper()
	var n int64
	require.NoError(t, env.db.Model(&model.Backup{}).Count(&n).Error)
	return n
}

// aud082Backdate sets a backup row's created_at, simulating a legacy row written
// before this process existed.
func aud082Backdate(t *testing.T, env *backupFlowEnv, backupID string, at time.Time) {
	t.Helper()
	require.NoError(t, env.db.Model(&model.Backup{}).
		Where("backup_id = ?", backupID).
		Update("created_at", at).Error)
}

// (a) create with executor available: the acceptance path is unchanged - the row
// starts pending and the async run advances it to succeeded.
func TestAud082_CreateWithExecutorAvailableExecutes(t *testing.T) {
	env := newExecTestEnv(t, "sqlite", execSQLiteSource(t), &execObjStoreStub{})

	resp, err := env.service.Create(env.authCtx(t), &BackupCreateRequest{Name: "aud082-normal"})
	require.NoError(t, err, "执行器可用时创建必须照常接受")
	require.NotNil(t, resp)
	assert.Equal(t, "pending", resp.Status, "接受响应立即返回 pending")
	assert.Empty(t, resp.ErrorMessage)

	waitBackupStatus(t, env, resp.Id, "succeeded")
}

// execSQLiteSource builds a real sqlite database file for the executor to dump.
func execSQLiteSource(t *testing.T) string {
	t.Helper()
	src := filepath.Join(t.TempDir(), "aud082-src.db")
	writeRealSQLiteDB(t, src)
	return src
}

// (b1) create with the object store missing: rejected up-front with the stable
// code and reason, and no pending row is written.
func TestAud082_CreateWithoutObjectStoreRejected(t *testing.T) {
	env := newExecTestEnv(t, "sqlite", "/dev/null", nil)

	_, err := env.service.Create(env.authCtx(t), &BackupCreateRequest{Name: "aud082-nostore"})
	require.Error(t, err, "缺对象存储时创建必须被拒绝")
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
	assert.Equal(t, "backup_executor_unavailable", codeErr.ErrorCode(), "必须暴露稳定 snake_case 错误码")
	require.NotNil(t, codeErr.Details)
	assert.Equal(t, "object_store_missing", codeErr.Details["reason"])
	assert.Contains(t, err.Error(), "对象存储")

	assert.EqualValues(t, 0, aud082RowCount(t, env), "拒绝不得留下任何备份记录")
}

// (b2) create with an unsupported database driver: rejected up-front with the
// driver reason, and no pending row is written.
func TestAud082_CreateWithUnsupportedDriverRejected(t *testing.T) {
	env := newExecTestEnv(t, "oracle", "dsn", &execObjStoreStub{})

	_, err := env.service.Create(env.authCtx(t), &BackupCreateRequest{Name: "aud082-oracle"})
	require.Error(t, err, "驱动不支持时创建必须被拒绝")
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
	assert.Equal(t, "backup_executor_unavailable", codeErr.ErrorCode())
	require.NotNil(t, codeErr.Details)
	assert.Equal(t, "driver_unsupported", codeErr.Details["reason"])

	assert.EqualValues(t, 0, aud082RowCount(t, env), "拒绝不得留下任何备份记录")
}

// (b3) the same rejection through the handler + router: the wire contract carries
// 400 with the stable code and the machine-readable reason.
func TestAud082_HandlerCreateRejectsWithStableCode(t *testing.T) {
	env := setupBackupFlowEnv(t)
	env.svcCtx.ObjectStore = nil // storage not configured

	rec := doBackupRequest(t, env, http.MethodPost, "/api/v1/backups", `{"name":"aud082-handler","type":"full"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "backup_executor_unavailable")
	assert.Contains(t, rec.Body.String(), "object_store_missing")
	assert.EqualValues(t, 0, aud082RowCount(t, env), "拒绝不得留下任何备份记录")
}

// (c1) legacy pending rows converge on list: a pending row older than the maximum
// run budget can no longer be in flight and is flipped to failed with a reason,
// while a fresh pending row (possibly in flight) is left untouched.
func TestAud082_LegacyPendingConvergedOnList(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	legacy := createBackupRow(t, env, &model.Backup{BackupID: "aud082-legacy", Name: "legacy", Type: "full", Status: "pending"})
	aud082Backdate(t, env, legacy.BackupID, time.Now().Add(-2*time.Hour))
	fresh := createBackupRow(t, env, &model.Backup{BackupID: "aud082-fresh", Name: "fresh", Type: "full", Status: "pending"})

	resp, err := env.service.List(ctx, &BackupsListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)

	byID := map[string]Backup{}
	for _, item := range resp.Items {
		byID[item.Id] = item
	}
	assert.Equal(t, "failed", byID[legacy.BackupID].Status, "悬挂 pending 必须在读侧收敛为 failed")
	assert.NotEmpty(t, byID[legacy.BackupID].ErrorMessage, "收敛必须带原因")
	assert.Equal(t, "pending", byID[fresh.BackupID].Status, "窗内 pending 可能仍在执行，不得误伤")

	// The convergence is persisted, not just a read-side view.
	converged, err := env.svcCtx.BackupModel.FindByBackupID(ctx, legacy.BackupID)
	require.NoError(t, err)
	assert.Equal(t, "failed", converged.Status)
	assert.Contains(t, converged.ErrorMessage, "重新创建")
	inFlight, err := env.svcCtx.BackupModel.FindByBackupID(ctx, fresh.BackupID)
	require.NoError(t, err)
	assert.Equal(t, "pending", inFlight.Status)
}

// (c2) legacy mirror-only pending entries: AUD-083 migrated the mirror into the
// database on list (one-way import, then the snapshot is drained), and the AUD-082
// database convergence runs right after the import, so an old forever-pending
// mirror entry surfaces as failed in the very same read while a fresh entry stays
// pending. Persistence lives in the database rows now — the mirror convergence
// half (AUD-083 seam) was retired.
func TestAud082_LegacyPendingConvergedInExtensionMirror(t *testing.T) {
	env := setupBackupFlowEnv(t)
	ctx := env.authCtx(t)

	installBackupAdvancedExtension(t, env, map[string]any{"backups": []Backup{
		{
			Id:        "aud082-ext-legacy",
			Name:      "legacy",
			Type:      "full",
			Status:    "pending",
			CreatedAt: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
		},
		{
			Id:        "aud082-ext-fresh",
			Name:      "fresh",
			Type:      "full",
			Status:    "pending",
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}})

	resp, err := env.service.List(ctx, &BackupsListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)

	byID := map[string]Backup{}
	for _, item := range resp.Items {
		byID[item.Id] = item
	}
	assert.Equal(t, "failed", byID["aud082-ext-legacy"].Status, "镜像悬挂 pending 必须收敛")
	assert.NotEmpty(t, byID["aud082-ext-legacy"].ErrorMessage)
	assert.Equal(t, "pending", byID["aud082-ext-fresh"].Status)

	// The convergence is persisted on the imported database rows, not just in the
	// response (the mirror snapshot itself was drained by the migration).
	converged, err := env.svcCtx.BackupModel.FindByBackupID(ctx, "aud082-ext-legacy")
	require.NoError(t, err)
	assert.Equal(t, "failed", converged.Status)
	assert.Contains(t, converged.ErrorMessage, "重新创建")
	inFlight, err := env.svcCtx.BackupModel.FindByBackupID(ctx, "aud082-ext-fresh")
	require.NoError(t, err)
	assert.Equal(t, "pending", inFlight.Status)

	items, ok, err := env.service.loadBackupsFromExtensionInstallation(ctx)
	require.NoError(t, err)
	require.False(t, ok, "迁移完成后镜像快照必须被清空")
	require.Empty(t, items)
}

// (c3) a zombie pending hit on the idempotency dedupe path converges and the retry
// creates a fresh, actually executable backup instead of re-issuing a dead pending.
func TestAud082_ZombiePendingWithIdempotencyKeyConvergedOnRetry(t *testing.T) {
	env := newExecTestEnv(t, "sqlite", execSQLiteSource(t), &execObjStoreStub{})

	key := "aud082-zombie-key"
	legacy := &model.Backup{
		BackupID:       "aud082-zombie",
		Name:           "zombie",
		Type:           "full",
		Status:         "pending",
		IdempotencyKey: &key,
	}
	require.NoError(t, env.svcCtx.BackupModel.Create(context.Background(), legacy))
	aud082Backdate(t, env, legacy.BackupID, time.Now().Add(-2*time.Hour))

	resp, err := env.service.Create(env.authCtx(t), &BackupCreateRequest{Name: "aud082-zombie-retry", Type: "full", IdempotencyKey: key})
	require.NoError(t, err, "同键重试必须照常成功（不得复用悬挂 pending）")
	assert.NotEqual(t, legacy.BackupID, resp.Id, "悬挂 pending 不得作为新创建结果复用")

	converged, err := env.svcCtx.BackupModel.FindByBackupID(context.Background(), legacy.BackupID)
	require.NoError(t, err)
	assert.Equal(t, "failed", converged.Status, "同键悬挂 pending 必须被收敛")
	assert.NotEmpty(t, converged.ErrorMessage)

	// The released key now belongs to the fresh row only.
	var n int64
	require.NoError(t, env.db.Model(&model.Backup{}).Where("idempotency_key = ?", key).Count(&n).Error)
	assert.EqualValues(t, 1, n)

	waitBackupStatus(t, env, resp.Id, "succeeded")
}
