package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileAtomicParentPathIsFile(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	err := WriteFileAtomic(filepath.Join(blocker, "state.json"), []byte("x"), 0o644)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestWriteFileAtomicBaseNameTooLong(t *testing.T) {
	dir := t.TempDir()
	longBase := strings.Repeat("a", 300)
	err := WriteFileAtomic(filepath.Join(dir, longBase), []byte("x"), 0o644)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file name too long")

	// No temp file should remain in the directory.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestWriteFileAtomicWriteError(t *testing.T) {
	var orig syscall.Rlimit
	require.NoError(t, syscall.Getrlimit(syscall.RLIMIT_FSIZE, &orig))

	zero := orig
	zero.Cur = 0
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &zero); err != nil {
		t.Skipf("cannot set RLIMIT_FSIZE: %v", err)
	}
	defer func() {
		_ = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &orig)
	}()

	dir := t.TempDir()
	err := WriteFileAtomic(filepath.Join(dir, "state.json"), []byte("payload"), 0o644)
	require.Error(t, err)

	// Restore limit before doing any further I/O assertions.
	require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_FSIZE, &orig))

	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)
	assert.Empty(t, entries, "failed write must not leave the final file")
}

func TestWriteFileAtomicOverwritesExistingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(path, []byte("old-content"), 0o644))

	require.NoError(t, WriteFileAtomic(path, []byte("new-content"), 0o600))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new-content", string(data))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestWriteFileAtomicEmptyData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	require.NoError(t, WriteFileAtomic(path, nil, 0o644))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}
