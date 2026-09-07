package objstore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileStoreValidateAndCleanPathAbsError 在被删除的工作目录中构造
// filepath.Abs 失败，覆盖 validateAndCleanPath 的两个 Abs 错误分支。
func TestFileStoreValidateAndCleanPathAbsError(t *testing.T) {
	orig, err := os.Getwd()
	require.NoError(t, err)
	doomed := t.TempDir()
	require.NoError(t, os.Chdir(doomed))
	require.NoError(t, os.RemoveAll(doomed))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	fs := &fileStore{base: "b2rel"}

	_, errAbs := fs.validateAndCleanPath("b2rel/old/x.txt")
	assert.ErrorContains(t, errAbs, "invalid path")

	_, err = fs.validateAndCleanPath("/abs/x.txt")
	assert.ErrorContains(t, err, "invalid base path")

	require.NoError(t, os.Chdir(orig))
}

// fakeS3Server 是离线的 S3 端点，用于驱动 s3Store.RenamePrefix 的
// 读中途失败与删除失败分支。
type fakeS3Server struct {
	srv       *httptest.Server
	truncGet  atomic.Bool
	failDel   atomic.Bool
	putSeen   atomic.Int32
	getsSeen  atomic.Int32
	failedGet atomic.Bool
}

func newFakeS3Server(t *testing.T) *fakeS3Server {
	t.Helper()
	f := &fakeS3Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/b2bkt")
		switch {
		case r.Method == http.MethodGet && (path == "" || path == "/"):
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult><Name>b2bkt</Name><Prefix>old/</Prefix><KeyCount>1</KeyCount><MaxKeys>1000</MaxKeys><IsTruncated>false</IsTruncated>
<Contents><Key>old/1.txt</Key><LastModified>2024-01-01T00:00:00.000Z</LastModified><ETag>"e"</ETag><Size>4</Size><StorageClass>STANDARD</StorageClass></Contents>
</ListBucketResult>`))
		case r.Method == http.MethodGet && path == "/old/1.txt":
			f.getsSeen.Add(1)
			if f.truncGet.CompareAndSwap(true, false) {
				f.failedGet.Store(true)
				w.Header().Set("Content-Length", "100")
				w.Header().Set("Content-Type", "text/plain")
				_, _ = w.Write([]byte("data"))
				if hj, ok := w.(http.Hijacker); ok {
					if conn, _, err := hj.Hijack(); err == nil {
						_ = conn.Close()
					}
				}
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("data"))
		case r.Method == http.MethodPut:
			f.putSeen.Add(1)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete:
			if f.failDel.Load() {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newB2S3Store(t *testing.T, f *fakeS3Server) *s3Store {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	st, err := OpenS3(context.Background(), Config{
		Bucket:         "b2bkt",
		Region:         "us-east-1",
		Endpoint:       f.srv.URL,
		ForcePathStyle: true,
		SignedURLTTL:   0,
	})
	require.NoError(t, err)
	s, ok := st.(*s3Store)
	require.True(t, ok)
	return s
}

func TestB2S3StoreRenamePrefixCopyError(t *testing.T) {
	f := newFakeS3Server(t)
	s := newB2S3Store(t, f)
	f.truncGet.Store(true)

	err := s.RenamePrefix(context.Background(), "old", "new")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to copy object")
	assert.True(t, f.failedGet.Load(), "truncated GET should have been served")
	assert.Equal(t, int32(1), f.putSeen.Load())
}

func TestB2S3StoreRenamePrefixDeleteError(t *testing.T) {
	f := newFakeS3Server(t)
	s := newB2S3Store(t, f)
	f.failDel.Store(true)

	err := s.RenamePrefix(context.Background(), "old", "new")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete old object")
	assert.Equal(t, int32(1), f.putSeen.Load())
}
