package objstore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/memblob"
)

// newMemS3Store 构造基于 memblob 内存桶的 s3Store，用于离线测试 S3 驱动逻辑。
func newMemS3Store(t *testing.T) *s3Store {
	t.Helper()
	bk, err := blob.OpenBucket(context.Background(), "mem://")
	require.NoError(t, err)
	t.Cleanup(func() { _ = bk.Close() })
	return &s3Store{bk: bk, ttl: 15 * time.Minute}
}

func TestS3Store_PutNestedCreatesDirMarkers(t *testing.T) {
	ctx := context.Background()
	s := newMemS3Store(t)

	require.NoError(t, s.Put(ctx, "a/b/c.txt", strings.NewReader("hello"), 5, "text/plain"))

	res, err := s.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	keys := make([]string, 0, len(res.Objects))
	for _, o := range res.Objects {
		keys = append(keys, o.Key)
	}
	assert.ElementsMatch(t, []string{"a/", "a/b/", "a/b/c.txt"}, keys)

	var target *ObjectInfo
	for i := range res.Objects {
		if res.Objects[i].Key == "a/b/c.txt" {
			target = &res.Objects[i]
		}
	}
	require.NotNil(t, target)
	assert.Equal(t, int64(5), target.Size)
}

func TestS3Store_PutRootKey(t *testing.T) {
	ctx := context.Background()
	s := newMemS3Store(t)

	require.NoError(t, s.Put(ctx, "root.txt", strings.NewReader("data"), 4, ""))
	res, err := s.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	require.Len(t, res.Objects, 1)
	assert.Equal(t, "root.txt", res.Objects[0].Key)
}

func TestS3Store_SignedURL_PublicURL(t *testing.T) {
	ctx := context.Background()
	s := newMemS3Store(t)
	s.publicURL = "https://cdn.example.com/"

	u, err := s.SignedURL(ctx, "path/file.txt", "GET", 0)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/path/file.txt", u)
}

func TestS3Store_SignedURL_NotSupportedByMemblob(t *testing.T) {
	ctx := context.Background()
	s := newMemS3Store(t)

	_, err := s.SignedURL(ctx, "file.txt", "GET", time.Minute)
	require.Error(t, err)
}

func TestS3Store_DeleteSingleObject(t *testing.T) {
	ctx := context.Background()
	s := newMemS3Store(t)

	require.NoError(t, s.Put(ctx, "file.txt", strings.NewReader("x"), 1, ""))
	require.NoError(t, s.Delete(ctx, "file.txt"))

	res, err := s.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	assert.Empty(t, res.Objects)
}

// TestS3Store_DeleteFolderKeySanitized 文档化当前行为：sanitizeKey 会去掉尾部斜杠，
// Delete("dir/") 实际删除的是对象 "dir"（非前缀递归删除）。
// 尾斜杠键走前缀递归删除（历史上被 sanitizeKey 吃掉尾斜杠后报 NotFound，已修复）。
func TestS3Store_DeleteFolderKeySanitized(t *testing.T) {
	ctx := context.Background()
	s := newMemS3Store(t)

	require.NoError(t, s.CreatePrefix(ctx, "dir"))
	require.NoError(t, s.Put(ctx, "dir/f1.txt", strings.NewReader("1"), 1, ""))

	require.NoError(t, s.Delete(ctx, "dir/"))

	res, err := s.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	for _, o := range res.Objects {
		assert.NotContains(t, o.Key, "dir/", "prefix must be deleted, got %s", o.Key)
	}
}

func TestS3Store_ListDelimiter(t *testing.T) {
	ctx := context.Background()
	s := newMemS3Store(t)
	require.NoError(t, s.Put(ctx, "a/one.txt", strings.NewReader("1"), 1, ""))
	require.NoError(t, s.Put(ctx, "a/two.txt", strings.NewReader("2"), 1, ""))
	require.NoError(t, s.Put(ctx, "top.txt", strings.NewReader("3"), 1, ""))

	res, err := s.List(ctx, "", "", "/", 0)
	require.NoError(t, err)
	assert.Contains(t, res.Prefixes, "a/")
	assert.Len(t, res.Objects, 1)
	assert.Equal(t, "top.txt", res.Objects[0].Key)
}

func TestS3Store_ListMarkerAndLimit(t *testing.T) {
	ctx := context.Background()
	s := newMemS3Store(t)
	for _, k := range []string{"f1.txt", "f2.txt", "f3.txt"} {
		require.NoError(t, s.Put(ctx, k, strings.NewReader("x"), 1, ""))
	}

	res, err := s.List(ctx, "", "f1.txt", "", 1)
	require.NoError(t, err)
	require.Len(t, res.Objects, 1)
	assert.Equal(t, "f2.txt", res.Objects[0].Key)
	assert.True(t, res.IsTruncated)
	assert.Equal(t, "f2.txt", res.NextMarker)
}

func TestS3Store_RenamePrefix(t *testing.T) {
	ctx := context.Background()
	s := newMemS3Store(t)
	// 直接写入裸对象（不经 Put 的目录标记逻辑），验证重命名搬运本身
	for _, k := range []string{"old/1.txt", "old/2.txt"} {
		w, err := s.bk.NewWriter(ctx, k, nil)
		require.NoError(t, err)
		_, err = w.Write([]byte("data"))
		require.NoError(t, err)
		require.NoError(t, w.Close())
	}

	require.NoError(t, s.RenamePrefix(ctx, "old", "new"))

	res, err := s.List(ctx, "new/", "", "", 0)
	require.NoError(t, err)
	keys := make([]string, 0, len(res.Objects))
	for _, o := range res.Objects {
		keys = append(keys, o.Key)
	}
	assert.ElementsMatch(t, []string{"new/1.txt", "new/2.txt"}, keys)

	oldRes, err := s.List(ctx, "old/", "", "", 0)
	require.NoError(t, err)
	assert.Empty(t, oldRes.Objects)
}

func TestOpenS3_DefaultTTLAndFastFailure(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")

	st, err := OpenS3(context.Background(), Config{
		Bucket:   "bkt",
		Region:   "us-east-1",
		Endpoint: "http://127.0.0.1:1", // 不可达端点，触发快速失败
	})
	require.NoError(t, err)
	s, ok := st.(*s3Store)
	require.True(t, ok)
	assert.Equal(t, 15*time.Minute, s.ttl)

	err = s.Put(context.Background(), "k", strings.NewReader("v"), 1, "")
	require.Error(t, err)
}

func TestOpenS3_TTLFromConfig(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")

	st, err := OpenS3(context.Background(), Config{
		Bucket:       "bkt",
		Region:       "us-east-1",
		Endpoint:     "http://127.0.0.1:1",
		SignedURLTTL: 5 * time.Minute,
	})
	require.NoError(t, err)
	s, ok := st.(*s3Store)
	require.True(t, ok)
	assert.Equal(t, 5*time.Minute, s.ttl)
}

func TestS3Store_PutReaderError(t *testing.T) {
	ctx := context.Background()
	s := newMemS3Store(t)

	err := s.Put(ctx, "bad.txt", &failingReadSeeker{err: errors.New("boom")}, 1, "")
	require.Error(t, err)
}

// failingReadSeeker 实现总是失败的 ReadSeeker，用于覆盖读取错误分支。
type failingReadSeeker struct {
	err error
}

func (f *failingReadSeeker) Read([]byte) (int, error)       { return 0, f.err }
func (f *failingReadSeeker) Seek(int64, int) (int64, error) { return 0, nil }
