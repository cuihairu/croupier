package objstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFileStore_PutContentType tests Put with various content types
func TestFileStore_PutContentType(t *testing.T) {
	tmpDir := t.TempDir()
	store := &fileStore{
		base:         tmpDir,
		publicPrefix: "/uploads/",
		ttl:          15 * time.Minute,
	}

	ctx := context.Background()
	contentTypes := []string{
		"text/plain",
		"application/json",
		"image/png",
		"application/octet-stream",
		"",
	}

	for _, ct := range contentTypes {
		t.Run(ct, func(t *testing.T) {
			data := strings.NewReader("test content")
			err := store.Put(ctx, "test/file.txt", data, 12, ct)
			if err != nil {
				t.Errorf("Put error with content type %q: %v", ct, err)
			}
		})
	}
}

// TestFileStore_Put_SpecialKeys tests Put with special key formats
func TestFileStore_Put_SpecialKeys(t *testing.T) {
	tmpDir := t.TempDir()
	store := &fileStore{
		base:         tmpDir,
		publicPrefix: "/uploads/",
		ttl:          15 * time.Minute,
	}

	ctx := context.Background()

	tests := []struct {
		name        string
		key         string
		expectError bool
	}{
		{"simple", "file.txt", false},
		{"with path", "dir/file.txt", false},
		{"nested path", "a/b/c/file.txt", false},
		{"with leading slash", "/leading/file.txt", false},
		{"with dot segments", "a/../b/file.txt", false},
		{"with multiple slashes", "a///b//file.txt", false},
		{"unicode", "测试/文件.txt", false},
		{"with spaces", "my file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := strings.NewReader("test")
			err := store.Put(ctx, tt.key, data, 4, "text/plain")
			if (err != nil) != tt.expectError {
				t.Errorf("Put(%q) error = %v, expectError %v", tt.key, err, tt.expectError)
			}
		})
	}
}

// TestFileStore_Delete_Scenarios tests various delete scenarios
func TestFileStore_Delete_Scenarios(t *testing.T) {
	tmpDir := t.TempDir()
	store := &fileStore{
		base:         tmpDir,
		publicPrefix: "/uploads/",
		ttl:          15 * time.Minute,
	}

	ctx := context.Background()

	// Create test files
	testFile := "test/file.txt"
	nestedDir := "nested/dir/file.txt"
	folder := "folder/"

	data := strings.NewReader("test")
	_ = store.Put(ctx, testFile, data, 4, "text/plain")
	_ = store.Put(ctx, nestedDir, data, 4, "text/plain")
	_ = store.CreatePrefix(ctx, "folder")

	// Test deleting non-existent file
	t.Run("delete non-existent", func(t *testing.T) {
		err := store.Delete(ctx, "nonexistent.txt")
		if err == nil {
			t.Error("Expected error deleting non-existent file")
		}
	})

	// Test deleting existing file
	t.Run("delete existing file", func(t *testing.T) {
		err := store.Delete(ctx, testFile)
		if err != nil {
			t.Errorf("Delete error: %v", err)
		}

		// Verify file is gone
		path := filepath.Join(tmpDir, "test", "file.txt")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("File should be deleted")
		}
	})

	// Test deleting nested file
	t.Run("delete nested file", func(t *testing.T) {
		err := store.Delete(ctx, nestedDir)
		if err != nil {
			t.Errorf("Delete error: %v", err)
		}
	})

	// Test deleting folder
	t.Run("delete folder", func(t *testing.T) {
		err := store.Delete(ctx, folder)
		if err != nil {
			t.Errorf("Delete folder error: %v", err)
		}

		// Verify folder is gone
		path := filepath.Join(tmpDir, "folder")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("Folder should be deleted")
		}
	})
}

