package storage

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	objstore "github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFileStore returns a file-backed object store rooted at a fresh temp dir.
func newFileStore(t *testing.T) objstore.Store {
	t.Helper()
	store, err := objstore.OpenFile(context.Background(), objstore.Config{
		Driver:  "file",
		BaseDir: t.TempDir(),
	})
	require.NoError(t, err)
	return store
}

func setupSvcCtxWithStore(t *testing.T) *svc.ServiceContext {
	t.Helper()
	return &svc.ServiceContext{ObjectStore: newFileStore(t)}
}

func setupSvcCtxNoStore(t *testing.T) *svc.ServiceContext {
	t.Helper()
	return &svc.ServiceContext{ObjectStore: nil}
}

func newTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, rec
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("expected status %d, got %d body=%s", want, rec.Code, rec.Body.String())
	}
}

func assertErrorShape(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	body := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "body not JSON object: %s", rec.Body.String())
	errCode, _ := body["error"].(string)
	require.NotEmpty(t, errCode, "missing 'error' field, body=%s", rec.Body.String())
	msg, _ := body["message"].(string)
	require.NotEmpty(t, msg, "missing 'message' field, body=%s", rec.Body.String())
}

func TestService_ListObjects_Empty(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))

	resp, err := svc.ListObjects(context.Background(), &ListObjectsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Objects)
}

func TestService_NotConfigured_ReturnsBadRequest(t *testing.T) {
	svc := NewService(setupSvcCtxNoStore(t))

	// Every method calls requireStore() first and must reject gracefully.
	_, err := svc.ListObjects(context.Background(), &ListObjectsRequest{})
	require.Error(t, err)

	_, err = svc.SignedUrl(context.Background(), &SignedUrlRequest{Path: "x"})
	require.Error(t, err)
}

func TestService_UploadThenList_RoundTrip(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))

	uploadResp, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
		Path:        "folder/file.txt",
		File:        strings.NewReader("hello world"),
		Size:        11,
		ContentType: "text/plain",
	})
	require.NoError(t, err)
	require.NotNil(t, uploadResp)
	assert.Equal(t, "folder/file.txt", uploadResp.Path)

	listResp, err := svc.ListObjects(context.Background(), &ListObjectsRequest{Prefix: "folder/"})
	require.NoError(t, err)
	require.NotEmpty(t, listResp.Objects)
	assert.Equal(t, "folder/file.txt", listResp.Objects[0].Key)
}

func TestService_UploadObject_MissingFileAndPath(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))

	resp, err := svc.UploadObject(context.Background(), &UploadObjectRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_DeleteObject_MissingPath(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))

	resp, err := svc.DeleteObject(context.Background(), &DeleteObjectRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_BatchDeleteObjects_EmptyPaths(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))

	resp, err := svc.BatchDeleteObjects(context.Background(), &BatchDeleteObjectsRequest{Paths: []string{}})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_BatchDeleteObjects_OK(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))

	// Seed two objects.
	for _, p := range []string{"a.txt", "b.txt"} {
		_, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
			Path: p, File: strings.NewReader("x"), Size: 1,
		})
		require.NoError(t, err)
	}

	resp, err := svc.BatchDeleteObjects(context.Background(), &BatchDeleteObjectsRequest{
		Paths: []string{"a.txt", "b.txt"},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.ElementsMatch(t, []string{"a.txt", "b.txt"}, resp.Deleted)
	assert.Empty(t, resp.Failed)
}

func TestService_CreateDirectory_OK(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))

	resp, err := svc.CreateDirectory(context.Background(), &CreateDirectoryRequest{Prefix: "album"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "album/", resp.Prefix)
}

func TestService_CreateDirectory_MissingPrefix(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))

	resp, err := svc.CreateDirectory(context.Background(), &CreateDirectoryRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_SignedUrl_MissingPath(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))

	resp, err := svc.SignedUrl(context.Background(), &SignedUrlRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_SignedUrl_OK(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))

	resp, err := svc.SignedUrl(context.Background(), &SignedUrlRequest{Path: "doc.txt", Expire: 60})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.URL)
}
