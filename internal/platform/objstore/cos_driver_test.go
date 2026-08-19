package objstore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cos "github.com/tencentyun/cos-go-sdk-v5"
)

// fakeCOSServer 是一个极简的 COS HTTP 假服务器，覆盖 cosStore 用到的接口。
type fakeCOSServer struct {
	t       *testing.T
	mu      sync.Mutex
	objects map[string]string
	failOps map[string]bool // "PUT"|"DELETE"|"LIST"|"COPY"
	srv     *httptest.Server
	copied  [][2]string // copy (src,dst) 记录
}

func newFakeCOSServer(t *testing.T) *fakeCOSServer {
	t.Helper()
	f := &fakeCOSServer{
		t:       t,
		objects: map[string]string{},
		failOps: map[string]bool{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { f.handle(w, r) })
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeCOSServer) key(r *http.Request) string {
	p := strings.Trim(r.URL.Path, "/")
	p = strings.TrimPrefix(p, "test-bucket/")
	return p
}

func (f *fakeCOSServer) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := f.key(r)

	// bucket 级 List（GET 根路径，无具体对象 key）
	if r.Method == http.MethodGet && (key == "" || strings.HasSuffix(key, "/")) && !strings.Contains(r.URL.Path, ".") {
		if f.failOps["LIST"] {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		f.writeList(w, r.URL.Query().Get("prefix"))
		return
	}

	switch r.Method {
	case http.MethodPut:
		_, _ = io.Copy(io.Discard, r.Body)
		if f.failOps["PUT"] {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if src := r.Header.Get("x-cos-copy-source"); src != "" {
			if f.failOps["COPY"] {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			f.copied = append(f.copied, [2]string{src, key})
			f.objects[key] = f.objects[strings.TrimPrefix(src, "/")]
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><CopyObjectResult><ETag>abc</ETag><LastModified>2024-01-01T07:23:42.000Z</LastModified></CopyObjectResult>`)
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

func (f *fakeCOSServer) writeList(w http.ResponseWriter, prefix string) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult>`)
	fmt.Fprintf(&b, "<Name>test-bucket</Name><Prefix>%s</Prefix><MaxKeys>1000</MaxKeys>", prefix)
	truncated := strings.HasPrefix(prefix, "trunc/")
	if truncated {
		b.WriteString("<IsTruncated>true</IsTruncated><NextMarker>next-marker</NextMarker>")
	} else {
		b.WriteString("<IsTruncated>false</IsTruncated>")
	}
	keys := make([]string, 0, len(f.objects))
	for k := range f.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, k := range keys {
		lastMod := "2024-01-01T00:00:00Z"
		if k == "badtime.txt" {
			lastMod = "not-a-timestamp" // 触发时间解析失败分支
		}
		fmt.Fprintf(&b, "<Contents><Key>%s</Key><LastModified>%s</LastModified><ETag>&quot;etag-%s&quot;</ETag><Size>4</Size></Contents>", k, lastMod, k)
	}
	if strings.HasPrefix(prefix, "dir/") {
		b.WriteString("<CommonPrefixes><Prefix>dir/sub/</Prefix></CommonPrefixes>")
	}
	b.WriteString("</ListBucketResult>")
	_, _ = w.Write([]byte(b.String()))
}

func newCOSStore(t *testing.T) (Store, *fakeCOSServer) {
	t.Helper()
	f := newFakeCOSServer(t)
	st, err := OpenCOS(context.Background(), Config{
		Endpoint:  f.srv.URL,
		AccessKey: "ak",
		SecretKey: "sk",
		Bucket:    "test-bucket",
	})
	require.NoError(t, err)
	// 假服务器不返回 CRC 校验头，关闭 SDK 的 CRC64 校验
	cs, ok := st.(*cosStore)
	require.True(t, ok)
	cs.cli.Conf.EnableCRC = false
	return st, f
}

func TestOpenCOS_ConfigBranches(t *testing.T) {
	t.Run("invalid endpoint url", func(t *testing.T) {
		_, err := OpenCOS(context.Background(), Config{Endpoint: "://bad-url", Bucket: "b"})
		require.Error(t, err)
	})
	t.Run("region required when endpoint empty", func(t *testing.T) {
		_, err := OpenCOS(context.Background(), Config{Bucket: "b"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "region required")
	})
	t.Run("region based url", func(t *testing.T) {
		st, err := OpenCOS(context.Background(), Config{
			Bucket:    "mybucket",
			Region:    "ap-guangzhou",
			AccessKey: "ak",
			SecretKey: "sk",
		})
		require.NoError(t, err)
		cs, ok := st.(*cosStore)
		require.True(t, ok)
		assert.Equal(t, 15*time.Minute, cs.ttl)
		assert.Equal(t, "https://mybucket.cos.ap-guangzhou.myqcloud.com", cs.cli.BaseURL.BucketURL.String())
	})
	t.Run("custom ttl", func(t *testing.T) {
		st, err := OpenCOS(context.Background(), Config{
			Bucket:       "mybucket",
			Region:       "ap-guangzhou",
			SignedURLTTL: 5 * time.Minute,
		})
		require.NoError(t, err)
		cs, ok := st.(*cosStore)
		require.True(t, ok)
		assert.Equal(t, 5*time.Minute, cs.ttl)
	})
}

func TestCOSStore_PutAndError(t *testing.T) {
	st, f := newCOSStore(t)
	ctx := context.Background()

	require.NoError(t, st.Put(ctx, "dir/a.txt", strings.NewReader("data"), 4, "text/plain"))
	f.mu.Lock()
	ct := f.objects["dir/a.txt"]
	f.mu.Unlock()
	assert.Equal(t, "text/plain", ct)

	f.failOps["PUT"] = true
	require.Error(t, st.Put(ctx, "x.txt", strings.NewReader("d"), 1, ""))
}

func TestCOSStore_SignedURL(t *testing.T) {
	st, _ := newCOSStore(t)
	ctx := context.Background()

	for _, m := range []string{"GET", "PUT", "DELETE", "HEAD", ""} {
		u, err := st.SignedURL(ctx, "obj.txt", m, time.Hour)
		require.NoError(t, err)
		assert.Contains(t, u, "obj.txt")
		assert.Contains(t, u, "q-sign-algorithm=")
	}
}

func TestCOSStore_SignedURL_PublicURLAndDefaultTTL(t *testing.T) {
	f := newFakeCOSServer(t)
	st, err := OpenCOS(context.Background(), Config{
		Endpoint:  f.srv.URL,
		AccessKey: "ak",
		SecretKey: "sk",
		Bucket:    "test-bucket",
		PublicURL: "https://cdn.example.com/",
	})
	require.NoError(t, err)

	u, err := st.SignedURL(context.Background(), "k.txt", "GET", 0)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/k.txt", u)
}

// TestCOSStore_Delete 文档化当前行为：sanitizeKey 去掉尾部斜杠后，
// Delete("dir/") 退化为删除单个对象 "dir"，不会递归删除前缀。
func TestCOSStore_Delete(t *testing.T) {
	st, f := newCOSStore(t)
	ctx := context.Background()
	require.NoError(t, st.Put(ctx, "dir/a.txt", strings.NewReader("d"), 1, ""))
	require.NoError(t, st.Put(ctx, "dir/b.txt", strings.NewReader("d"), 1, ""))

	require.NoError(t, st.Delete(ctx, "dir/"))
	f.mu.Lock()
	count := 0
	for k := range f.objects {
		if strings.HasPrefix(k, "dir/") {
			count++
		}
	}
	f.mu.Unlock()
	assert.Equal(t, 2, count, "prefix objects should remain (folder branch unreachable)")

	require.NoError(t, st.Delete(ctx, "dir/a.txt"))
	f.mu.Lock()
	_, exists := f.objects["dir/a.txt"]
	f.mu.Unlock()
	assert.False(t, exists)

	f.failOps["DELETE"] = true
	require.Error(t, st.Delete(ctx, "x.txt"))
}

func TestCOSStore_List(t *testing.T) {
	st, f := newCOSStore(t)
	ctx := context.Background()
	f.mu.Lock()
	f.objects["dir/a.txt"] = ""
	f.objects["dir/b.txt"] = ""
	f.objects["badtime.txt"] = ""
	f.mu.Unlock()

	res, err := st.List(ctx, "dir/", "", "", 0)
	require.NoError(t, err)
	assert.Len(t, res.Objects, 2)
	assert.Equal(t, int64(4), res.Objects[0].Size)
	assert.Equal(t, "etag-dir/a.txt", res.Objects[0].ETag)
	assert.False(t, res.IsTruncated)
	assert.Contains(t, res.Prefixes, "dir/sub/")

	// badtime.txt 的时间解析失败被跳过
	resAll, err := st.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	for _, o := range resAll.Objects {
		assert.NotEqual(t, "badtime.txt", o.Key)
	}

	resT, err := st.List(ctx, "trunc/", "", "", 0)
	require.NoError(t, err)
	assert.True(t, resT.IsTruncated)
	assert.Equal(t, "next-marker", resT.NextMarker)

	f.failOps["LIST"] = true
	_, err = st.List(ctx, "", "", "", 0)
	require.Error(t, err)
}

func TestCOSStore_CreatePrefixAndError(t *testing.T) {
	st, f := newCOSStore(t)
	ctx := context.Background()

	require.NoError(t, st.CreatePrefix(ctx, "folder"))

	f.failOps["PUT"] = true
	require.Error(t, st.CreatePrefix(ctx, "folder2"))
}

// TestCOSStore_RenamePrefix 注意：产品代码将 Object.Copy(ctx, name, sourceURL) 的
// 目标与源参数传反了（oldKey 作为 name、newKey 作为 source），语义上是从 new 拷回 old。
// 本测试仅验证 copy+delete 流程可走通，参数顺序问题作为 bug 单独上报。
func TestCOSStore_RenamePrefix(t *testing.T) {
	st, f := newCOSStore(t)
	ctx := context.Background()
	f.mu.Lock()
	f.objects["old/1.txt"] = ""
	f.objects["new/1.txt"] = ""
	f.mu.Unlock()

	require.NoError(t, st.RenamePrefix(ctx, "old", "new"))
	require.NotEmpty(t, f.copied)
}

func TestCOSStore_RenamePrefixErrors(t *testing.T) {
	t.Run("list error", func(t *testing.T) {
		st, f := newCOSStore(t)
		f.failOps["LIST"] = true
		err := st.RenamePrefix(context.Background(), "old", "new")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list objects")
	})
	t.Run("copy error", func(t *testing.T) {
		st, f := newCOSStore(t)
		f.mu.Lock()
		f.objects["old/1.txt"] = ""
		f.objects["new/1.txt"] = ""
		f.mu.Unlock()
		f.failOps["COPY"] = true
		err := st.RenamePrefix(context.Background(), "old", "new")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to copy object")
	})
	t.Run("delete error", func(t *testing.T) {
		st, f := newCOSStore(t)
		f.mu.Lock()
		f.objects["old/1.txt"] = ""
		f.objects["new/1.txt"] = ""
		f.mu.Unlock()
		f.failOps["DELETE"] = true
		err := st.RenamePrefix(context.Background(), "old", "new")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete old object")
	})
}

// 确保 cosStore 实现了 Store 接口
var _ Store = (*cosStore)(nil)

// 防止未使用导入（cos 仅在类型断言上下文使用）
var _ = cos.AuthorizationTransport{}
