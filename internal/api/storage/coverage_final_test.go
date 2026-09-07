package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFinalHandler_UploadObject_FileHeaderOpenError covers the branch where
// c.FormFile succeeds but FileHeader.Open fails. The multipart part is forced
// to spill to a temp file (ParseMultipartForm with maxMemory=1), then the temp
// file is removed before the handler runs so Open() fails with ENOENT.
func TestFinalHandler_UploadObject_FileHeaderOpenError(t *testing.T) {
	snap := tempMultipartFiles(t)

	req := multipartUpload(t, "/storage/objects", map[string]string{}, "spawn.txt", "payload-large-enough-to-spill")
	require.NoError(t, req.ParseMultipartForm(1))

	for f := range tempMultipartFiles(t) {
		if !snap[f] {
			require.NoError(t, os.Remove(f))
		}
	}

	router := extraRouter(NewHandler(NewService(setupSvcCtxWithStore(t))))
	rec := doExtraReq(t, router, req)
	assertStatus(t, rec, 400)
	assertErrorShape(t, rec)
}

// TestFinalHandler_CreateDirectory_BindError covers the invalid JSON body branch.
func TestFinalHandler_CreateDirectory_BindError(t *testing.T) {
	router := extraRouter(setupHandler(t))
	rec := doReq(t, router, "POST", "/storage/directories", `{nope`)
	assertStatus(t, rec, 400)
	assertErrorShape(t, rec)
}

// TestFinalService_NormalizeStoragePath_RootCollapsesToEmpty covers the
// path.Join(".") reset branch when every segment is filtered out.
func TestFinalService_NormalizeStoragePath_RootCollapsesToEmpty(t *testing.T) {
	assert.Equal(t, "", normalizeStoragePath("/"))
	assert.Equal(t, "", normalizeStoragePath("///"))
	assert.Equal(t, "", normalizeStoragePath("../.."))
	assert.Equal(t, "", normalizeStoragePath(`\`))
}

// tempMultipartFiles snapshots existing multipart temp files under the system
// temp dir so tests can identify files created by ParseMultipartForm.
func tempMultipartFiles(t *testing.T) map[string]bool {
	t.Helper()
	res := make(map[string]bool)
	_matches, err := filepath.Glob(filepath.Join(os.TempDir(), "multipart-*"))
	require.NoError(t, err)
	for _, m := range _matches {
		res[m] = true
	}
	return res
}
