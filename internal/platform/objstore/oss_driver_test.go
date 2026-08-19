package objstore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOSSServer 是一个极简的 OSS HTTP 假服务器，覆盖 objstore ossStore 用到的接口。
type fakeOSSServer struct {
	t        *testing.T
	mu       sync.Mutex
	objects  map[string]string // key -> contentType
	failOps  map[string]bool   // "PUT"|"DELETE"|"LIST"|"COPY" -> fail
	srv      *httptest.Server
	bucket   string
	copyFrom string // 最近一次 CopyObject 的源
}

func newFakeOSSServer(t *testing.T) *fakeOSSServer {
	t.Helper()
	f := &fakeOSSServer{
		t:       t,
		objects: map[string]string{},
		failOps: map[string]bool{},
		bucket:  "mybucket",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.handle(w, r)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeOSSServer) key(r *http.Request) string {
	p := strings.TrimPrefix(r.URL.Path, "/"+f.bucket+"/")
	return p
}

func (f *fakeOSSServer) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Method == http.MethodGet && r.URL.Path == "/"+f.bucket+"/" || r.URL.Path == "/"+f.bucket {
		if f.failOps["LIST"] {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		prefix := r.URL.Query().Get("prefix")
		f.writeList(w, prefix, r.URL.Query().Get("marker"))
		return
	}
	key := f.key(r)
	switch r.Method {
	case http.MethodPut:
		_, _ = io.Copy(io.Discard, r.Body)
		if f.failOps["PUT"] {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if src := r.Header.Get("X-Oss-Copy-Source"); src != "" {
			if f.failOps["COPY"] {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			f.copyFrom = src
			// 源格式: /mybucket/<url-escaped-key>
			decoded, err := url.QueryUnescape(src)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			srcKey := strings.TrimPrefix(decoded, "/"+f.bucket+"/")
			if _, ok := f.objects[srcKey]; !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			f.objects[key] = f.objects[srcKey]
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><CopyObjectResult><LastModified>2024-01-01T00:00:00Z</LastModified><ETag>"abc"</ETag></CopyObjectResult>`)
			return
		}
		f.objects[key] = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		if f.failOps["DELETE"] {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		delete(f.objects, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeOSSServer) writeList(w http.ResponseWriter, prefix, marker string) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult>`)
	b.WriteString("<Name>mybucket</Name>")
	fmt.Fprintf(&b, "<Prefix>%s</Prefix><Marker>%s</Marker><MaxKeys>1000</MaxKeys>", prefix, marker)
	truncated := strings.HasPrefix(prefix, "trunc/")
	if truncated {
		b.WriteString("<IsTruncated>true</IsTruncated><NextMarker>next-marker</NextMarker>")
	} else {
		b.WriteString("<IsTruncated>false</IsTruncated>")
	}
	keys := make([]string, 0, len(f.objects))
	for k := range f.objects {
		if strings.HasPrefix(k, prefix) && k > marker {
			keys = append(keys, k)
		}
	}
	// 简单排序保证输出稳定
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, k := range keys {
		if strings.HasSuffix(k, "/") {
			continue // 目录标记不作为 Contents 返回，交给 CommonPrefixes
		}
		b.WriteString(`<Contents>`)
		fmt.Fprintf(&b, "<Key>%s</Key><LastModified>2024-01-01T00:00:00Z</LastModified><ETag>&quot;etag-%s&quot;</ETag><Size>4</Size><StorageClass>Standard</StorageClass>", k, k)
		b.WriteString(`</Contents>`)
	}
	if strings.HasPrefix(prefix, "dir/") {
		b.WriteString("<CommonPrefixes><Prefix>dir/sub/</Prefix></CommonPrefixes>")
	}
	b.WriteString("</ListBucketResult>")
	_, _ = w.Write([]byte(b.String()))
}

func newOSSStore(t *testing.T) (Store, *fakeOSSServer) {
	t.Helper()
	f := newFakeOSSServer(t)
	st, err := OpenOSS(context.Background(), Config{
		Endpoint:  f.srv.URL,
		AccessKey: "ak",
		SecretKey: "sk",
		Bucket:    f.bucket,
	})
	require.NoError(t, err)
	return st, f
}

func TestOSSStore_PutCreatesDirMarkers(t *testing.T) {
	st, f := newOSSStore(t)
	ctx := context.Background()

	require.NoError(t, st.Put(ctx, "dir/a.txt", strings.NewReader("data"), 4, "text/plain"))

	f.mu.Lock()
	ct, ok := f.objects["dir/a.txt"]
	markerExists := false
	for k := range f.objects {
		if k == "dir/" {
			markerExists = true
		}
	}
	f.mu.Unlock()
	require.True(t, ok, "object should be stored")
	assert.Equal(t, "text/plain", ct)
	assert.True(t, markerExists, "dir marker should be created")
}

func TestOSSStore_PutError(t *testing.T) {
	st, f := newOSSStore(t)
	f.failOps["PUT"] = true

	err := st.Put(context.Background(), "x.txt", strings.NewReader("d"), 1, "")
	require.Error(t, err)
}

func TestOSSStore_SignedURLMethods(t *testing.T) {
	st, _ := newOSSStore(t)
	ctx := context.Background()

	for _, m := range []string{"GET", "PUT", "DELETE", ""} {
		u, err := st.SignedURL(ctx, "dir/a.txt", m, time.Hour)
		require.NoError(t, err)
		assert.Contains(t, u, "mybucket")
		assert.Contains(t, u, "Signature=")
	}
	_, err := st.SignedURL(ctx, "dir/a.txt", "POST", time.Hour)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported method")
}

func TestOSSStore_SignedURL_PublicURL(t *testing.T) {
	st, _ := newOSSStore(t)
	u, err := st.SignedURL(context.Background(), "k.txt", "GET", 0)
	require.NoError(t, err)
	assert.Contains(t, u, "Signature=")
}

func TestOSSStore_SignedURL_PublicURLOverride(t *testing.T) {
	f := newFakeOSSServer(t)
	st, err := OpenOSS(context.Background(), Config{
		Endpoint:  f.srv.URL,
		AccessKey: "ak",
		SecretKey: "sk",
		Bucket:    f.bucket,
		PublicURL: "https://cdn.example.com",
	})
	require.NoError(t, err)

	u, err := st.SignedURL(context.Background(), "k.txt", "GET", 0)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/k.txt", u)
}

func TestOSSStore_DeleteSingle(t *testing.T) {
	st, f := newOSSStore(t)
	ctx := context.Background()
	require.NoError(t, st.Put(ctx, "x.txt", strings.NewReader("d"), 1, ""))

	require.NoError(t, st.Delete(ctx, "x.txt"))
	f.mu.Lock()
	_, exists := f.objects["x.txt"]
	f.mu.Unlock()
	assert.False(t, exists)
}

// TestOSSStore_DeleteFolderKeySanitized 文档化当前行为：sanitizeKey 去掉尾部斜杠后，
// Delete("dir/") 退化为删除单个对象 "dir"，不会递归删除前缀（递归分支不可达）。
func TestOSSStore_DeleteFolderKeySanitized(t *testing.T) {
	st, f := newOSSStore(t)
	ctx := context.Background()
	require.NoError(t, st.Put(ctx, "dir/a.txt", strings.NewReader("d"), 1, ""))
	require.NoError(t, st.Put(ctx, "dir/b.txt", strings.NewReader("d"), 1, ""))

	// 假服务器对 DELETE 一律返回 204，此处验证请求不会触发 List（递归分支未进入）
	require.NoError(t, st.Delete(ctx, "dir/"))
	f.mu.Lock()
	count := 0
	for k := range f.objects {
		if strings.HasPrefix(k, "dir/") {
			count++ // dir/ 标记 + 两个文件
		}
	}
	f.mu.Unlock()
	assert.Equal(t, 3, count, "prefix objects should remain (folder branch unreachable)")
}

func TestOSSStore_DeleteErrors(t *testing.T) {
	t.Run("delete error", func(t *testing.T) {
		st, f := newOSSStore(t)
		f.failOps["DELETE"] = true
		require.Error(t, st.Delete(context.Background(), "x.txt"))
	})
}

func TestOSSStore_List(t *testing.T) {
	st, f := newOSSStore(t)
	ctx := context.Background()
	f.mu.Lock()
	f.objects["dir/a.txt"] = ""
	f.objects["dir/b.txt"] = ""
	f.mu.Unlock()

	res, err := st.List(ctx, "dir/", "", "", 0)
	require.NoError(t, err)
	assert.Len(t, res.Objects, 2)
	assert.Equal(t, "dir/a.txt", res.Objects[0].Key)
	assert.Equal(t, int64(4), res.Objects[0].Size)
	assert.Equal(t, `"etag-dir/a.txt"`, res.Objects[0].ETag)
	assert.False(t, res.IsTruncated)
	assert.Contains(t, res.Prefixes, "dir/sub/")
}

func TestOSSStore_ListTruncatedAndError(t *testing.T) {
	st, f := newOSSStore(t)
	ctx := context.Background()

	res, err := st.List(ctx, "trunc/", "", "", 0)
	require.NoError(t, err)
	assert.True(t, res.IsTruncated)
	assert.Equal(t, "next-marker", res.NextMarker)

	f.failOps["LIST"] = true
	_, err = st.List(ctx, "any/", "", "", 0)
	require.Error(t, err)
}

func TestOSSStore_CreatePrefixAndError(t *testing.T) {
	st, f := newOSSStore(t)
	ctx := context.Background()

	require.NoError(t, st.CreatePrefix(ctx, "folder"))

	f.failOps["PUT"] = true
	require.Error(t, st.CreatePrefix(ctx, "folder2"))
}

func TestOSSStore_RenamePrefix(t *testing.T) {
	st, f := newOSSStore(t)
	ctx := context.Background()
	f.mu.Lock()
	f.objects["old/1.txt"] = "text/plain"
	f.objects["old/2.txt"] = ""
	f.mu.Unlock()
	// 目标对象需先存在（假服务器 Copy 校验源存在）
	require.NoError(t, st.Put(ctx, "old/1.txt", strings.NewReader("data"), 4, ""))
	require.NoError(t, st.Put(ctx, "old/2.txt", strings.NewReader("data"), 4, ""))

	require.NoError(t, st.RenamePrefix(ctx, "old", "new"))

	f.mu.Lock()
	_, hasNew := f.objects["new/1.txt"]
	_, hasOld := f.objects["old/1.txt"]
	f.mu.Unlock()
	assert.True(t, hasNew)
	assert.False(t, hasOld)
}

func TestOSSStore_RenamePrefixErrors(t *testing.T) {
	t.Run("list error", func(t *testing.T) {
		st, f := newOSSStore(t)
		f.failOps["LIST"] = true
		err := st.RenamePrefix(context.Background(), "old", "new")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list objects")
	})
	t.Run("copy error", func(t *testing.T) {
		st, f := newOSSStore(t)
		f.mu.Lock()
		f.objects["old/1.txt"] = ""
		f.mu.Unlock()
		f.failOps["COPY"] = true
		err := st.RenamePrefix(context.Background(), "old", "new")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to copy object")
	})
	t.Run("delete error", func(t *testing.T) {
		st, f := newOSSStore(t)
		f.mu.Lock()
		f.objects["old/1.txt"] = ""
		f.mu.Unlock()
		f.failOps["DELETE"] = true
		err := st.RenamePrefix(context.Background(), "old", "new")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete old object")
	})
}
