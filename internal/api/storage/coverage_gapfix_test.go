package storage

import (
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	objstore "github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gapfixStore 在 failingStore 基础上记录 List/Delete 收到的 key，
// 用于断言防御分支对入参的修正结果。
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

// withSlashStrippingNormalizer 注入"剥离尾部斜杠"的归一化实现，
// 使补斜杠防御分支可达；结束后还原缝隙。
func withSlashStrippingNormalizer(t *testing.T) {
	t.Helper()
	orig := normalizeStoragePath
	normalizeStoragePath = func(raw string) string { return strings.TrimSuffix(raw, "/") }
	t.Cleanup(func() { normalizeStoragePath = orig })
}

// path.Join 对空参数切片返回 ""，clean == "." 兜底仅在缝隙注入
// 恒返回 "." 的 Join 时可达；验证结果归零且不追加尾部斜杠。
func TestGapfixService_NormalizeStoragePath_JoinDotSeam(t *testing.T) {
	orig := joinStoragePath
	joinStoragePath = func(parts ...string) string { return "." }
	t.Cleanup(func() { joinStoragePath = orig })

	assert.Equal(t, "", normalizeStoragePath("a/b"))
	assert.Equal(t, "", normalizeStoragePath("a/b/"))
}

// gin 的 c.FormFile 内部已先 Open 同一 FileHeader，handler 侧第二次
// Open 失败真实请求不可达；注入故障验证 500 internal_error 透传。
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
	assert.Equal(t, "open spilled file failed", body["message"])
}

// 归一化结果丢失尾部斜杠时，ListObjects 的防御分支应把目录前缀
// 的 "/" 补回后再下推给 store。
func TestGapfixService_ListObjects_PrefixSlashGuard(t *testing.T) {
	withSlashStrippingNormalizer(t)

	store := &gapfixStore{}
	svcObj := NewService(&svc.ServiceContext{ObjectStore: store})

	resp, err := svcObj.ListObjects(context.Background(), &ListObjectsRequest{Prefix: "dir/"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, []string{"dir/"}, store.listedPrefixes)
}

// 归一化结果丢失尾部斜杠时，DeleteObject 的防御分支应补回 "/"
// 并以带斜杠路径调用 store、返回给调用方。
func TestGapfixService_DeleteObject_SlashGuard(t *testing.T) {
	withSlashStrippingNormalizer(t)

	store := &gapfixStore{}
	svcObj := NewService(&svc.ServiceContext{ObjectStore: store})

	resp, err := svcObj.DeleteObject(context.Background(), &DeleteObjectRequest{Path: "dir/"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "dir/", resp.Path)
	assert.Equal(t, []string{"dir/"}, store.deletedKeys)
}

// 归一化结果丢失尾部斜杠时，BatchDeleteObjects 的防御分支应逐条
// 补回 "/"：目录条目被修正，普通文件条目保持原样。
func TestGapfixService_BatchDeleteObjects_SlashGuard(t *testing.T) {
	withSlashStrippingNormalizer(t)

	store := &gapfixStore{}
	svcObj := NewService(&svc.ServiceContext{ObjectStore: store})

	resp, err := svcObj.BatchDeleteObjects(context.Background(), &BatchDeleteObjectsRequest{
		Paths: []string{"dir/", "a.txt"},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, []string{"dir/", "a.txt"}, resp.Deleted)
	assert.Empty(t, resp.Failed)
	assert.Equal(t, []string{"dir/", "a.txt"}, store.deletedKeys)
}