// TestFileStore_List_AllOptions tests List with various option combinations
func TestFileStore_List_AllOptions(t *testing.T) {
	tmpDir := t.TempDir()
	store := &fileStore{
		base:         tmpDir,
		publicPrefix: "/uploads/",
		ttl:          15 * time.Minute,
	}

	ctx := context.Background()

	// Create test structure
	_ = store.CreatePrefix(ctx, "dir1")
	_ = store.CreatePrefix(ctx, "dir2")
	data := strings.NewReader("test")
	_ = store.Put(ctx, "file1.txt", data, 4, "text/plain")
	_ = store.Put(ctx, "dir1/file2.txt", data, 4, "text/plain")
	_ = store.Put(ctx, "dir2/file3.txt", data, 4, "text/plain")

	tests := []struct {
		name      string
		prefix    string
		marker    string
		delimiter string
		limit     int
	}{
		{"all files", "", "", "", 0},
		{"with prefix", "dir1/", "", "", 0},
		{"with limit", "", "", "", 2},
		{"with delimiter", "", "", "/", 0},
		{"prefix and delimiter", "dir1/", "", "/", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := store.List(ctx, tt.prefix, tt.marker, tt.delimiter, tt.limit)
			if err != nil {
				t.Errorf("List error: %v", err)
			}
			// Verify result structure
			_ = result.Objects
			_ = result.Prefixes
			_ = result.IsTruncated
			_ = result.NextMarker
		})
	}
}

// TestFileStore_List_EmptyResults tests List with empty results
func TestFileStore_List_EmptyResults(t *testing.T) {
	tmpDir := t.TempDir()
	store := &fileStore{
		base:         tmpDir,
		publicPrefix: "/uploads/",
		ttl:          15 * time.Minute,
	}

	ctx := context.Background()

	// Create the directory first to avoid errors
	_ = os.MkdirAll(filepath.Join(tmpDir, "testdir"), 0755)

	result, err := store.List(ctx, "testdir/", "", "", 0)
	if err != nil {
		t.Errorf("List error: %v", err)
	}
	if len(result.Objects) != 0 {
		t.Errorf("Expected 0 objects, got %d", len(result.Objects))
	}
	if len(result.Prefixes) != 0 {
		t.Errorf("Expected 0 prefixes, got %d", len(result.Prefixes))
	}
}

// TestFileStore_CreatePrefix_Variations tests CreatePrefix with various inputs
func TestFileStore_CreatePrefix_Variations(t *testing.T) {
	tmpDir := t.TempDir()
	store := &fileStore{
		base:         tmpDir,
		publicPrefix: "/uploads/",
		ttl:          15 * time.Minute,
	}

	ctx := context.Background()

	tests := []struct {
		name   string
		prefix string
	}{
		{"simple", "testdir"},
		{"with slash", "testdir/"},
		{"nested", "a/b/c"},
		{"with leading slash", "/testdir/"},
		{"deep nested", "a/b/c/d/e"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.CreatePrefix(ctx, tt.prefix)
			if err != nil {
				t.Errorf("CreatePrefix(%q) error: %v", tt.prefix, err)
			}
		})
	}
}

// TestFileStore_RenamePrefix_Comprehensive tests comprehensive rename scenarios
func TestFileStore_RenamePrefix_Comprehensive(t *testing.T) {
	tmpDir := t.TempDir()
	store := &fileStore{
		base:         tmpDir,
		publicPrefix: "/uploads/",
		ttl:          15 * time.Minute,
	}

	ctx := context.Background()

	tests := []struct {
		name      string
		oldPrefix string
		newPrefix string
		setup     func()
	}{
		{
			name:      "simple rename",
			oldPrefix: "old/",
			newPrefix: "new/",
			setup: func() {
				_ = store.CreatePrefix(ctx, "old/")
				data := strings.NewReader("test")
				_ = store.Put(ctx, "old/file.txt", data, 4, "text/plain")
			},
		},
		{
			name:      "nested rename",
			oldPrefix: "a/",
			newPrefix: "x/",
			setup: func() {
				_ = store.CreatePrefix(ctx, "a/")
				data := strings.NewReader("test")
				_ = store.Put(ctx, "a/file.txt", data, 4, "text/plain")
			},
		},
		{
			name:      "rename with files",
			oldPrefix: "src/",
			newPrefix: "dst/",
			setup: func() {
				_ = store.CreatePrefix(ctx, "src/")
				data := strings.NewReader("test")
				_ = store.Put(ctx, "src/f1.txt", data, 4, "text/plain")
				_ = store.Put(ctx, "src/f2.txt", data, 4, "text/plain")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := store.RenamePrefix(ctx, tt.oldPrefix, tt.newPrefix)
			if err != nil {
				t.Errorf("RenamePrefix error: %v", err)
			}

			// Verify new prefix exists
			newPath := filepath.Join(tmpDir, filepath.FromSlash(tt.newPrefix))
			if _, err := os.Stat(newPath); err != nil {
				t.Errorf("New prefix path doesn't exist: %v", err)
			}
		})
	}
}

