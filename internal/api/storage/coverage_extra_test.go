package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	objstore "github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingStore lets each test inject errors into specific Store operations.
type failingStore struct {
	putErr        error
	signedURLErr  error
	deleteErr     error
	listErr       error
	createPrefErr error
	renamePrefErr error

	putKeys     []string
	deletedKeys []string
}

func (f *failingStore) Put(_ context.Context, key string, _ objstore.ReadSeeker, _ int64, _ string) error {
	f.putKeys = append(f.putKeys, key)
	return f.putErr
}

func (f *failingStore) SignedURL(_ context.Context, key, _ string, _ time.Duration) (string, error) {
	if f.signedURLErr != nil {
		return "", f.signedURLErr
	}
	return "https://signed/" + key, nil
}

func (f *failingStore) Delete(_ context.Context, key string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deletedKeys = append(f.deletedKeys, key)
	return nil
}

func (f *failingStore) List(_ context.Context, _, _, _ string, _ int) (objstore.ListResult, error) {
	if f.listErr != nil {
		return objstore.ListResult{}, f.listErr
	}
	return objstore.ListResult{Objects: []objstore.ObjectInfo{{Key: "a.txt", Size: 1}}}, nil
}

func (f *failingStore) CreatePrefix(_ context.Context, _ string) error { return f.createPrefErr }

func (f *failingStore) RenamePrefix(_ context.Context, _, _ string) error { return f.renamePrefErr }

// errSeeker always fails on Seek to cover the reset-stream failure branch.
type errSeeker struct{}

func (errSeeker) Read([]byte) (int, error)       { return 0, nil }
func (errSeeker) Seek(int64, int) (int64, error) { return 0, errors.New("seek failed") }

// --- helpers -------------------------------------------------------------

func extraRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/storage/signed-url", handler.SignedUrl)
	r.GET("/storage/signed-url-alias", handler.SignedURL)
	r.GET("/storage/objects", handler.ListObjects)
	r.POST("/storage/objects", handler.UploadObject)
	r.DELETE("/storage/objects", handler.DeleteObject)
	r.POST("/storage/objects/batch-delete", handler.BatchDeleteObjects)
	r.POST("/storage/directories", handler.CreateDirectory)
	r.POST("/storage/directories/rename", handler.RenameDirectory)
	return r
}

func doExtraReq(t *testing.T, router *gin.Engine, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	return rec
}

func jsonBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	m := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m), "body=%s", rec.Body.String())
	return m
}

// firstErr unwraps the (response, error) pair so callers can require success.
func firstErr[T any](r T, err error) error { return err }

func multipartUpload(t *testing.T, target string, fields map[string]string, filename, content string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		require.NoError(t, w.WriteField(k, v))
	}
	if filename != "" {
		fw, err := w.CreateFormFile("file", filename)
		require.NoError(t, err)
		_, err = fw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	req := httptest.NewRequest(http.MethodPost, target, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

// --- handler tests -------------------------------------------------------

func TestExtraHandler_SignedUrl_BindError(t *testing.T) {
	router := extraRouter(setupHandler(t))
	rec := doExtraReq(t, router, httptest.NewRequest(http.MethodGet, "/storage/signed-url?path=a&expire=abc", nil))
	assert.True(t, rec.Code >= 400, "expected error status, got %d body=%s", rec.Code, rec.Body.String())
	assertErrorShape(t, rec)
}

func TestExtraHandler_SignedUrl_AliasRoute(t *testing.T) {
	router := extraRouter(setupHandler(t))
	rec := doExtraReq(t, router, httptest.NewRequest(http.MethodGet, "/storage/signed-url-alias?path=doc.txt&expire=30", nil))
	assertStatus(t, rec, http.StatusOK)
	assert.NotEmpty(t, jsonBody(t, rec)["url"])
}

func TestExtraHandler_ListObjects_BindError(t *testing.T) {
	router := extraRouter(setupHandler(t))
	rec := doExtraReq(t, router, httptest.NewRequest(http.MethodGet, "/storage/objects?maxKeys=oops", nil))
	assert.True(t, rec.Code >= 400, "expected error status, got %d body=%s", rec.Code, rec.Body.String())
	assertErrorShape(t, rec)
}

func TestExtraHandler_UploadObject_MultipartWithFile(t *testing.T) {
	svcCtx := setupSvcCtxWithStore(t)
	handler := NewHandler(NewService(svcCtx))
	router := extraRouter(handler)

	req := multipartUpload(t, "/storage/objects",
		map[string]string{"path": "", "contentType": ""},
		"photo.png", "png-bytes")

	rec := doExtraReq(t, router, req)
	assertStatus(t, rec, http.StatusOK)
	body := jsonBody(t, rec)
	assert.Equal(t, "photo.png", body["path"])
}

func TestExtraHandler_UploadObject_MultipartWithPathAndContentType(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtxWithStore(t)))
	router := extraRouter(handler)

	req := multipartUpload(t, "/storage/objects",
		map[string]string{"path": "custom/name.txt", "contentType": "text/plain"},
		"ignored-name.txt", "hello")

	rec := doExtraReq(t, router, req)
	assertStatus(t, rec, http.StatusOK)
	assert.Equal(t, "custom/name.txt", jsonBody(t, rec)["path"])
}

