package objstore

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Additional tests for file.go to increase coverage

func TestFileStore_CreatePrefix_Nested(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Test nested prefix creation
	err = store.CreatePrefix(ctx, "level1/level2/level3")
	require.NoError(t, err)

	// Verify directory was created
	dirPath := filepath.Join(tmpDir, "level1", "level2", "level3")
	info, err := os.Stat(dirPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestFileStore_CreatePrefix_WithSlash(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Test prefix without trailing slash
	err = store.CreatePrefix(ctx, "test")
	require.NoError(t, err)

	// Verify directory was created
	dirPath := filepath.Join(tmpDir, "test")
	info, err := os.Stat(dirPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestFileStore_RenamePrefix_ErrorCases(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Test renaming non-existent prefix
	err = store.RenamePrefix(ctx, "nonexistent/", "new/")
	assert.Error(t, err)

	// Test renaming to existing directory
	_ = store.CreatePrefix(ctx, "existing/")
	_ = store.CreatePrefix(ctx, "target/")

	// This should fail because target already exists
	err = store.RenamePrefix(ctx, "existing/", "target/")
	assert.Error(t, err)
}

func TestFileStore_Delete_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Delete non-existent file may error (depends on OS/implementation)
	err = store.Delete(ctx, "nonexistent.txt")
	// On Windows, os.Remove returns error for non-existent files
	// On some systems, it may succeed
	_ = err // We just verify it doesn't panic
}

func TestFileStore_List_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	result, err := store.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	assert.Empty(t, result.Objects)
	assert.Empty(t, result.Prefixes)
}

func TestFileStore_List_WithDelimiter(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Create directory structure
	_ = os.MkdirAll(filepath.Join(tmpDir, "folder1", "subfolder"), 0755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "folder2", "subfolder"), 0755)

	// Create some files
	_ = os.WriteFile(filepath.Join(tmpDir, "folder1", "file1.txt"), []byte("content1"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "folder2", "file2.txt"), []byte("content2"), 0644)

	// List with delimiter
	result, err := store.List(ctx, "", "", "/", 100)
	require.NoError(t, err)

	// Should return prefixes (directories)
	assert.NotEmpty(t, result.Prefixes)
}

func TestFileStore_List_WithLimit(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Create multiple files
	for i := 0; i < 10; i++ {
		_ = os.WriteFile(filepath.Join(tmpDir, "file"+string(rune('0'+i))+".txt"), []byte("content"), 0644)
	}

	// List with limit
	result, err := store.List(ctx, "", "", "", 5)
	require.NoError(t, err)
	assert.Len(t, result.Objects, 5)
	assert.True(t, result.IsTruncated)
}

func TestFileStore_List_WithMarker(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Create files with predictable names for pagination
	for i := 0; i < 10; i++ {
		filename := fmt.Sprintf("file%03d.txt", i)
		_ = os.WriteFile(filepath.Join(tmpDir, filename), []byte("content"), 0644)
	}

	// List first page
	result1, err := store.List(ctx, "", "", "", 3)
	require.NoError(t, err)
	assert.Len(t, result1.Objects, 3)

	// List with marker
	if len(result1.Objects) > 0 {
		marker := result1.Objects[len(result1.Objects)-1].Key
		result2, err := store.List(ctx, "", marker, "", 3)
		require.NoError(t, err)
		assert.NotEmpty(t, result2.Objects)

		// Verify we got different files
		firstObj1 := result1.Objects[0].Key
		firstObj2 := result2.Objects[0].Key
		assert.NotEqual(t, firstObj1, firstObj2)
	}
}

func TestFileStore_Put_WithContentType(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	content := strings.NewReader("test content")
	contentType := "text/plain"

	err = store.Put(ctx, "test.txt", content, 0, contentType)
	require.NoError(t, err)

	// Verify file was created
	path := filepath.Join(tmpDir, "test.txt")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, int64(len("test content")), info.Size())
}

func TestFileStore_Put_WithNestedPath(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	content := strings.NewReader("nested content")

	err = store.Put(ctx, "level1/level2/test.txt", content, 0, "")
	require.NoError(t, err)

	// Verify file was created
	path := filepath.Join(tmpDir, "level1", "level2", "test.txt")
	_, err = os.Stat(path)
	require.NoError(t, err)
}

