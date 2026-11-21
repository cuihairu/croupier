// AUD-007 回归：全局备份接口补齐操作授权。
//
// 覆盖零角色账号（存在但无任何角色/权限）在列表、创建、下载、删除四个操作
// 上均被服务层 RequireAnyPermission 拒绝（403 / permission_denied），且拒绝
// 时无任何副作用（不返回列表条目、不新建备份行、不删除目标备份、不产出下载
// 重定向或文件流）。超管（HasAdminRole 直通）作为对照保持可用。
package backup

import (
	"context"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// backupViewerUsername 是「零角色」账号：管理员存在、登录上下文正常，但未被
// 分配任何角色，因此权限集合为空，RequireAnyPermission 必须拒绝。
const backupViewerUsername = "backup_norole_viewer"

// seedBackupViewer 幂等创建零角色账号并返回用户名。
func seedBackupViewer(t *testing.T, db *gorm.DB) string {
	t.Helper()
	ctx := context.Background()
	adminModel := model.NewAdminModel(db)
	admin, err := adminModel.FindByUsername(ctx, backupViewerUsername)
	if err != nil {
		admin = &model.Admin{Username: backupViewerUsername, Status: 1}
		require.NoError(t, adminModel.Create(ctx, admin, "password123"))
	}
	return backupViewerUsername
}

// viewerCtx 返回携带零角色账号用户名的请求上下文。
func (e *backupFlowEnv) viewerCtx(t *testing.T) context.Context {
	t.Helper()
	return context.WithValue(context.Background(), "username", backupViewerUsername)
}

// assertPermissionDenied 断言为 403 / permission_denied 的稳定拒绝。
func assertPermissionDenied(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err, "低权限请求必须被拒绝")
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusForbidden, codeErr.Code)
	assert.Equal(t, "permission_denied", codeErr.ErrorCode())
}

// backupRowCount 统计备份表行数，用于「无副作用」断言。
func backupRowCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&model.Backup{}).Count(&n).Error)
	return n
}

// TestBackupService_ZeroRolePermissionDenied 验证四个备份操作对零角色账号一律
// 403，且每个操作都无副作用；对照超管账号保持可用。
func TestBackupService_ZeroRolePermissionDenied(t *testing.T) {
	env := setupBackupFlowEnv(t)
	adminCtx := env.authCtx(t)
	seedBackupViewer(t, env.db)
	viewerCtx := env.viewerCtx(t)

	// 对照：超管可正常建备份并读取列表，证明数据面存在、拒绝非「资源缺失」。
	created, err := env.service.Create(adminCtx, &BackupCreateRequest{Name: "perm-target", Type: "full"})
	require.NoError(t, err)
	require.NotEmpty(t, created.Id)
	allowedResp, err := env.service.List(adminCtx, &BackupsListRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, allowedResp.Items)

	// List：403，且不返回任何条目。
	listResp, err := env.service.List(viewerCtx, &BackupsListRequest{})
	assertPermissionDenied(t, err)
	assert.Nil(t, listResp)

	// Create：403，且不新增备份行。
	before := backupRowCount(t, env.db)
	createResp, err := env.service.Create(viewerCtx, &BackupCreateRequest{Name: "should-not-exist"})
	assertPermissionDenied(t, err)
	assert.Nil(t, createResp)
	assert.Equal(t, before, backupRowCount(t, env.db), "拒绝创建不得留下任何备份记录")

	// Delete：403，且目标备份仍然存在。
	err = env.service.Delete(viewerCtx, &BackupDeleteRequest{ID: created.Id})
	assertPermissionDenied(t, err)
	_, err = env.svcCtx.BackupModel.FindByBackupID(adminCtx, created.Id)
	require.NoError(t, err, "拒绝删除后目标备份必须仍然存在")

	// Download：403，且不产出重定向或文件流。
	payload, err := env.service.Download(viewerCtx, &BackupDownloadRequest{ID: created.Id})
	assertPermissionDenied(t, err)
	assert.Nil(t, payload)
}

// TestHandler_BackupPermission_ZeroRoleDenied 走 handler + 路由，验证 403 与
// permission_denied 稳定错误码落到 HTTP 响应，且无副作用。
func TestHandler_BackupPermission_ZeroRoleDenied(t *testing.T) {
	env := setupBackupFlowEnv(t)
	seedBackupViewer(t, env.db)
	createBackupRow(t, env, &model.Backup{BackupID: "bkp-perm", Name: "perm", Type: "full"})

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/backups", ""},
		{http.MethodPost, "/api/v1/backups", `{"name":"nope","type":"full"}`},
		{http.MethodDelete, "/api/v1/backups/bkp-perm", ""},
		{http.MethodGet, "/api/v1/backups/bkp-perm/download", ""},
	} {
		rec := doBackupRequestAs(t, env, tc.method, tc.path, tc.body, backupViewerUsername)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s 应返回 403", tc.method, tc.path)
		assert.Contains(t, rec.Body.String(), "permission_denied", "%s %s 应携带 permission_denied 稳定码", tc.method, tc.path)
	}

	// 拒绝后数据面不变：目标备份仍在，未新建备份。
	row, err := env.svcCtx.BackupModel.FindByBackupID(context.Background(), "bkp-perm")
	require.NoError(t, err)
	assert.Equal(t, "bkp-perm", row.BackupID)
	assert.Equal(t, int64(1), backupRowCount(t, env.db))
}