func TestExtraHandler_UploadObject_InvalidJSON_Returns400(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtxWithStore(t)))
	router := extraRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/storage/objects", strings.NewReader("{broken"))
	req.Header.Set("Content-Type", "application/json")
	rec := doExtraReq(t, router, req)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

func TestExtraHandler_UploadObject_NoFile_Returns400(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtxWithStore(t)))
	router := extraRouter(handler)

	req := multipartUpload(t, "/storage/objects", map[string]string{}, "", "")
	rec := doExtraReq(t, router, req)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

func TestExtraHandler_RenameDirectory_OK(t *testing.T) {
	svcObj := NewService(setupSvcCtxWithStore(t))
	require.NoError(t, firstErr(svcObj.CreateDirectory(context.Background(), &CreateDirectoryRequest{Prefix: "old"})))
	handler := NewHandler(svcObj)
	router := extraRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/storage/directories/rename",
		strings.NewReader(`{"oldPrefix":"old","newPrefix":"new"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := doExtraReq(t, router, req)
	assertStatus(t, rec, http.StatusOK)
	body := jsonBody(t, rec)
	assert.Equal(t, "old/", body["oldPrefix"])
	assert.Equal(t, "new/", body["newPrefix"])
}

func TestExtraHandler_RenameDirectory_MissingPrefixes_Returns400(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtxWithStore(t)))
	router := extraRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/storage/directories/rename", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := doExtraReq(t, router, req)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

func TestExtraHandler_RenameDirectory_InvalidJSON_Returns400(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtxWithStore(t)))
	router := extraRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/storage/directories/rename", strings.NewReader(`nope`))
	req.Header.Set("Content-Type", "application/json")
	rec := doExtraReq(t, router, req)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

// --- service tests -------------------------------------------------------

func TestExtraService_NormalizeStoragePath_DotOnly(t *testing.T) {
	assert.Equal(t, "", normalizeStoragePath("."))
	assert.Equal(t, "", normalizeStoragePath("./"))
	assert.Equal(t, "a/b", normalizeStoragePath(`\a\..\b\.`))
	assert.Equal(t, "", normalizeStoragePath("  "))
}

func TestExtraService_NotConfigured_AllMethodsRejected(t *testing.T) {
	svcObj := NewService(&svc.ServiceContext{})
	ctx := context.Background()

	for name, call := range map[string]func() (*string, error){
		"SignedUrl": func() (*string, error) {
			r, err := svcObj.SignedUrl(ctx, &SignedUrlRequest{Path: "x"})
			if r != nil {
				return &r.URL, err
			}
			return nil, err
		},
		"ListObjects": func() (*string, error) {
			r, err := svcObj.ListObjects(ctx, &ListObjectsRequest{})
			if r != nil {
				return &r.NextMarker, err
			}
			return nil, err
		},
		"UploadObject": func() (*string, error) {
			r, err := svcObj.UploadObject(ctx, &UploadObjectRequest{Path: "x"})
			if r != nil {
				return &r.Path, err
			}
			return nil, err
		},
		"DeleteObject": func() (*string, error) {
			r, err := svcObj.DeleteObject(ctx, &DeleteObjectRequest{Path: "x"})
			if r != nil {
				return &r.Path, err
			}
			return nil, err
		},
		"BatchDeleteObjects": func() (*string, error) {
			r, err := svcObj.BatchDeleteObjects(ctx, &BatchDeleteObjectsRequest{Paths: []string{"x"}})
			if r != nil {
				return &r.Failed[0], err
			}
			return nil, err
		},
		"CreateDirectory": func() (*string, error) {
			r, err := svcObj.CreateDirectory(ctx, &CreateDirectoryRequest{Prefix: "x"})
			if r != nil {
				return &r.Prefix, err
			}
			return nil, err
		},
		"RenameDirectory": func() (*string, error) {
			r, err := svcObj.RenameDirectory(ctx, &RenameDirectoryRequest{OldPrefix: "a", NewPrefix: "b"})
			if r != nil {
				return &r.NewPrefix, err
			}
			return nil, err
		},
	} {
		_, err := call()
		assert.Error(t, err, "%s should reject when store is not configured", name)
	}
}

func TestExtraService_SignedURL_StoreError(t *testing.T) {
	store := &failingStore{signedURLErr: errors.New("sign denied")}
	svcObj := NewService(&svc.ServiceContext{ObjectStore: store})

	resp, err := svcObj.SignedUrl(context.Background(), &SignedUrlRequest{Path: "doc.txt"})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "获取签名链接失败")
}

func TestExtraService_ListObjects_StoreError(t *testing.T) {
	store := &failingStore{listErr: errors.New("list boom")}
	svcObj := NewService(&svc.ServiceContext{ObjectStore: store})

	resp, err := svcObj.ListObjects(context.Background(), &ListObjectsRequest{Prefix: "p"})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "获取对象列表失败")
}

func TestExtraService_ListObjects_PrefixAndLimitHandling(t *testing.T) {
	store := &failingStore{}
	svcObj := NewService(&svc.ServiceContext{ObjectStore: store})

	resp, err := svcObj.ListObjects(context.Background(), &ListObjectsRequest{
		Prefix: "folder/sub/", Marker: "../x", Delimiter: "/", MaxKeys: 7,
	})
	require.NoError(t, err)
	require.Len(t, resp.Objects, 1)
	assert.Equal(t, "a.txt", resp.Objects[0].Key)

	// MaxKeys <= 0 falls back to Limit.
	_, err = svcObj.ListObjects(context.Background(), &ListObjectsRequest{Prefix: "folder/", Limit: 3})
	require.NoError(t, err)
}

func TestExtraService_UploadObject_StorePutError(t *testing.T) {
	store := &failingStore{putErr: errors.New("put rejected")}
	svcObj := NewService(&svc.ServiceContext{ObjectStore: store})

	resp, err := svcObj.UploadObject(context.Background(), &UploadObjectRequest{
		Path: "a.txt", File: strings.NewReader("data"), Size: 4,
	})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "上传对象失败")
}

func TestExtraService_UploadObject_SeekError(t *testing.T) {
	store := &failingStore{}
	svcObj := NewService(&svc.ServiceContext{ObjectStore: store})

	resp, err := svcObj.UploadObject(context.Background(), &UploadObjectRequest{
		Path: "a.txt", File: errSeeker{}, Size: 1,
	})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "重置上传流失败")
}

func TestExtraService_UploadObject_PathFallbacks(t *testing.T) {
	store := &failingStore{}
	svcObj := NewService(&svc.ServiceContext{ObjectStore: store})
	ctx := context.Background()

	// Falls back to PreassignedID when Path empty.
	resp, err := svcObj.UploadObject(ctx, &UploadObjectRequest{
		PreassignedID: "pre/id.bin", File: strings.NewReader("x"), Size: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, "pre/id.bin", resp.Path)

	// Falls back to OriginalName next.
	resp, err = svcObj.UploadObject(ctx, &UploadObjectRequest{
		OriginalName: "orig/file.txt", File: strings.NewReader("x"), Size: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, "orig/file.txt", resp.Path)

	// Content-only upload derives size and default content type.
	resp, err = svcObj.UploadObject(ctx, &UploadObjectRequest{Path: "c.txt", Content: "text-body"})
	require.NoError(t, err)
	assert.Equal(t, "c.txt", resp.Path)
	assert.ElementsMatch(t, []string{"pre/id.bin", "orig/file.txt", "c.txt"}, store.putKeys)
}

func TestExtraService_DeleteObject_StoreError(t *testing.T) {
	store := &failingStore{deleteErr: errors.New("delete denied")}
	svcObj := NewService(&svc.ServiceContext{ObjectStore: store})

	resp, err := svcObj.DeleteObject(context.Background(), &DeleteObjectRequest{Path: "gone.txt"})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "删除对象失败")
}

func TestExtraService_BatchDeleteObjects_PartialFailure(t *testing.T) {
	store := &failingStore{deleteErr: errors.New("locked")}
	svcObj := NewService(&svc.ServiceContext{ObjectStore: store})

	resp, err := svcObj.BatchDeleteObjects(context.Background(), &BatchDeleteObjectsRequest{
		Paths: []string{"a.txt", "", "dir/b.txt"},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Deleted)
	assert.ElementsMatch(t, []string{"a.txt", "dir/b.txt"}, resp.Failed)
}

func TestExtraService_CreateDirectory_StoreError(t *testing.T) {
	store := &failingStore{createPrefErr: errors.New("mkdir denied")}
	svcObj := NewService(&svc.ServiceContext{ObjectStore: store})

	resp, err := svcObj.CreateDirectory(context.Background(), &CreateDirectoryRequest{Prefix: "assets"})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "创建目录失败")
}

func TestExtraService_RenameDirectory_MissingPrefixes(t *testing.T) {
	svcObj := NewService(&svc.ServiceContext{ObjectStore: &failingStore{}})
	ctx := context.Background()

	for _, req := range []*RenameDirectoryRequest{
		{},
		{NewPrefix: "new"},
		{OldPrefix: "old"},
		{OldPrefix: ".", NewPrefix: "new"},
		{OldPrefix: "old", NewPrefix: "./"},
	} {
		resp, err := svcObj.RenameDirectory(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	}
}

func TestExtraService_RenameDirectory_StoreError(t *testing.T) {
	store := &failingStore{renamePrefErr: errors.New("rename denied")}
	svcObj := NewService(&svc.ServiceContext{ObjectStore: store})

	resp, err := svcObj.RenameDirectory(context.Background(), &RenameDirectoryRequest{
		OldPrefix: "old", NewPrefix: "new",
	})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "重命名目录失败")
}

func TestExtraService_NilServiceRequireStore(t *testing.T) {
	var svcObj *Service
	_, err := svcObj.requireStore()
	assert.Error(t, err)
}