// TestFileStore_SignedURL_PublicURL tests SignedURL with public URL
func TestFileStore_SignedURL_PublicURL(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		key       string
		expected  string
	}{
		{
			name:      "with public URL",
			publicURL: "https://cdn.example.com",
			key:       "test/file.txt",
			expected:  "https://cdn.example.com/test/file.txt",
		},
		{
			name:      "public URL with trailing slash",
			publicURL: "https://cdn.example.com/",
			key:       "test/file.txt",
			expected:  "https://cdn.example.com/test/file.txt",
		},
		{
			name:      "public URL with path",
			publicURL: "https://cdn.example.com/files",
			key:       "test/file.txt",
			expected:  "https://cdn.example.com/files/test/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fileStore{
				base:         t.TempDir(),
				publicPrefix: "/uploads/",
				ttl:          15 * time.Minute,
				publicURL:    tt.publicURL,
			}

			ctx := context.Background()
			url, err := store.SignedURL(ctx, tt.key, "GET", time.Hour)
			if err != nil {
				t.Errorf("SignedURL error: %v", err)
			}
			if url != tt.expected {
				t.Errorf("URL = %q, want %q", url, tt.expected)
			}
		})
	}
}

// TestFileStore_SignedURL_RelativePath tests SignedURL without public URL
func TestFileStore_SignedURL_RelativePath(t *testing.T) {
	store := &fileStore{
		base:         t.TempDir(),
		publicPrefix: "/custom/",
		ttl:          15 * time.Minute,
	}

	ctx := context.Background()

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"simple", "file.txt", "/custom/file.txt"},
		{"with path", "dir/file.txt", "/custom/dir/file.txt"},
		{"nested", "a/b/file.txt", "/custom/a/b/file.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := store.SignedURL(ctx, tt.key, "GET", time.Hour)
			if err != nil {
				t.Errorf("SignedURL error: %v", err)
			}
			if url != tt.expected {
				t.Errorf("URL = %q, want %q", url, tt.expected)
			}
		})
	}
}

// TestFileStore_SignedURL_DeleteMethod2 tests DELETE method rejection
func TestFileStore_SignedURL_DeleteMethod2(t *testing.T) {
	store := &fileStore{
		base:         t.TempDir(),
		publicPrefix: "/uploads/",
		ttl:          15 * time.Minute,
	}

	ctx := context.Background()

	_, err := store.SignedURL(ctx, "test/file.txt", "DELETE", time.Hour)
	if err == nil {
		t.Error("Expected error for DELETE method")
	}
	if err != nil && err.Error() != "not supported" {
		t.Errorf("Expected 'not supported' error, got: %v", err)
	}
}

// TestFileStore_SignedURL_AllMethods tests all allowed methods
func TestFileStore_SignedURL_AllMethods(t *testing.T) {
	store := &fileStore{
		base:         t.TempDir(),
		publicPrefix: "/uploads/",
		ttl:          15 * time.Minute,
	}

	ctx := context.Background()

	methods := []string{"GET", "PUT", "POST", ""}
	for _, method := range methods {
		t.Run("method_"+method, func(t *testing.T) {
			_, err := store.SignedURL(ctx, "test/file.txt", method, time.Hour)
			if err != nil {
				t.Errorf("SignedURL(%s) error: %v", method, err)
			}
		})
	}
}

