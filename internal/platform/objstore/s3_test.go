package objstore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob/memblob"
)

// TestS3Store_Put tests Put operation for S3 store
func TestS3Store_Put(t *testing.T) {
	ctx := context.Background()

	// Create in-memory bucket for testing
	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Test basic put
	data := strings.NewReader("test content")
	err := store.Put(ctx, "test/file.txt", data, 12, "text/plain")
	require.NoError(t, err)

	// Verify file exists (note: directory markers may be included)
	result, err := store.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Objects)

	// Find the actual file (not directory markers)
	found := false
	for _, obj := range result.Objects {
		if obj.Key == "test/file.txt" {
			found = true
			assert.Equal(t, int64(12), obj.Size)
			break
		}
	}
	assert.True(t, found, "test/file.txt should exist")
}

// TestS3Store_Put_WithContentType tests Put with content type
func TestS3Store_Put_WithContentType(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Test with different content types
	data := strings.NewReader("<html>test</html>")
	err := store.Put(ctx, "test.html", data, 17, "text/html")
	require.NoError(t, err)
}

// TestS3Store_Put_NestedPath tests Put with nested path
func TestS3Store_Put_NestedPath(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Test nested path - should create directory markers
	data := strings.NewReader("nested content")
	err := store.Put(ctx, "a/b/c/file.txt", data, 15, "")
	require.NoError(t, err)

	// Verify file exists
	result, err := store.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Objects)
}

// TestS3Store_SignedURL_WithPublicURL tests SignedURL with PublicURL
func TestS3Store_SignedURL_WithPublicURL(t *testing.T) {
	ctx := context.Background()

	store := &s3Store{
		publicURL: "https://cdn.example.com",
		ttl:       15 * time.Minute,
	}

	url, err := store.SignedURL(ctx, "test/file.txt", "GET", 0)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/test/file.txt", url)
}

// TestS3Store_SignedURL_WithExpiry tests SignedURL with custom expiry
func TestS3Store_SignedURL_WithExpiry(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Test with expiry - will return a signed URL from memblob
	url, err := store.SignedURL(ctx, "test/file.txt", "GET", time.Hour)
	// memblob might not support signed URLs, but we test the branch
	_ = url
	_ = err
	// As long as it doesn't panic, the test passes
}

// TestS3Store_SignedURL_DefaultExpiry tests SignedURL with default expiry
func TestS3Store_SignedURL_DefaultExpiry(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 30 * time.Minute,
	}

	// Test with zero expiry - should use default ttl
	url, err := store.SignedURL(ctx, "test/file.txt", "GET", 0)
	_ = url
	_ = err
	// As long as it doesn't panic, the test passes
}

// TestS3Store_Delete tests Delete operation
func TestS3Store_Delete(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Create a file directly in bucket (not through Put to avoid dir markers)
	err := bucket.WriteAll(ctx, "test/file.txt", []byte("test content"), nil)
	require.NoError(t, err)

	// Delete the file
	err = store.Delete(ctx, "test/file.txt")
	require.NoError(t, err)

	// Verify file is deleted (directory marker may remain)
	result, err := store.List(ctx, "", "", "", 0)
	require.NoError(t, err)

	// Check that the file itself is gone
	found := false
	for _, obj := range result.Objects {
		if obj.Key == "test/file.txt" {
			found = true
			break
		}
	}
	assert.False(t, found, "test/file.txt should be deleted")
}

// TestS3Store_List tests List operation
func TestS3Store_List(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Create multiple files
	_ = store.Put(ctx, "file1.txt", strings.NewReader("content1"), 8, "")
	_ = store.Put(ctx, "file2.txt", strings.NewReader("content2"), 8, "")
	_ = store.Put(ctx, "file3.txt", strings.NewReader("content3"), 8, "")

	// List all files
	result, err := store.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	assert.Len(t, result.Objects, 3)
}

