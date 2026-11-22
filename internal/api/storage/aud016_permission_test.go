// AUD-016 回归：通用对象接口按现有权限定义控制写操作与签名访问，并保护
// 平台保留前缀（backups/、releases/ 等）。
//
// 根因：storage/service.go 直接用输入 path 写全局存储，仅凭认证（Authority
// 中间件）即可覆盖 backups/ 对象。修复为服务层权限门禁 + 保留前缀写边界。
package storage

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// uploadReq 构造一个以内存内容上传的请求（File 为 nil 走 Content 分支）。
func uploadReq(path, content string) *UploadObjectRequest {
	return &UploadObjectRequest{Path: path, Content: content}
}

// assertForbidden 断言错误为 403 且携带期望稳定错误码。
func assertForbidden(t *testing.T, err error, code string) {
	t.Helper()
	require.Error(t, err, "必须被拒绝")
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr, "必须为 CodeError")
	assert.Equal(t, http.StatusForbidden, codeErr.Code, "必须 403")
	assert.Equal(t, code, codeErr.ErrorCode(), "稳定错误码")
}

// assertNotExists 断言对象不存在（无副作用）。
func assertNotExists(t *testing.T, s *Service, key string) {
	t.Helper()
	_, err := s.svcCtx.ObjectStore.Get(context.Background(), key)
	assert.ErrorIs(t, err, os.ErrNotExist, "对象 %s 不得存在", key)
}

// assertExistsContent 断言对象存在且内容一致（保留前缀对象未被改动）。
func assertExistsContent(t *testing.T, s *Service, key, want string) {
	t.Helper()
	rc, err := s.svcCtx.ObjectStore.Get(context.Background(), key)
	require.NoError(t, err, "对象 %s 必须存在", key)
	defer rc.Close()
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, want, string(got), "对象 %s 内容不变", key)
}

// TestStorage_Aud016_ZeroRoleDeniedOnMutations 验证零角色账号在六个操作面上
// 一律 403 permission_denied 且无副作用；超管对照可用。
func TestStorage_Aud016_ZeroRoleDeniedOnMutations(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	ctx := viewerCtx()

	_, err := svc.UploadObject(ctx, uploadReq("uploads/leak.txt", "x"))
	assertForbidden(t, err, "permission_denied")
	assertNotExists(t, svc, "uploads/leak.txt")

	_, err = svc.DeleteObject(ctx, &DeleteObjectRequest{Path: "uploads/leak.txt"})
	assertForbidden(t, err, "permission_denied")

	_, err = svc.BatchDeleteObjects(ctx, &BatchDeleteObjectsRequest{Paths: []string{"uploads/leak.txt"}})
	assertForbidden(t, err, "permission_denied")

	_, err = svc.CreateDirectory(ctx, &CreateDirectoryRequest{Prefix: "uploads"})
	assertForbidden(t, err, "permission_denied")

	_, err = svc.RenameDirectory(ctx, &RenameDirectoryRequest{OldPrefix: "uploads", NewPrefix: "media"})
	assertForbidden(t, err, "permission_denied")

	_, err = svc.SignedUrl(ctx, &SignedUrlRequest{Path: "uploads/leak.txt"})
	assertForbidden(t, err, "permission_denied")

	// 超管对照：数据面存在、拒绝非「资源缺失」。
	up, err := svc.UploadObject(authCtx(), uploadReq("uploads/ok.txt", "body"))
	require.NoError(t, err)
	assert.Equal(t, "uploads/ok.txt", up.Path)
	assertExistsContent(t, svc, "uploads/ok.txt", "body")
}

// TestStorage_Aud016_UploadsWriteGrantsNormalOps 验证仅持 uploads:write 的
// 操作员可执行常规存储操作（数据面正常），但不能触碰保留前缀（写边界），
// 也不能为备份对象签发签名链接（镜像 backup:read）。
func TestStorage_Aud016_UploadsWriteGrantsNormalOps(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	ctx := operatorCtx()

	up, err := svc.UploadObject(ctx, uploadReq("uploads/notes.txt", "note-body"))
	require.NoError(t, err)
	assert.Equal(t, "uploads/notes.txt", up.Path)
	assertExistsContent(t, svc, "uploads/notes.txt", "note-body")

	// 覆盖自身对象允许（同域写）。
	_, err = svc.UploadObject(ctx, uploadReq("uploads/notes.txt", "note-v2"))
	require.NoError(t, err)
	assertExistsContent(t, svc, "uploads/notes.txt", "note-v2")

	// 常规路径签名链接放行。
	sign, err := svc.SignedUrl(ctx, &SignedUrlRequest{Path: "uploads/notes.txt"})
	require.NoError(t, err)
	assert.NotEmpty(t, sign.URL)

	// 保留前缀写被拒：审计实证的「覆盖 backups/ 对象」路径。
	_, err = svc.UploadObject(ctx, uploadReq("backups/scope/db.sql", "pwn"))
	assertForbidden(t, err, "retention_prefix_forbidden")
	assertNotExists(t, svc, "backups/scope/db.sql")

	// 备份对象签名访问被拒（无 backup:read，不得绕过备份下载授权）。
	_, err = svc.SignedUrl(ctx, &SignedUrlRequest{Path: "backups/scope/db.sql"})
	assertForbidden(t, err, "permission_denied")
}