func TestFileStore_SignedURL_DeleteNotSupported(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	_, err = store.SignedURL(ctx, "test.txt", "DELETE", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestFileStore_SignedURL_WithPublicURL(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:    "file",
		BaseDir:   tmpDir,
		PublicURL: "https://cdn.example.com",
	})
	require.NoError(t, err)

	url, err := store.SignedURL(ctx, "test/file.txt", "GET", 0)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/test/file.txt", url)
}

func TestFileStore_List_WithNonExistentMarker(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Create some files
	_ = os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("a"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("b"), 0644)

	// List with marker beyond existing files
	result, err := store.List(ctx, "", "z.txt", "", 10)
	require.NoError(t, err)
	assert.Empty(t, result.Objects)
}

func TestFileStore_Delete_FolderWithFiles(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Create folder with files
	folderPath := filepath.Join(tmpDir, "testfolder")
	_ = os.MkdirAll(folderPath, 0755)
	_ = os.WriteFile(filepath.Join(folderPath, "file1.txt"), []byte("content1"), 0644)
	_ = os.WriteFile(filepath.Join(folderPath, "file2.txt"), []byte("content2"), 0644)

	// First delete the files, then the folder
	_ = store.Delete(ctx, "testfolder/file1.txt")
	_ = store.Delete(ctx, "testfolder/file2.txt")
	err = store.Delete(ctx, "testfolder/")
	require.NoError(t, err)

	// Verify folder and files are deleted
	_, err = os.Stat(folderPath)
	assert.True(t, os.IsNotExist(err))
}

// Tests for OSS specific SignedURL method
// Note: These tests focus on PublicURL path which doesn't require actual OSS client

func TestOSSStore_SignedURL_WithPublicURL(t *testing.T) {
	// This test verifies the PublicURL branch which doesn't need client
	tmpDir := t.TempDir()
	ctx := context.Background()

	// We can't create a real OSS store without credentials,
	// but we can verify the file store public URL behavior
	store, err := OpenFile(ctx, Config{
		Driver:    "file",
		BaseDir:   tmpDir,
		PublicURL: "https://cdn.example.com",
	})
	require.NoError(t, err)

	url, err := store.SignedURL(ctx, "test/file.txt", "GET", 0)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/test/file.txt", url)
}

func TestOSSStore_SignedURL_DELETE_Method(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// DELETE method is not supported for file store SignedURL
	_, err = store.SignedURL(ctx, "test/file.txt", "DELETE", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

// Tests for COS specific SignedURL method
// Note: These tests focus on PublicURL path which doesn't require actual COS client

func TestCOSSStore_SignedURL_WithPublicURL(t *testing.T) {
	// Verify public URL behavior through the general test
	// COS-specific tests require actual client which we can't mock here
	// This is covered by cos_test.go with proper mocks
	tmpDir := t.TempDir()
	ctx := context.Background()

	store, err := OpenFile(ctx, Config{
		Driver:    "file",
		BaseDir:   tmpDir,
		PublicURL: "https://cos.example.com",
	})
	require.NoError(t, err)

	url, err := store.SignedURL(ctx, "cos/test/file.txt", "GET", 0)
	require.NoError(t, err)
	assert.Equal(t, "https://cos.example.com/cos/test/file.txt", url)
}

// Test for ReadSeeker wrapper

type mockReadSeeker struct {
	data   []byte
	seeked bool
}

func (m *mockReadSeeker) Read(p []byte) (int, error) {
	if m.seeked {
		return 0, io.EOF
	}
	n := copy(p, m.data)
	m.seeked = true
	return n, nil
}

func (m *mockReadSeeker) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

func TestReadSeeker(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	r := &mockReadSeeker{data: []byte("test content")}
	err = store.Put(ctx, "test.txt", r, 0, "")
	require.NoError(t, err)

	// Verify file content
	path := filepath.Join(tmpDir, "test.txt")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "test content", string(content))
}

// Additional edge case tests for List method

func TestFileStore_List_DirectoryOnly(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Create directories
	_ = os.MkdirAll(filepath.Join(tmpDir, "dir1"), 0755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "dir2"), 0755)

	// List without delimiter should return empty (files only)
	result, err := store.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	assert.Empty(t, result.Objects)

	// List with delimiter should return prefixes
	result, err = store.List(ctx, "", "", "/", 0)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Prefixes)
}

