package objstore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withIdentitySanitizeDelete 注入恒等 key 净化：生产 sanitizeKey 经
// filepath.Clean 恒去除尾部斜杠，文件夹递归删除分支在公开 API 下不可达。
func withIdentitySanitizeDelete(t *testing.T) {
	t.Helper()
	prev := sanitizeDeleteKeyFn
	sanitizeDeleteKeyFn = func(key string) string { return key }
	t.Cleanup(func() { sanitizeDeleteKeyFn = prev })
}

// COS：fakeCOSServer 的 List 会返回前缀下全部 key（含文件夹标记），
// 覆盖 cosStore.Delete 的文件夹递归删除路径。

func TestCOSDelete_FolderRecursionViaSeam(t *testing.T) {
	withIdentitySanitizeDelete(t)
	st, f := newCOSStore(t)
	ctx := context.Background()

	f.mu.Lock()
	f.objects["folder/"] = ""
	f.objects["folder/a.txt"] = ""
	f.objects["folder/b.txt"] = ""
	f.objects["other.txt"] = ""
	f.mu.Unlock()

	require.NoError(t, st.Delete(ctx, "folder/"))

	f.mu.Lock()
	_, hasA := f.objects["folder/a.txt"]
	_, hasB := f.objects["folder/b.txt"]
	_, hasOther := f.objects["other.txt"]
	f.mu.Unlock()
	assert.False(t, hasA, "folder/a.txt should be recursively deleted")
	assert.False(t, hasB, "folder/b.txt should be recursively deleted")
	assert.True(t, hasOther, "objects outside the prefix must remain")
}

func TestCOSDelete_FolderListErrorViaSeam(t *testing.T) {
	withIdentitySanitizeDelete(t)
	st, f := newCOSStore(t)
	f.mu.Lock()
	f.objects["folder/a.txt"] = ""
	f.mu.Unlock()
	f.failOps["LIST"] = true

	err := st.Delete(context.Background(), "folder/")
	require.Error(t, err)
}

func TestCOSDelete_FolderObjectDeleteErrorViaSeam(t *testing.T) {
	withIdentitySanitizeDelete(t)
	st, f := newCOSStore(t)
	f.mu.Lock()
	f.objects["folder/"] = ""
	f.objects["folder/a.txt"] = ""
	f.mu.Unlock()
	f.failOps["DELETE"] = true

	err := st.Delete(context.Background(), "folder/")
	require.Error(t, err)
}

// ossFolderFake 是专为 OSS 文件夹递归删除设计的假服务器：与
// fakeOSSServer 的差异在于 List 结果包含文件夹标记对象本身（真实 OSS
// 行为），使 Delete 能走到“跳过标记 + 递归删除”分支。
type ossFolderFake struct {
	mu         sync.Mutex
	objects    map[string]string
	failList   bool
	failDelete bool
	srv        *httptest.Server
}

func newOSSFolderFake(t *testing.T) *ossFolderFake {
	t.Helper()
	f := &ossFolderFake{objects: map[string]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { f.handle(w, r) })
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *ossFolderFake) key(r *http.Request) string {
	return strings.TrimPrefix(r.URL.Path, "/mybucket/")
}

func (f *ossFolderFake) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if r.Method == http.MethodGet && (r.URL.Path == "/mybucket" || r.URL.Path == "/mybucket/") {
		if f.failList {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		f.writeList(w, r.URL.Query().Get("prefix"))
		return
	}

	key := f.key(r)
	switch r.Method {
	case http.MethodDelete:
		if f.failDelete {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		delete(f.objects, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *ossFolderFake) writeList(w http.ResponseWriter, prefix string) {
	keys := make([]string, 0, len(f.objects))
	for k := range f.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult>`)
	b.WriteString("<Name>mybucket</Name>")
	fmt.Fprintf(&b, "<Prefix>%s</Prefix><Marker></Marker><MaxKeys>1000</MaxKeys><IsTruncated>false</IsTruncated>", prefix)
	for _, k := range keys {
		fmt.Fprintf(&b, `<Contents><Key>%s</Key><LastModified>2024-01-01T00:00:00Z</LastModified><ETag>&quot;etag-%s&quot;</ETag><Size>4</Size><StorageClass>Standard</StorageClass></Contents>`, k, k)
	}
	b.WriteString(`</ListBucketResult>`)
	_, _ = w.Write([]byte(b.String()))
}

func newOSSFolderStore(t *testing.T) (Store, *ossFolderFake) {
	t.Helper()
	f := newOSSFolderFake(t)
	st, err := OpenOSS(context.Background(), Config{
		Endpoint:  f.srv.URL,
		AccessKey: "ak",
		SecretKey: "sk",
		Bucket:    "mybucket",
	})
	require.NoError(t, err)
	return st, f
}

func TestOSSDelete_FolderRecursionViaSeam(t *testing.T) {
	withIdentitySanitizeDelete(t)
	st, f := newOSSFolderStore(t)
	ctx := context.Background()

	f.mu.Lock()
	f.objects["folder/"] = ""
	f.objects["folder/a.txt"] = ""
	f.objects["folder/b.txt"] = ""
	f.objects["keep.txt"] = ""
	f.mu.Unlock()

	require.NoError(t, st.Delete(ctx, "folder/"))

	f.mu.Lock()
	_, hasA := f.objects["folder/a.txt"]
	_, hasB := f.objects["folder/b.txt"]
	_, hasMarker := f.objects["folder/"]
	_, hasKeep := f.objects["keep.txt"]
	f.mu.Unlock()
	assert.False(t, hasA, "folder/a.txt should be recursively deleted")
	assert.False(t, hasB, "folder/b.txt should be recursively deleted")
	assert.False(t, hasMarker, "folder marker itself should be deleted last")
	assert.True(t, hasKeep, "objects outside the prefix must remain")
}

func TestOSSDelete_FolderListErrorViaSeam(t *testing.T) {
	withIdentitySanitizeDelete(t)
	st, f := newOSSFolderStore(t)
	f.mu.Lock()
	f.objects["folder/a.txt"] = ""
	f.mu.Unlock()
	f.failList = true

	err := st.Delete(context.Background(), "folder/")
	require.Error(t, err)
}

func TestOSSDelete_FolderObjectDeleteErrorViaSeam(t *testing.T) {
	withIdentitySanitizeDelete(t)
	st, f := newOSSFolderStore(t)
	f.mu.Lock()
	f.objects["folder/"] = ""
	f.objects["folder/a.txt"] = ""
	f.mu.Unlock()
	f.failDelete = true

	err := st.Delete(context.Background(), "folder/")
	require.Error(t, err)
}