// TestStorage_Aud016_SuperAdminCannotTouchProtectedPrefixes 验证保留前缀写
// 边界对超管同样成立：平台维护对象不经通用接口写/删/改名。
func TestStorage_Aud016_SuperAdminCannotTouchProtectedPrefixes(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	ctx := authCtx()

	for _, tc := range []struct {
		name string
		path string
	}{
		{"backups 上传", "backups/scope/db.sql"},
		{"releases 上传", "releases/demo/1.0.0/app.bin"},
		{"backups 精确目录", "backups"},
	} {
		_, err := svc.UploadObject(ctx, uploadReq(tc.path, "x"))
		assertForbidden(t, err, "retention_prefix_forbidden")
		assertNotExists(t, svc, tc.path)
	}

	_, err := svc.DeleteObject(ctx, &DeleteObjectRequest{Path: "backups/scope/db.sql"})
	assertForbidden(t, err, "retention_prefix_forbidden")

	_, err = svc.CreateDirectory(ctx, &CreateDirectoryRequest{Prefix: "backups/scope"})
	assertForbidden(t, err, "retention_prefix_forbidden")

	// 双向：移入或移出保留前缀都拒绝。
	_, err = svc.RenameDirectory(ctx, &RenameDirectoryRequest{OldPrefix: "media", NewPrefix: "backups/scope"})
	assertForbidden(t, err, "retention_prefix_forbidden")
	_, err = svc.RenameDirectory(ctx, &RenameDirectoryRequest{OldPrefix: "backups/scope", NewPrefix: "media"})
	assertForbidden(t, err, "retention_prefix_forbidden")

	// 保留前缀签名访问：超管放行（读能力，非写）。
	sign, err := svc.SignedUrl(ctx, &SignedUrlRequest{Path: "backups/scope/db.sql"})
	require.NoError(t, err)
	assert.NotEmpty(t, sign.URL)
	sign, err = svc.SignedUrl(ctx, &SignedUrlRequest{Path: "releases/demo/1.0.0/app.bin"})
	require.NoError(t, err)
	assert.NotEmpty(t, sign.URL)
}

// TestStorage_Aud016_BatchWholeRejected 验证批量删除含保留前缀条目时整单
// 拒绝、无任何删除（无部分执行状态）。
func TestStorage_Aud016_BatchWholeRejected(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	ctx := authCtx()

	_, err := svc.UploadObject(ctx, uploadReq("uploads/a.txt", "a"))
	require.NoError(t, err)
	_, err = svc.UploadObject(ctx, uploadReq("uploads/b.txt", "b"))
	require.NoError(t, err)
	assertExistsContent(t, svc, "uploads/a.txt", "a")

	_, err = svc.BatchDeleteObjects(ctx, &BatchDeleteObjectsRequest{
		Paths: []string{"uploads/a.txt", "backups/scope/db.sql", "uploads/b.txt"},
	})
	assertForbidden(t, err, "retention_prefix_forbidden")

	// 无副作用：正常条目也不得被删除（整单拒绝）。
	assertExistsContent(t, svc, "uploads/a.txt", "a")
	assertExistsContent(t, svc, "uploads/b.txt", "b")
}

// TestStorage_Aud016_ProtectedPrefixObjectUntouched 验证预存的备份对象（由
// 子系统直写，模拟 backupexec）不被覆盖/删除/改名，内容保持一致。
func TestStorage_Aud016_ProtectedPrefixObjectUntouched(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	ctx := authCtx()
	const key = "backups/demo/prod/db.dump"

	// 子系统侧直写（不经通用接口），模拟备份执行器产物。
	require.NoError(t, svc.svcCtx.ObjectStore.Put(context.Background(), key,
		strings.NewReader("original-backup"), int64(len("original-backup")), "application/octet-stream"))

	_, err := svc.UploadObject(ctx, uploadReq(key, "overwrite-attempt"))
	assertForbidden(t, err, "retention_prefix_forbidden")
	assertExistsContent(t, svc, key, "original-backup")

	_, err = svc.DeleteObject(ctx, &DeleteObjectRequest{Path: key})
	assertForbidden(t, err, "retention_prefix_forbidden")
	assertExistsContent(t, svc, key, "original-backup")

	_, err = svc.BatchDeleteObjects(ctx, &BatchDeleteObjectsRequest{Paths: []string{key}})
	assertForbidden(t, err, "retention_prefix_forbidden")
	assertExistsContent(t, svc, key, "original-backup")
}
