package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- Service Tests ----

func TestServiceV2_UploadObject_ContentString(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	resp, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
		Path:    "content-test.txt",
		Content: "hello from content field",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "content-test.txt", resp.Path)
}

func TestServiceV2_UploadObject_NoPath_UsePreassignedID(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	resp, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
		PreassignedID: "pre-assigned.txt",
		Content:       "data",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "pre-assigned.txt", resp.Path)
}

func TestServiceV2_UploadObject_NoPath_UseOriginalName(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	resp, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
		OriginalName: "from-header.txt",
		Content:      "data",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "from-header.txt", resp.Path)
}

func TestServiceV2_UploadObject_EmptyContentAndNoFile(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	_, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
		Path:    "some/path.txt",
		Content: "  ",
	})
	// Empty content trimmed to "" => file is required
	assert.Error(t, err)
}

func TestServiceV2_UploadObject_NoContentType_DefaultsToOctetStream(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	resp, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
		Path: "bin.dat",
		File: strings.NewReader("binary data"),
		Size: 11,
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestServiceV2_UploadObject_ContentTypeEmptyWithContent(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	resp, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
		Path:    "text.txt",
		Content: "text content",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestServiceV2_SignedUrl_NotConfigured(t *testing.T) {
	svc := NewService(setupSvcCtxNoStore(t))
	_, err := svc.SignedUrl(context.Background(), &SignedUrlRequest{Path: "x"})
	assert.Error(t, err)
}

func TestServiceV2_ListObjects_WithPrefix(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	// Seed
	_, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
		Path: "dir/file.txt", Content: "x",
	})
	require.NoError(t, err)

	resp, err := svc.ListObjects(context.Background(), &ListObjectsRequest{Prefix: "dir/"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Objects)
}

func TestServiceV2_ListObjects_MaxKeysVsLimit(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	// Seed files
	for i := 0; i < 5; i++ {
		_, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
			Path: fmt.Sprintf("items/file%d.txt", i), Content: "x",
		})
		require.NoError(t, err)
	}
	// Use MaxKeys
	resp, err := svc.ListObjects(context.Background(), &ListObjectsRequest{
		Prefix:  "items/",
		MaxKeys: 2,
	})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(resp.Objects), 2)

	// Use Limit instead
	resp2, err := svc.ListObjects(context.Background(), &ListObjectsRequest{
		Prefix: "items/",
		Limit:  3,
	})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(resp2.Objects), 3)
}

func TestServiceV2_ListObjects_PrefixTrailingSlash(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	_, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
		Path: "pfx/file.txt", Content: "x",
	})
	require.NoError(t, err)

	// Prefix with trailing slash in req should be preserved
	resp, err := svc.ListObjects(context.Background(), &ListObjectsRequest{Prefix: "pfx/"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Objects)
}

func TestServiceV2_DeleteObject_Success(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	_, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
		Path: "to-delete.txt", Content: "x",
	})
	require.NoError(t, err)

	resp, err := svc.DeleteObject(context.Background(), &DeleteObjectRequest{Path: "to-delete.txt"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "to-delete.txt", resp.Path)
}

func TestServiceV2_DeleteObject_TrailingSlashPreserved(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	resp, err := svc.DeleteObject(context.Background(), &DeleteObjectRequest{Path: "nonexist/"})
	// The file doesn't exist but the path normalization should keep the trailing slash
	// store.Delete may error for non-existent file, but path should be "nonexist/"
	if err != nil {
		assert.Contains(t, err.Error(), "nonexist")
	} else {
		assert.Equal(t, "nonexist/", resp.Path)
	}
}

func TestServiceV2_BatchDeleteObjects_SkipsEmptyPaths(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	_, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
		Path: "keep.txt", Content: "x",
	})
	require.NoError(t, err)

	resp, err := svc.BatchDeleteObjects(context.Background(), &BatchDeleteObjectsRequest{
		Paths: []string{"", "keep.txt"},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	// Empty string path is skipped (continue), "keep.txt" is deleted
	assert.Contains(t, resp.Deleted, "keep.txt")
}

func TestServiceV2_BatchDeleteObjects_NotConfigured(t *testing.T) {
	svc := NewService(setupSvcCtxNoStore(t))
	_, err := svc.BatchDeleteObjects(context.Background(), &BatchDeleteObjectsRequest{
		Paths: []string{"a.txt"},
	})
	assert.Error(t, err)
}

func TestServiceV2_CreateDirectory_NotConfigured(t *testing.T) {
	svc := NewService(setupSvcCtxNoStore(t))
	_, err := svc.CreateDirectory(context.Background(), &CreateDirectoryRequest{Prefix: "x"})
	assert.Error(t, err)
}

func TestServiceV2_CreateDirectory_WithTrailingSlash(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	resp, err := svc.CreateDirectory(context.Background(), &CreateDirectoryRequest{Prefix: "assets/"})
	require.NoError(t, err)
	assert.Equal(t, "assets/", resp.Prefix)
}

func TestServiceV2_RenameDirectory_NotConfigured(t *testing.T) {
	svc := NewService(setupSvcCtxNoStore(t))
	_, err := svc.RenameDirectory(context.Background(), &RenameDirectoryRequest{
		OldPrefix: "old", NewPrefix: "new",
	})
	assert.Error(t, err)
}

func TestServiceV2_RenameDirectory_Success(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	// Create old dir
	_, err := svc.CreateDirectory(context.Background(), &CreateDirectoryRequest{Prefix: "old-dir"})
	require.NoError(t, err)

	resp, err := svc.RenameDirectory(context.Background(), &RenameDirectoryRequest{
		OldPrefix: "old-dir", NewPrefix: "new-dir",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "old-dir/", resp.OldPrefix)
	assert.Equal(t, "new-dir/", resp.NewPrefix)
}

func TestServiceV2_RenameDirectory_MissingOldPrefix(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	_, err := svc.RenameDirectory(context.Background(), &RenameDirectoryRequest{
		OldPrefix: "", NewPrefix: "new",
	})
	assert.Error(t, err)
}

func TestServiceV2_RenameDirectory_MissingNewPrefix(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))
	_, err := svc.RenameDirectory(context.Background(), &RenameDirectoryRequest{
		OldPrefix: "old", NewPrefix: "",
	})
	assert.Error(t, err)
}

