// AUD-039 regression: the object-list read surface carries the service-layer
// permission gate.
//
// Root cause: AUD-016 added requireStoragePermission to the storage-management
// write operations (upload/delete/batch-delete/directories) and to signed links,
// but ListObjects was missed -- any logged-in user could enumerate the full object
// inventory of the store (including platform-maintained object names such as
// backups/, releases/), leaking internal object naming and the existence of
// release artifacts. Fixed by giving the list read surface the same coarse-grained
// gate as the write surface (admin:all | uploads:write); prefix-level ABAC is left
// for a later split.
package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStorage_Aud039_ZeroRoleDeniedOnList verifies that a zero-role account is denied
// on list (403 permission_denied); the super-admin control passes through, proving the
// rejection is insufficient permission rather than "resource missing".
func TestStorage_Aud039_ZeroRoleDeniedOnList(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))

	_, err := svc.ListObjects(viewerCtx(), &ListObjectsRequest{})
	assertForbidden(t, err, "permission_denied")

	// Super-admin control: the data plane is readable.
	resp, err := svc.ListObjects(authCtx(), &ListObjectsRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestStorage_Aud039_UploadsWriteGrantsList verifies that an operator holding only
// uploads:write is allowed through on list (read and write share the same coarse-grained
// permission) and can see the objects they uploaded.
func TestStorage_Aud039_UploadsWriteGrantsList(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))

	up, err := svc.UploadObject(operatorCtx(), uploadReq("uploads/listable.txt", "listable"))
	require.NoError(t, err)
	assert.Equal(t, "uploads/listable.txt", up.Path)

	resp, err := svc.ListObjects(operatorCtx(), &ListObjectsRequest{Prefix: "uploads/"})
	require.NoError(t, err)
	assert.NotNil(t, resp)

	found := false
	for _, obj := range resp.Objects {
		if obj.Key == "uploads/listable.txt" {
			found = true
			break
		}
	}
	assert.True(t, found, "uploads:write 操作员列表必须看到已上传对象")
}