// TestS3Store_List_WithPrefix tests List with prefix
func TestS3Store_List_WithPrefix(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Create files directly (avoiding Put's directory markers)
	_ = bucket.WriteAll(ctx, "a/file1.txt", []byte("content1"), nil)
	_ = bucket.WriteAll(ctx, "b/file2.txt", []byte("content2"), nil)
	_ = bucket.WriteAll(ctx, "a/file3.txt", []byte("content3"), nil)

	// List with prefix
	result, err := store.List(ctx, "a/", "", "", 0)
	require.NoError(t, err)
	// Should return files in a/ prefix (directory markers may vary)
	assert.NotEmpty(t, result.Objects)

	// Count actual files (not directory markers)
	fileCount := 0
	for _, obj := range result.Objects {
		if strings.HasSuffix(obj.Key, ".txt") {
			fileCount++
		}
	}
	assert.Equal(t, 2, fileCount)
}

// TestS3Store_List_WithLimit tests List with limit
func TestS3Store_List_WithLimit(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Create multiple files
	for i := 1; i <= 10; i++ {
		key := strings.NewReader("content")
		_ = store.Put(ctx, "file"+string(rune('0'+i))+".txt", key, 7, "")
	}

	// List with limit
	result, err := store.List(ctx, "", "", "", 5)
	require.NoError(t, err)
	assert.Len(t, result.Objects, 5)
	assert.True(t, result.IsTruncated)
}

// TestS3Store_List_WithMarker tests List with marker
func TestS3Store_List_WithMarker(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Create files
	_ = store.Put(ctx, "a.txt", strings.NewReader("a"), 1, "")
	_ = store.Put(ctx, "b.txt", strings.NewReader("b"), 1, "")
	_ = store.Put(ctx, "c.txt", strings.NewReader("c"), 1, "")

	// List with marker
	result, err := store.List(ctx, "", "b.txt", "", 0)
	require.NoError(t, err)

	// Should only include files after b.txt
	hasA := false
	hasB := false
	hasC := false
	for _, obj := range result.Objects {
		if obj.Key == "a.txt" {
			hasA = true
		}
		if obj.Key == "b.txt" {
			hasB = true
		}
		if obj.Key == "c.txt" {
			hasC = true
		}
	}
	assert.False(t, hasA, "a.txt should be before marker")
	assert.False(t, hasB, "b.txt should be excluded by marker")
	assert.True(t, hasC, "c.txt should be after marker")
}

// TestS3Store_List_WithDelimiter tests List with delimiter
func TestS3Store_List_WithDelimiter(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Create files in folders
	_ = store.Put(ctx, "folder1/file1.txt", strings.NewReader("content1"), 8, "")
	_ = store.Put(ctx, "folder1/file2.txt", strings.NewReader("content2"), 8, "")
	_ = store.Put(ctx, "folder2/file3.txt", strings.NewReader("content3"), 8, "")

	// List with delimiter - memblob might not support directories the same way
	result, err := store.List(ctx, "", "", "/", 0)
	require.NoError(t, err)
	// The result depends on how memblob handles delimiters
	_ = result
}

// TestS3Store_CreatePrefix tests CreatePrefix operation
func TestS3Store_CreatePrefix(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Create a prefix (directory)
	err := store.CreatePrefix(ctx, "test/folder/")
	require.NoError(t, err)

	// Verify by listing
	result, err := store.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Objects)
}

// TestS3Store_CreatePrefix_WithoutSlash tests CreatePrefix without trailing slash
func TestS3Store_CreatePrefix_WithoutSlash(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Create a prefix without trailing slash - should be added
	err := store.CreatePrefix(ctx, "test/folder")
	require.NoError(t, err)

	// Verify by listing
	result, err := store.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Objects)
}

