// AUD-016 regression: the generic object interface gates write operations and
// signed access by the existing permission definitions, and protects the
// platform-reserved prefixes (backups/, releases/, etc.).
//
// Root cause: storage/service.go wrote to the global store using the input path
// directly, so authentication alone (the Authority middleware) was enough to
// overwrite backups/ objects. Fixed with a service-level permission gate plus a
// reserved-prefix write boundary.
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

// uploadReq builds an upload request with in-memory content (File nil takes the
// Content branch).
func uploadReq(path, content string) *UploadObjectRequest {
	return &UploadObjectRequest{Path: path, Content: content}
}

// assertForbidden asserts the error is 403 and carries the expected stable error code.
func assertForbidden(t *testing.T, err error, code string) {
	t.Helper()
	require.Error(t, err, "必须被拒绝")
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr, "必须为 CodeError")
	assert.Equal(t, http.StatusForbidden, codeErr.Code, "必须 403")
	assert.Equal(t, code, codeErr.ErrorCode(), "稳定错误码")
}

// assertNotExists asserts the object does not exist (no side effects).
func assertNotExists(t *testing.T, s *Service, key string) {
	t.Helper()
	_, err := s.svcCtx.ObjectStore.Get(context.Background(), key)
	assert.ErrorIs(t, err, os.ErrNotExist, "对象 %s 不得存在", key)
}

// assertExistsContent asserts the object exists with identical content (a
// protected-prefix object was left untouched).
func assertExistsContent(t *testing.T, s *Service, key, want string) {
	t.Helper()
	rc, err := s.svcCtx.ObjectStore.Get(context.Background(), key)
	require.NoError(t, err, "对象 %s 必须存在", key)
	defer rc.Close()
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, want, string(got), "对象 %s 内容不变", key)
}

// TestStorage_Aud016_ZeroRoleDeniedOnMutations verifies that a zero-role account
// gets 403 permission_denied with no side effects across all six operation
// surfaces; the super admin control still works.
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

	// Super-admin control: the data plane exists and rejects non-"resource missing".
	up, err := svc.UploadObject(authCtx(), uploadReq("uploads/ok.txt", "body"))
	require.NoError(t, err)
	assert.Equal(t, "uploads/ok.txt", up.Path)
	assertExistsContent(t, svc, "uploads/ok.txt", "body")
}

// TestStorage_Aud016_UploadsWriteGrantsNormalOps verifies that an operator holding
// only uploads:write can perform normal storage operations (the data plane works),
// but cannot touch reserved prefixes (write boundary), nor sign links for backup
// objects (mirrors backup:read).
func TestStorage_Aud016_UploadsWriteGrantsNormalOps(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	ctx := operatorCtx()

	up, err := svc.UploadObject(ctx, uploadReq("uploads/notes.txt", "note-body"))
	require.NoError(t, err)
	assert.Equal(t, "uploads/notes.txt", up.Path)
	assertExistsContent(t, svc, "uploads/notes.txt", "note-body")

	// Overwriting own objects allowed (same-domain write).
	_, err = svc.UploadObject(ctx, uploadReq("uploads/notes.txt", "note-v2"))
	require.NoError(t, err)
	assertExistsContent(t, svc, "uploads/notes.txt", "note-v2")

	// Signed links on regular paths are allowed through.
	sign, err := svc.SignedUrl(ctx, &SignedUrlRequest{Path: "uploads/notes.txt"})
	require.NoError(t, err)
	assert.NotEmpty(t, sign.URL)

	// Reserved-prefix writes rejected: the "overwrite a backups/ object" path proven by the audit.
	_, err = svc.UploadObject(ctx, uploadReq("backups/scope/db.sql", "pwn"))
	assertForbidden(t, err, "retention_prefix_forbidden")
	assertNotExists(t, svc, "backups/scope/db.sql")

	// Signed access to a backup object is denied (no backup:read; backup download authorization must not be bypassed).
	_, err = svc.SignedUrl(ctx, &SignedUrlRequest{Path: "backups/scope/db.sql"})
	assertForbidden(t, err, "permission_denied")
}

// TestStorage_Aud016_SuperAdminCannotTouchProtectedPrefixes verifies that the
// reserved-prefix write boundary holds for the super admin too: platform-maintained
// objects are not written/deleted/renamed through the generic interface.
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

	// Both directions: moving into or out of a reserved prefix is denied.
	_, err = svc.RenameDirectory(ctx, &RenameDirectoryRequest{OldPrefix: "media", NewPrefix: "backups/scope"})
	assertForbidden(t, err, "retention_prefix_forbidden")
	_, err = svc.RenameDirectory(ctx, &RenameDirectoryRequest{OldPrefix: "backups/scope", NewPrefix: "media"})
	assertForbidden(t, err, "retention_prefix_forbidden")

	// Signed access to reserved prefixes: the super admin is allowed through (read capability, not write).
	sign, err := svc.SignedUrl(ctx, &SignedUrlRequest{Path: "backups/scope/db.sql"})
	require.NoError(t, err)
	assert.NotEmpty(t, sign.URL)
	sign, err = svc.SignedUrl(ctx, &SignedUrlRequest{Path: "releases/demo/1.0.0/app.bin"})
	require.NoError(t, err)
	assert.NotEmpty(t, sign.URL)
}

// TestStorage_Aud016_BatchWholeRejected verifies that a batch delete containing a
// reserved-prefix entry is rejected as a whole, with nothing deleted (no partial
// execution state).
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

	// No side effects: even the normal entries must not be deleted (whole-order rejection).
	assertExistsContent(t, svc, "uploads/a.txt", "a")
	assertExistsContent(t, svc, "uploads/b.txt", "b")
}

// TestStorage_Aud016_ProtectedPrefixObjectUntouched verifies that a pre-existing
// backup object (written directly by a subsystem, simulating backupexec) is never
// overwritten/deleted/renamed and keeps its content.
func TestStorage_Aud016_ProtectedPrefixObjectUntouched(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	ctx := authCtx()
	const key = "backups/demo/prod/db.dump"

	// Subsystem-side direct write (not through the generic interface), simulating a backup runner artifact.
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
