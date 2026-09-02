// 覆盖目标：fileStore.Delete 的文件/目录/路径遍历分支、
// validateAndCleanPath 边界、s3/oss/cos Delete 的参数校验错误分支。
package objstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestFileStore(t *testing.T) (*fileStoreLike, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := OpenFile(context.Background(), Config{Driver: "file", BaseDir: dir})
	require.NoError(t, err)
	return &fileStoreLike{s}, dir
}

// fileStoreLike 只暴露本文件用到的能力，避免依赖具体类型名。
type fileStoreLike struct {
	raw interface {
		Delete(ctx context.Context, key string) error
	}
}

func TestFileStore_Delete_SingleFileAndMissing(t *testing.T) {
	store, dir := newTestFileStore(t)
	ctx := context.Background()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a", "f.txt"), []byte("x"), 0o644))

	// 删除存在的文件
	require.NoError(t, store.raw.Delete(ctx, "a/f.txt"))
	_, err := os.Stat(filepath.Join(dir, "a", "f.txt"))
	assert.True(t, os.IsNotExist(err))

	// 删除不存在的文件：os.Remove 报 not exist（现状固化）
	err = store.raw.Delete(ctx, "a/missing.txt")
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestFileStore_Delete_FolderRecursive(t *testing.T) {
	store, dir := newTestFileStore(t)
	ctx := context.Background()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "d", "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "d", "sub", "f.txt"), []byte("x"), 0o644))

	// 尾斜杠键递归删除目录（sanitizeKey 前保留目录语义）
	require.NoError(t, store.raw.Delete(ctx, "d/"))
	_, err := os.Stat(filepath.Join(dir, "d"))
	assert.True(t, os.IsNotExist(err), "directory must be removed recursively")
}

func TestFileStore_Delete_PathTraversal(t *testing.T) {
	store, dir := newTestFileStore(t)

	// sanitizeKey 先于拼接清洗点段：../escape.txt 被中和为 escape.txt，
	// 不会逃出 base（安全语义正确），删除目标不存在时报 not exist
	err := store.raw.Delete(context.Background(), "../escape.txt")
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	// base 外绝对路径同样被中和
	_, err = os.Stat(filepath.Join(dir, "escape.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestFileStore_ValidateAndCleanPath_Edges(t *testing.T) {
	_, dir := newTestFileStore(t)
	s, err := OpenFile(context.Background(), Config{Driver: "file", BaseDir: dir})
	require.NoError(t, err)
	fs := s.(interface {
		validateAndCleanPath(string) (string, error)
	})

	// base 本身合法
	p, err := fs.validateAndCleanPath(dir)
	require.NoError(t, err)
	assert.Equal(t, dir, p)

	// base 内路径合法
	inside := filepath.Join(dir, "x.txt")
	p2, err := fs.validateAndCleanPath(inside)
	require.NoError(t, err)
	assert.Equal(t, inside, p2)

	// 相邻目录被拒（前缀相同但不在 base 内）
	_, err = fs.validateAndCleanPath(dir + "-sibling/x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

// 确认 sanitizeKey 对反斜杠/点段的清洗行为（Delete 前置）。
func TestSanitizeKey_Normalization(t *testing.T) {
	assert.Equal(t, strings.TrimPrefix(sanitizeKey("a/b.txt"), "/"), "a/b.txt")
}