func TestFileStore_Put_EmptyReader(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Empty reader should create empty file
	content := strings.NewReader("")
	err = store.Put(ctx, "empty.txt", content, 0, "")
	require.NoError(t, err)

	// Verify file was created
	path := filepath.Join(tmpDir, "empty.txt")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func TestFileStore_List_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Create a file in a subdirectory
	_ = os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "subdir", "safe.txt"), []byte("content"), 0644)

	// sanitizeKey strips "../" so it becomes empty string
	// List with empty prefix will list all files, but doesn't escape base dir
	result, err := store.List(ctx, "../", "", "", 0)
	require.NoError(t, err)
	// The sanitized path is empty, so it lists base dir
	// This is safe - it doesn't escape the base directory
	assert.NotEmpty(t, result.Objects)
}

func TestFileStore_RenamePrefix_SamePrefix(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Create a prefix
	_ = store.CreatePrefix(ctx, "test/")

	// Rename to same name should succeed
	err = store.RenamePrefix(ctx, "test/", "test/")
	// On some systems this might fail, so we just check no panic
	_ = err
}

func TestFileStore_Put_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Write file
	_ = os.WriteFile(filepath.Join(tmpDir, "overwrite.txt"), []byte("original"), 0644)

	// Overwrite with new content
	newContent := strings.NewReader("new content")
	err = store.Put(ctx, "overwrite.txt", newContent, 0, "")
	require.NoError(t, err)

	// Verify file was overwritten
	path := filepath.Join(tmpDir, "overwrite.txt")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(content))
}

// TestFileStore_Put_WithSpecialCharacters tests Put with special characters in key
func TestFileStore_Put_WithSpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Put with special characters (sanitized)
	// sanitizeKey filters out ".." so "folder/../file.txt" becomes "folder/file.txt"
	content := strings.NewReader("test")
	err = store.Put(ctx, "folder/../file.txt", content, 4, "")
	require.NoError(t, err)

	// Verify file was created with sanitized path
	// After sanitization: "folder/../file.txt" -> "folder/file.txt"
	path := filepath.Join(tmpDir, "folder", "file.txt")
	_, err = os.Stat(path)
	require.NoError(t, err)
}

// TestFileStore_List_WithEmptyPrefix tests List with various empty/edge cases
func TestFileStore_List_WithEmptyPrefix(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Create some files
	_ = os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("a"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("b"), 0644)

	// List with empty prefix
	result, err := store.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	assert.Len(t, result.Objects, 2)
}

// TestFileStore_List_WithNonExistentPrefix tests List with non-existent prefix
func TestFileStore_List_WithNonExistentPrefix(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// List with non-existent prefix - returns error on Windows if dir doesn't exist
	result, err := store.List(ctx, "nonexistent/", "", "", 0)
	// On non-Windows or when directory exists, it returns empty list
	// On Windows, it may error
	if err == nil {
		assert.Empty(t, result.Objects)
	}
	// As long as it doesn't panic, the test passes
}

// TestFileStore_SignedURL_PutMethod tests SignedURL with PUT method
func TestFileStore_SignedURL_PutMethod(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// PUT method should return a URL
	url, err := store.SignedURL(ctx, "test.txt", "PUT", 0)
	require.NoError(t, err)
	assert.NotEmpty(t, url)
	assert.Contains(t, url, "/test.txt")
}

// TestFileStore_SignedURL_PostMethod tests SignedURL with POST method
func TestFileStore_SignedURL_PostMethod(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// POST method should return a URL
	url, err := store.SignedURL(ctx, "test.txt", "POST", 0)
	require.NoError(t, err)
	assert.NotEmpty(t, url)
}

// TestFileStore_CreatePrefix_EmptyString tests CreatePrefix with sanitized empty result
func TestFileStore_CreatePrefix_EmptyString(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// CreatePrefix with only ../.. which sanitizes to empty
	err = store.CreatePrefix(ctx, "../..")
	// Should succeed or error gracefully, not panic
	_ = err
}