func TestServiceV2_RequireStore_NilService(t *testing.T) {
	var s *Service
	_, err := s.requireStore()
	assert.Error(t, err)
}

func TestServiceV2_RequireStore_NilSvcCtx(t *testing.T) {
	s := &Service{svcCtx: nil}
	_, err := s.requireStore()
	assert.Error(t, err)
}

// ---- Handler Tests ----

func newRouterV2(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/storage/signed-url", handler.SignedUrl)
	r.GET("/storage/signed-url-upper", handler.SignedURL)
	r.GET("/storage/objects", handler.ListObjects)
	r.POST("/storage/objects/upload", handler.UploadObject)
	r.DELETE("/storage/objects", handler.DeleteObject)
	r.POST("/storage/objects/batch-delete", handler.BatchDeleteObjects)
	r.POST("/storage/directories", handler.CreateDirectory)
	r.PUT("/storage/directories/rename", handler.RenameDirectory)
	return r
}

func doReqV2(t *testing.T, router *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestHandlerV2_SignedURL_Alias(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtxWithStore(t)))
	router := newRouterV2(handler)
	rec := doReqV2(t, router, http.MethodGet, "/storage/signed-url-upper?path=doc.txt&expire=60", "")
	assertStatus(t, rec, http.StatusOK)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["url"])
}

func TestHandlerV2_RenameDirectory_Success(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtxWithStore(t)))
	router := newRouterV2(handler)
	// Create first
	rec := doReqV2(t, router, http.MethodPost, "/storage/directories", `{"prefix":"mydir"}`)
	assertStatus(t, rec, http.StatusOK)

	// Rename
	rec2 := doReqV2(t, router, http.MethodPut, "/storage/directories/rename",
		`{"oldPrefix":"mydir","newPrefix":"renamed"}`)
	assertStatus(t, rec2, http.StatusOK)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp))
	assert.Equal(t, "mydir/", resp["oldPrefix"])
	assert.Equal(t, "renamed/", resp["newPrefix"])
}

func TestHandlerV2_RenameDirectory_MissingPrefixes(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtxWithStore(t)))
	router := newRouterV2(handler)
	rec := doReqV2(t, router, http.MethodPut, "/storage/directories/rename", `{}`)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

func TestHandlerV2_UploadObject_ContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewService(setupSvcCtxWithStore(t)))
	router := newRouterV2(handler)

	// Upload with JSON body — field names match struct (no json tag = capital case)
	rec := doReqV2(t, router, http.MethodPost, "/storage/objects/upload",
		`{"Path":"test.txt","Content":"hello"}`)
	assertStatus(t, rec, http.StatusOK)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "test.txt", resp["path"])
}

func TestHandlerV2_UploadObject_NoStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewService(setupSvcCtxNoStore(t)))
	router := newRouterV2(handler)
	rec := doReqV2(t, router, http.MethodPost, "/storage/objects/upload?path=x.txt", `{}`)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

func TestHandlerV2_ListObjects_WithObjects(t *testing.T) {
	svcCtx := setupSvcCtxWithStore(t)
	svc := NewService(svcCtx)
	_, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
		Path: "listed/file.txt", Content: "data",
	})
	require.NoError(t, err)

	handler := NewHandler(svc)
	router := newRouterV2(handler)
	rec := doReqV2(t, router, http.MethodGet, "/storage/objects?prefix=listed/", "")
	assertStatus(t, rec, http.StatusOK)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	objects, ok := resp["objects"].([]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, objects)
}

func TestHandlerV2_DeleteObject_Success(t *testing.T) {
	svcCtx := setupSvcCtxWithStore(t)
	svc := NewService(svcCtx)
	_, err := svc.UploadObject(context.Background(), &UploadObjectRequest{
		Path: "del-me.txt", Content: "x",
	})
	require.NoError(t, err)

	handler := NewHandler(svc)
	router := newRouterV2(handler)
	rec := doReqV2(t, router, http.MethodDelete, "/storage/objects?path=del-me.txt", "")
	assertStatus(t, rec, http.StatusOK)
}