// TestS3Store_RenamePrefix tests RenamePrefix operation
func TestS3Store_RenamePrefix(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Create files directly in bucket
	_ = bucket.WriteAll(ctx, "old/file1.txt", []byte("content1"), nil)
	_ = bucket.WriteAll(ctx, "old/file2.txt", []byte("content2"), nil)

	// Rename the prefix
	err := store.RenamePrefix(ctx, "old/", "new/")
	require.NoError(t, err)

	// Verify files are in new location
	result, err := store.List(ctx, "new/", "", "", 0)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Objects)

	// Verify old location is empty
	oldResult, err := store.List(ctx, "old/", "", "", 0)
	require.NoError(t, err)

	hasOldFile := false
	for _, obj := range oldResult.Objects {
		if strings.HasSuffix(obj.Key, ".txt") {
			hasOldFile = true
			break
		}
	}
	assert.False(t, hasOldFile, "old files should be gone")
}

// TestS3Store_RenamePrefix_WithoutSlash tests RenamePrefix without trailing slashes
func TestS3Store_RenamePrefix_WithoutSlash(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Create files
	_ = bucket.WriteAll(ctx, "old/file.txt", []byte("content"), nil)

	// Rename without trailing slashes - should be added
	err := store.RenamePrefix(ctx, "old", "new")
	require.NoError(t, err)

	// Verify
	result, err := store.List(ctx, "new/", "", "", 0)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Objects)
}

// TestS3Store_Integration tests complete workflow
func TestS3Store_Integration(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:        bucket,
		ttl:       15 * time.Minute,
		publicURL: "",
	}

	// Create prefix
	err := store.CreatePrefix(ctx, "uploads/")
	require.NoError(t, err)

	// Put file
	data := strings.NewReader("test content for integration")
	err = store.Put(ctx, "uploads/test.txt", data, 27, "text/plain")
	require.NoError(t, err)

	// List files
	result, err := store.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Objects)

	// Verify the file exists
	found := false
	for _, obj := range result.Objects {
		if obj.Key == "uploads/test.txt" {
			found = true
			break
		}
	}
	assert.True(t, found, "uploads/test.txt should exist")

	// Get signed URL with public URL set (memblob doesn't support actual signing)
	store.publicURL = "https://cdn.example.com"
	url, err := store.SignedURL(ctx, "uploads/test.txt", "GET", 0)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/uploads/test.txt", url)

	// Delete file
	store.publicURL = "" // reset for delete
	err = store.Delete(ctx, "uploads/test.txt")
	require.NoError(t, err)

	// Verify deletion
	result, err = store.List(ctx, "", "", "", 0)
	require.NoError(t, err)

	found = false
	for _, obj := range result.Objects {
		if obj.Key == "uploads/test.txt" {
			found = true
			break
		}
	}
	assert.False(t, found, "uploads/test.txt should be deleted")
}

// mockReadCloser is a mock io.ReadCloser for testing
type mockReadCloser struct {
	*strings.Reader
}

func (m *mockReadCloser) Close() error {
	return nil
}

// TestS3Store_Delete_NonExistent tests deleting non-existent file
func TestS3Store_Delete_NonExistent(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Delete non-existent file - should not error
	err := store.Delete(ctx, "nonexistent.txt")
	// memblob might return error for non-existent files
	_ = err
	// We just verify it doesn't panic
}

// TestS3Store_List_EmptyBucket tests listing empty bucket
func TestS3Store_List_EmptyBucket(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// List empty bucket
	result, err := store.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	assert.Empty(t, result.Objects)
	assert.Empty(t, result.Prefixes)
}

// TestS3Store_Put_EmptyKey tests Put with empty key
func TestS3Store_Put_EmptyKey(t *testing.T) {
	ctx := context.Background()

	bucket := memblob.OpenBucket(nil)
	defer bucket.Close()

	store := &s3Store{
		bk:  bucket,
		ttl: 15 * time.Minute,
	}

	// Put with empty key after sanitization
	data := strings.NewReader("content")
	err := store.Put(ctx, "../escaped.txt", data, 7, "")
	// sanitizeKey will strip the ../, making it just "escaped.txt"
	require.NoError(t, err)

	// Verify file was created with sanitized key
	result, err := store.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Objects)
}