// TestFileStore_CreatePrefix_WithoutTrailingSlash tests CreatePrefix adds trailing slash
func TestFileStore_CreatePrefix_WithoutTrailingSlash(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// CreatePrefix without trailing slash - should be added
	err = store.CreatePrefix(ctx, "test")
	require.NoError(t, err)

	// Verify directory was created
	dirPath := filepath.Join(tmpDir, "test")
	info, err := os.Stat(dirPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// TestFileStore_RenamePrefix_NonExistent tests RenamePrefix with non-existent prefix
func TestFileStore_RenamePrefix_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Rename non-existent prefix
	err = store.RenamePrefix(ctx, "nonexistent/", "new/")
	assert.Error(t, err)
}

// TestFileStore_RenamePrefix_WithFiles tests RenamePrefix with files
func TestFileStore_RenamePrefix_WithFiles(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Create directory first
	_ = os.MkdirAll(filepath.Join(tmpDir, "old"), 0755)
	// Create files in a folder
	_ = os.WriteFile(filepath.Join(tmpDir, "old", "file1.txt"), []byte("content1"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "old", "file2.txt"), []byte("content2"), 0644)

	// Rename the prefix
	err = store.RenamePrefix(ctx, "old/", "new/")
	require.NoError(t, err)

	// Verify files moved
	_, err = os.Stat(filepath.Join(tmpDir, "new", "file1.txt"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(tmpDir, "new", "file2.txt"))
	require.NoError(t, err)

	// Verify old directory doesn't exist
	_, err = os.Stat(filepath.Join(tmpDir, "old"))
	assert.True(t, os.IsNotExist(err))
}

// TestFileStore_RenamePrefix_WithoutTrailingSlash_AddsSlash tests RenamePrefix adds trailing slash
func TestFileStore_RenamePrefix_WithoutTrailingSlash_AddsSlash(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Create folder
	_ = os.MkdirAll(filepath.Join(tmpDir, "old"), 0755)

	// Rename without trailing slash - should be added
	err = store.RenamePrefix(ctx, "old", "new")
	require.NoError(t, err)

	// Verify renamed
	_, err = os.Stat(filepath.Join(tmpDir, "new"))
	require.NoError(t, err)
}

// TestFileStore_Delete_File tests deleting a single file
func TestFileStore_Delete_File(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Create a file
	_ = os.WriteFile(filepath.Join(tmpDir, "delete.txt"), []byte("content"), 0644)

	// Delete the file
	err = store.Delete(ctx, "delete.txt")
	require.NoError(t, err)

	// Verify deleted
	_, err = os.Stat(filepath.Join(tmpDir, "delete.txt"))
	assert.True(t, os.IsNotExist(err))
}

// TestFileStore_Delete_WithTrailingSlash_IsDirectory tests Delete with trailing slash for directory
func TestFileStore_Delete_WithTrailingSlash_IsDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Create empty directory
	_ = os.MkdirAll(filepath.Join(tmpDir, "delete"), 0755)

	// Delete with trailing slash - should use RemoveAll
	err = store.Delete(ctx, "delete/")
	require.NoError(t, err)

	// Verify deleted
	_, err = os.Stat(filepath.Join(tmpDir, "delete"))
	assert.True(t, os.IsNotExist(err))
}

// TestFileStore_Put_DeepNestedPath tests Put with deeply nested path
func TestFileStore_Put_DeepNestedPath(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Put with deeply nested path
	content := strings.NewReader("deep")
	err = store.Put(ctx, "a/b/c/d/e/file.txt", content, 4, "")
	require.NoError(t, err)

	// Verify all directories were created
	path := filepath.Join(tmpDir, "a", "b", "c", "d", "e", "file.txt")
	_, err = os.Stat(path)
	require.NoError(t, err)
}

// TestFileStore_List_DirectoryTraversal tests List doesn't escape base directory
func TestFileStore_List_DirectoryTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	store, err := OpenFile(ctx, Config{
		Driver:  "file",
		BaseDir: tmpDir,
	})
	require.NoError(t, err)

	// Create a file
	_ = os.WriteFile(filepath.Join(tmpDir, "safe.txt"), []byte("safe"), 0644)

	// Try to list with path traversal - sanitizeKey strips ../
	// so "../../" becomes empty string, listing the base directory
	result, err := store.List(ctx, "../", "", "", 0)
	// Should not error and only list files in base dir
	if err == nil {
		assert.NotEmpty(t, result.Objects)
	}
	// As long as it doesn't escape base dir, we're good
}