// TestFileStore_Integration_Full tests full file store workflow
func TestFileStore_Integration_Full(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Open store
	cfg := Config{Driver: "file", BaseDir: tmpDir}
	store, err := OpenFile(ctx, cfg)
	if err != nil {
		t.Fatalf("OpenFile error: %v", err)
	}

	// 1. Create prefixes
	_ = store.CreatePrefix(ctx, "uploads")
	_ = store.CreatePrefix(ctx, "backup")

	// 2. Upload files
	contents := map[string]string{
		"uploads/file1.txt": "content1",
		"uploads/file2.txt": "content2",
		"backup/data.txt":   "backup",
	}

	for key, content := range contents {
		data := strings.NewReader(content)
		err := store.Put(ctx, key, data, int64(len(content)), "text/plain")
		if err != nil {
			t.Errorf("Put(%s) error: %v", key, err)
		}
	}

	// 3. List files
	result, err := store.List(ctx, "uploads/", "", "", 0)
	if err != nil {
		t.Errorf("List error: %v", err)
	}
	if len(result.Objects) < 2 {
		t.Errorf("Expected at least 2 files, got %d", len(result.Objects))
	}

	// 4. List with delimiter
	result, err = store.List(ctx, "", "", "/", 0)
	if err != nil {
		t.Errorf("List with delimiter error: %v", err)
	}
	if len(result.Prefixes) < 2 {
		t.Errorf("Expected at least 2 prefixes, got %d", len(result.Prefixes))
	}

	// 5. Rename prefix
	_ = store.CreatePrefix(ctx, "old/")
	data := strings.NewReader("test")
	_ = store.Put(ctx, "old/file.txt", data, 4, "text/plain")
	err = store.RenamePrefix(ctx, "old/", "new/")
	if err != nil {
		t.Errorf("RenamePrefix error: %v", err)
	}

	// 6. Delete files
	_ = store.Delete(ctx, "uploads/file1.txt")
	_ = store.Delete(ctx, "uploads/file2.txt")

	// 7. Delete folder
	_ = store.Delete(ctx, "uploads/")
}

// TestFileStore_ErrorScenarios tests various error scenarios
func TestFileStore_ErrorScenarios(t *testing.T) {
	tmpDir := t.TempDir()
	store := &fileStore{
		base:         tmpDir,
		publicPrefix: "/uploads/",
		ttl:          15 * time.Minute,
	}

	ctx := context.Background()

	// Test delete non-existent
	err := store.Delete(ctx, "nonexistent/file.txt")
	if err == nil {
		t.Error("Expected error deleting non-existent file")
	}

	// Create a directory for list test
	_ = os.MkdirAll(filepath.Join(tmpDir, "testdir"), 0755)
	result, err := store.List(ctx, "testdir/", "", "", 0)
	if err != nil {
		t.Errorf("List error: %v", err)
	}
	if len(result.Objects) != 0 {
		t.Error("Expected no objects in empty directory")
	}
}

// TestOpenFile_Validation tests OpenFile validation
func TestOpenFile_Validation(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		baseDir     string
		expectError bool
	}{
		{
			name:        "valid temp dir",
			baseDir:     "",
			expectError: false,
		},
		{
			name:        "explicit valid dir",
			baseDir:     t.TempDir(),
			expectError: false,
		},
		{
			name:        "empty base dir",
			baseDir:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Driver: "file", BaseDir: tt.baseDir}
			if tt.name == "valid temp dir" {
				cfg.BaseDir = t.TempDir()
			}
			if tt.name == "empty base dir" {
				cfg.BaseDir = ""
			}

			store, err := OpenFile(ctx, cfg)
			if (err != nil) != tt.expectError {
				t.Errorf("OpenFile() error = %v, expectError %v", err, tt.expectError)
			}
			if !tt.expectError && store == nil {
				t.Error("Expected non-nil store when no error")
			}
		})
	}
}
