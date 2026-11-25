package storage

import (
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	objstore "github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gapfixStore builds on failingStore and records the keys received by List/Delete,
// used to assert the defense branches' correction of inputs.
type gapfixStore struct {
	failingStore
	listedPrefixes []string
	deletedKeys    []string
}

func (g *gapfixStore) List(_ context.Context, prefix, _, _ string, _ int) (objstore.ListResult, error) {
	g.listedPrefixes = append(g.listedPrefixes, prefix)
	return objstore.ListResult{}, nil
}

func (g *gapfixStore) Delete(_ context.Context, key string) error {
	g.deletedKeys = append(g.deletedKeys, key)
	return nil
}

// withSlashStrippingNormalizer injects a "strip trailing slash" normalization
// implementation, making the slash-restoring defense branch reachable; the seam is
// restored afterwards.
func withSlashStrippingNormalizer(t *testing.T) {
	t.Helper()
	orig := normalizeStoragePath
	normalizeStoragePath = func(raw string) string { return strings.TrimSuffix(raw, "/") }
	t.Cleanup(func() { normalizeStoragePath = orig })
}

// path.Join returns "" for an empty argument slice; the clean == "." fallback is
// reachable only when the seam injects a Join that always returns "."; verify the
// result is zeroed out and no trailing slash is appended.
func TestGapfixService_NormalizeStoragePath_JoinDotSeam(t *testing.T) {
	orig := joinStoragePath
	joinStoragePath = func(parts ...string) string { return "." }
	t.Cleanup(func() { joinStoragePath = orig })

	assert.Equal(t, "", normalizeStoragePath("a/b"))
	assert.Equal(t, "", normalizeStoragePath("a/b/"))
}

// gin's c.FormFile already Opens the same FileHeader, so the handler's second Open
// failing is unreachable in a real request; inject the fault to verify the 500
// internal_error fallback (AUD-047 opaque: the raw text only goes to the log).
func TestGapfixHandler_UploadObject_FileOpenErrorSeam(t *testing.T) {
	orig := openMultipartFileHeader
	openMultipartFileHeader = func(*multipart.FileHeader) (multipart.File, error) {
		return nil, errors.New("open spilled file failed")
	}
	t.Cleanup(func() { openMultipartFileHeader = orig })

	handler := setupHandler(t)
	router := extraRouter(handler)

	req := multipartUpload(t, "/storage/objects", map[string]string{"path": "p.txt"}, "p.txt", "hello")
	rec := doExtraReq(t, router, req)
	assertStatus(t, rec, http.StatusInternalServerError)
	body := jsonBody(t, rec)
	assert.Equal(t, "internal_error", body["error"])
	assert.NotContains(t, rec.Body.String(), "open spilled file failed")
}

// When normalization drops the trailing slash, ListObjects' defense branch should
// restore the "/" of a directory prefix before pushing it down to the store.
func TestGapfixService_ListObjects_PrefixSlashGuard(t *testing.T) {
	withSlashStrippingNormalizer(t)

	store := &gapfixStore{}
	svcObj := NewService(identitySvcCtx(t, store))

	resp, err := svcObj.ListObjects(authCtx(), &ListObjectsRequest{Prefix: "dir/"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, []string{"dir/"}, store.listedPrefixes)
}

// When normalization drops the trailing slash, DeleteObject's defense branch should
// restore the "/", call the store with the slash-suffixed path, and return it to the
// caller.
func TestGapfixService_DeleteObject_SlashGuard(t *testing.T) {
	withSlashStrippingNormalizer(t)

	store := &gapfixStore{}
	svcObj := NewService(identitySvcCtx(t, store))

	resp, err := svcObj.DeleteObject(authCtx(), &DeleteObjectRequest{Path: "dir/"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "dir/", resp.Path)
	assert.Equal(t, []string{"dir/"}, store.deletedKeys)
}

// When normalization drops the trailing slash, BatchDeleteObjects' defense branch
// should restore "/" entry by entry: directory entries are corrected, ordinary file
// entries are kept as-is.
func TestGapfixService_BatchDeleteObjects_SlashGuard(t *testing.T) {
	withSlashStrippingNormalizer(t)

	store := &gapfixStore{}
	svcObj := NewService(identitySvcCtx(t, store))

	resp, err := svcObj.BatchDeleteObjects(authCtx(), &BatchDeleteObjectsRequest{
		Paths: []string{"dir/", "a.txt"},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, []string{"dir/", "a.txt"}, resp.Deleted)
	assert.Empty(t, resp.Failed)
	assert.Equal(t, []string{"dir/", "a.txt"}, store.deletedKeys)
}
