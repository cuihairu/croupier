// 覆盖目标：s3Store 错误分支（closed bucket / fileblob 权限）、OpenS3 打开失败、
// fileStore base="/" 路径遍历分支与 List Rel 错误、OpenOSS 打开失败、
// OSS List delimiter 选项、COS SignedURL 空 key 与默认 TTL 分支。
package objstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/memblob"
)

// newClosedS3StoreV9 返回底层 bucket 已关闭的 s3Store，所有驱动操作都会失败。
func newClosedS3StoreV9(t *testing.T) *s3Store {
	t.Helper()
	bk, err := blob.OpenBucket(context.Background(), "mem://")
	require.NoError(t, err)
	require.NoError(t, bk.Close())
	return &s3Store{bk: bk, ttl: 15 * time.Minute}
}

func TestS3StoreClosedBucketErrorsV9(t *testing.T) {
	ctx := context.Background()
	s := newClosedS3StoreV9(t)

	require.Error(t, s.Put(ctx, "a.txt", strings.NewReader("x"), 1, ""))
	_, err := s.List(ctx, "", "", "", 0)
	require.Error(t, err)
	require.Error(t, s.Delete(ctx, "dir/"))
	require.Error(t, s.CreatePrefix(ctx, "p"))
	require.Error(t, s.RenamePrefix(ctx, "old", "new"))

	// expiry<=0 时回退到默认 TTL 分支（memblob 不支持签名，最终仍报错）
	_, err = s.SignedURL(ctx, "k.txt", "GET", 0)
	require.Error(t, err)
}

func TestOpenS3InvalidBucketNameV9(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")

	_, err := OpenS3(context.Background(), Config{
		Bucket:   "bad bucket",
		Region:   "us-east-1",
		Endpoint: "http://127.0.0.1:1",
	})
	require.Error(t, err)
}

// newFileBlobS3StoreV9 构造基于 fileblob 的 s3Store，用于权限类错误注入。
func newFileBlobS3StoreV9(t *testing.T) (*s3Store, string) {
	t.Helper()
	root := t.TempDir()
	bk, err := blob.OpenBucket(context.Background(), "file://"+root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = bk.Close() })
	return &s3Store{bk: bk, ttl: 15 * time.Minute}, root
}

func TestS3StoreDeletePrefixDeleteFailureV9(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-based test requires non-root user")
	}
	s, root := newFileBlobS3StoreV9(t)
	oldDir := filepath.Join(root, "old")
	require.NoError(t, os.MkdirAll(oldDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(oldDir, "1.txt"), []byte("data"), 0o644))

	// List 可读但目录不可写：前缀删除时 bk.Delete 失败
	require.NoError(t, os.Chmod(oldDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(oldDir, 0o755) })

	err := s.Delete(context.Background(), "old/")
	require.Error(t, err)
	require.Contains(t, err.Error(), "permission denied")
}

func TestS3StoreRenamePrefixReadFailureV9(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-based test requires non-root user")
	}
	s, root := newFileBlobS3StoreV9(t)
	src := filepath.Join(root, "old", "1.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(src), 0o755))
	require.NoError(t, os.WriteFile(src, []byte("data"), 0o644))

	// 源文件不可读：List 成功但 NewReader 失败
	require.NoError(t, os.Chmod(src, 0o000))
	t.Cleanup(func() { _ = os.Chmod(src, 0o644) })

	err := s.RenamePrefix(context.Background(), "old", "new")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read object")
}

func TestS3StoreRenamePrefixWriteFailureV9(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-based test requires non-root user")
	}
	s, root := newFileBlobS3StoreV9(t)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "old"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "old", "1.txt"), []byte("data"), 0o644))

	// 根目录不可写：无法为新前缀创建目录，NewWriter 失败
	require.NoError(t, os.Chmod(root, 0o555))
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	err := s.RenamePrefix(context.Background(), "old", "new")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create writer")
}

func TestS3StoreRenamePrefixCloseFailureV9(t *testing.T) {
	s, root := newFileBlobS3StoreV9(t)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "old"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "old", "1.txt"), []byte("data"), 0o644))

	// 已取消的 ctx：fileblob Writer.Close 检查 ctx 并报错
	cctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.RenamePrefix(cctx, "old", "new")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to write object")
}

func TestFileStoreRootBaseTraversalV9(t *testing.T) {
	st, err := OpenFile(context.Background(), Config{BaseDir: "/"})
	require.NoError(t, err)
	ctx := context.Background()

	require.Error(t, st.Put(ctx, "zz-put.txt", strings.NewReader("x"), 1, ""))
	require.Error(t, st.Delete(ctx, "zz-file.txt"))
	require.Error(t, st.Delete(ctx, "zz-dir/"))
	_, err = st.List(ctx, "zz-prefix", "", "", 0)
	require.Error(t, err)
	require.Error(t, st.CreatePrefix(ctx, "zz-prefix"))

	// 旧前缀校验失败
	err = st.RenamePrefix(ctx, "zz-old", "zz-new")
	require.Error(t, err)
	require.Contains(t, err.Error(), "path traversal detected")

	// 旧前缀为空（等于 base 本身）通过校验，新前缀校验失败
	err = st.RenamePrefix(ctx, "", "zz-new")
	require.Error(t, err)
	require.Contains(t, err.Error(), "path traversal detected")
}

func TestFileStoreListRelativeBaseRelErrorV9(t *testing.T) {
	rel := "filestore_relbase_v9"
	require.NoError(t, os.MkdirAll(rel, 0o755))
	t.Cleanup(func() { _ = os.RemoveAll(rel) })
	require.NoError(t, os.WriteFile(filepath.Join(rel, "f.txt"), []byte("x"), 0o644))

	// 相对 base + 绝对 walk 路径：filepath.Rel 失败
	s := &fileStore{base: rel}
	_, err := s.List(context.Background(), "", "", "", 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Rel:")
}

func TestOpenOSSInvalidEndpointAndBucketV9(t *testing.T) {
	_, err := OpenOSS(context.Background(), Config{
		Endpoint:  "http://a b",
		AccessKey: "ak",
		SecretKey: "sk",
		Bucket:    "bkt",
	})
	require.Error(t, err)

	_, err = OpenOSS(context.Background(), Config{
		Endpoint:  "http://oss.example.com",
		AccessKey: "ak",
		SecretKey: "sk",
		Bucket:    "UP",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "bucket name")
}

func TestOSSStoreListDelimiterOptionV9(t *testing.T) {
	st, f := newOSSStore(t)
	f.mu.Lock()
	f.objects["dir/a.txt"] = ""
	f.mu.Unlock()

	res, err := st.List(context.Background(), "dir/", "", "/", 10)
	require.NoError(t, err)
	require.Len(t, res.Objects, 1)
	require.Contains(t, res.Prefixes, "dir/sub/")
}

func TestCOSStoreSignedURLEmptyKeyV9(t *testing.T) {
	f := newFakeCOSServer(t)
	st, err := OpenCOS(context.Background(), Config{
		Endpoint:  f.srv.URL,
		AccessKey: "ak",
		SecretKey: "sk",
		Bucket:    "test-bucket",
	})
	require.NoError(t, err)

	// expiry<=0 回退默认 TTL，且空 key 使 GetPresignedURL 报错
	_, err = st.SignedURL(context.Background(), "", "GET", 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "object key is empty")
}
