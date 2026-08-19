package objstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFileStore(t *testing.T) Store {
	t.Helper()
	dir := t.TempDir()
	st, err := OpenFile(context.Background(), Config{BaseDir: dir})
	require.NoError(t, err)
	return st
}

func TestFileStore_ValidatePath(t *testing.T) {
	dir := t.TempDir()
	s := &fileStore{base: dir}

	require.NoError(t, s.validatePath(filepath.Join(dir, "a/b.txt")))
	require.NoError(t, s.validatePath(dir))
	require.Error(t, s.validatePath(filepath.Dir(dir)))
	require.Error(t, s.validatePath("/etc/passwd"))
}

func TestFileStore_ValidateAndCleanPathTraversal(t *testing.T) {
	dir := t.TempDir()
	s := &fileStore{base: dir}

	clean, err := s.validateAndCleanPath(filepath.Join(dir, "sub/../x.txt"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "x.txt"), clean)

	_, err = s.validateAndCleanPath(filepath.Join(dir, "..", "escape.txt"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestOpenFile_MkdirAllError(t *testing.T) {
	parent := t.TempDir()
	blocker := filepath.Join(parent, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("file"), 0o644))

	// blocker 是文件，其下的子目录无法创建
	_, err := OpenFile(context.Background(), Config{BaseDir: filepath.Join(blocker, "sub")})
	require.Error(t, err)
}

func TestFileStore_PutErrors(t *testing.T) {
	t.Run("create fails on existing directory", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "a"), 0o755))
		s := &fileStore{base: dir}

		err := s.Put(context.Background(), "a", strings.NewReader("x"), 1, "")
		require.Error(t, err)
	})
	t.Run("mkdir fails on existing file parent", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "a"), []byte("f"), 0o644))
		s := &fileStore{base: dir}

		err := s.Put(context.Background(), "a/b.txt", strings.NewReader("x"), 1, "")
		require.Error(t, err)
	})
	t.Run("reader error", func(t *testing.T) {
		dir := t.TempDir()
		s := &fileStore{base: dir}

		err := s.Put(context.Background(), "bad.txt", &failingReadSeeker{err: errors.New("read boom")}, 1, "")
		require.Error(t, err)
	})
}

// TestFileStore_DeleteFolderKeySanitized 文档化当前行为：sanitizeKey 去掉尾部斜杠后，
// Delete("dir/") 退化为删除单个路径 "dir"（RemoveAll 分支不可达）。
// 空目录可被删除；非空目录会报 "directory not empty"。
func TestFileStore_DeleteFolderKeySanitized(t *testing.T) {
	st := newFileStore(t)
	ctx := context.Background()

	require.NoError(t, st.CreatePrefix(ctx, "emptydir"))
	require.NoError(t, st.Delete(ctx, "emptydir/"))

	require.NoError(t, st.Put(ctx, "dir/a.txt", strings.NewReader("1"), 1, ""))
	require.NoError(t, st.Put(ctx, "dir/sub/b.txt", strings.NewReader("2"), 1, ""))
	err := st.Delete(ctx, "dir/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory not empty")
}

func TestFileStore_DeleteNonexistent(t *testing.T) {
	st := newFileStore(t)
	err := st.Delete(context.Background(), "missing.txt")
	require.Error(t, err)
}

func TestFileStore_ListWalkError(t *testing.T) {
	st := newFileStore(t)

	_, err := st.List(context.Background(), "nonexistent-dir/", "", "", 0)
	require.Error(t, err)
}

func TestFileStore_SignedURLDeleteUnsupported(t *testing.T) {
	st := newFileStore(t)
	_, err := st.SignedURL(context.Background(), "k.txt", "DELETE", 0)
	require.Error(t, err)
}
