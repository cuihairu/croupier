// AUD-007 regression: the global backup endpoints gained operation authorization.
//
// Covers a zero-role account (exists but has no roles/permissions) being rejected by
// the service-layer RequireAnyPermission (403 / permission_denied) across the four
// operations list, create, download, delete, with no side effects on rejection (no
// list entries returned, no backup row created, target backup not deleted, no download
// redirect or file stream produced). The super-admin (HasAdminRole pass-through)
// remains usable as the control.
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

// backupViewerUsername is the "zero-role" account: the admin exists and the login
// context is normal, but no role is assigned, so the permission set is empty and
// RequireAnyPermission must reject.
const backupViewerUsername = "backup_norole_viewer"

// seedBackupViewer idempotently creates the zero-role account and returns the username.
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

// viewerCtx returns a request context carrying the zero-role account's username.
func (e *backupFlowEnv) viewerCtx(t *testing.T) context.Context {
	t.Helper()
	return context.WithValue(context.Background(), "username", backupViewerUsername)
}

// assertPermissionDenied asserts a stable 403 / permission_denied rejection.
func assertPermissionDenied(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err, "低权限请求必须被拒绝")
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusForbidden, codeErr.Code)
	assert.Equal(t, "permission_denied", codeErr.ErrorCode())
}

// backupRowCount counts the rows in the backup table, for the "no side effects" assertions.
func backupRowCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&model.Backup{}).Count(&n).Error)
	return n
}

// TestBackupService_ZeroRolePermissionDenied verifies all four backup operations return
// 403 for a zero-role account with no side effects per operation; the super-admin
// control account remains usable.
func TestBackupService_ZeroRolePermissionDenied(t *testing.T) {
	env := setupBackupFlowEnv(t)
	adminCtx := env.authCtx(t)
	seedBackupViewer(t, env.db)
	viewerCtx := env.viewerCtx(t)

	// Control: the super-admin can create a backup and read the list normally, proving
	// the data plane exists and the rejection is not "resource missing".
	created, err := env.service.Create(adminCtx, &BackupCreateRequest{Name: "perm-target", Type: "full"})
	require.NoError(t, err)
	require.NotEmpty(t, created.Id)
	allowedResp, err := env.service.List(adminCtx, &BackupsListRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, allowedResp.Items)
	// The control create runs an async backup writing into the temp dir; await
	// its terminal state so TempDir cleanup does not race the executor.
	waitBackupStatus(t, env, created.Id, "succeeded")

	// List: 403, and no entries are returned.
	listResp, err := env.service.List(viewerCtx, &BackupsListRequest{})
	assertPermissionDenied(t, err)
	assert.Nil(t, listResp)

	// Create: 403, and no backup row is added.
	before := backupRowCount(t, env.db)
	createResp, err := env.service.Create(viewerCtx, &BackupCreateRequest{Name: "should-not-exist"})
	assertPermissionDenied(t, err)
	assert.Nil(t, createResp)
	assert.Equal(t, before, backupRowCount(t, env.db), "拒绝创建不得留下任何备份记录")

	// Delete: 403, and the target backup still exists.
	err = env.service.Delete(viewerCtx, &BackupDeleteRequest{ID: created.Id})
	assertPermissionDenied(t, err)
	_, err = env.svcCtx.BackupModel.FindByBackupID(adminCtx, created.Id)
	require.NoError(t, err, "拒绝删除后目标备份必须仍然存在")

	// Download: 403, and no redirect or file stream is produced.
	payload, err := env.service.Download(viewerCtx, &BackupDownloadRequest{ID: created.Id})
	assertPermissionDenied(t, err)
	assert.Nil(t, payload)
}

// TestHandler_BackupPermission_ZeroRoleDenied goes through the handler + router and
// verifies the 403 and the permission_denied stable error code land in the HTTP
// response with no side effects.
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

	// The data plane is unchanged after rejection: the target backup still exists and
	// no backup was created.
	row, err := env.svcCtx.BackupModel.FindByBackupID(context.Background(), "bkp-perm")
	require.NoError(t, err)
	assert.Equal(t, "bkp-perm", row.BackupID)
	assert.Equal(t, int64(1), backupRowCount(t, env.db))
}
