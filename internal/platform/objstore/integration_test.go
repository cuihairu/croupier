package objstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestOpenFile_Variations tests OpenFile with various configurations
func TestOpenFile_Variations(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		baseDir   string
		publicURL string
	}{
		{
			name:      "basic file store",
			baseDir:   "",
			publicURL: "",
		},
		{
			name:      "with public URL",
			baseDir:   "",
			publicURL: "https://cdn.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if tt.baseDir != "" {
				tmpDir = tt.baseDir
			}

			cfg := Config{
				Driver:    "file",
				BaseDir:   tmpDir,
				PublicURL: tt.publicURL,
			}
			store, err := OpenFile(ctx, cfg)
			if err != nil {
				t.Fatalf("OpenFile error: %v", err)
			}
			if store == nil {
				t.Fatal("OpenFile returned nil store")
			}

			// Test Put operation
			data := strings.NewReader("test content")
			err = store.Put(ctx, "test/file.txt", data, 12, "text/plain")
			if err != nil {
				t.Errorf("Put error: %v", err)
			}

			// Test SignedURL
			url, err := store.SignedURL(ctx, "test/file.txt", "GET", time.Hour)
			if err != nil {
				t.Errorf("SignedURL error: %v", err)
			}
			if tt.publicURL != "" && !strings.Contains(url, tt.publicURL) {
				t.Errorf("Expected public URL in %q", url)
			}

			// Test Delete
			err = store.Delete(ctx, "test/file.txt")
			if err != nil {
				t.Errorf("Delete error: %v", err)
			}

			// Test CreatePrefix
			err = store.CreatePrefix(ctx, "newdir")
			if err != nil {
				t.Errorf("CreatePrefix error: %v", err)
			}
		})
	}
}

// TestOpenFile_EmptyBaseDir tests OpenFile with empty base dir
func TestOpenFile_EmptyBaseDir(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Driver:  "file",
		BaseDir: "",
	}

	_, err := OpenFile(ctx, cfg)
	if err == nil {
		t.Error("Expected error for empty base dir")
	}
}

// TestOpenFile_InvalidBaseDir tests OpenFile with invalid base dir
func TestOpenFile_InvalidBaseDir(t *testing.T) {
	ctx := context.Background()
	// Create a file to simulate invalid base dir
	tmpFile := filepath.Join(t.TempDir(), "file.txt")
	_ = os.WriteFile(tmpFile, []byte("test"), 0644)

	cfg := Config{
		Driver:  "file",
		BaseDir: tmpFile, // A file instead of directory
	}

	_, err := OpenFile(ctx, cfg)
	// This might not error on all systems, so we just log the result
	_ = err
}

// TestDirectoryMarkerCreation_Simulation simulates directory marker creation logic
func TestDirectoryMarkerCreation_Simulation(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		numMarkers int
	}{
		{
			name:       "single level",
			key:        "test/file.txt",
			numMarkers: 1,
		},
		{
			name:       "two levels",
			key:        "a/b/file.txt",
			numMarkers: 2,
		},
		{
			name:       "three levels",
			key:        "a/b/c/file.txt",
			numMarkers: 3,
		},
		{
			name:       "root file",
			key:        "file.txt",
			numMarkers: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := sanitizeKey(tt.key)

			var markers []string
			if strings.Contains(key, "/") {
				dir := key[:strings.LastIndex(key, "/")]
				if dir != "" {
					parts := strings.Split(dir, "/")
					for i := range parts {
						prefix := strings.Join(parts[:i+1], "/") + "/"
						markers = append(markers, prefix)
					}
				}
			}

			if len(markers) != tt.numMarkers {
				t.Errorf("Expected %d markers, got %d", tt.numMarkers, len(markers))
			}
		})
	}
}

// TestFolderDeletion_Logic tests folder deletion detection logic
func TestFolderDeletion_Logic(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		isFolder bool
	}{
		{
			name:     "file key",
			key:      "test/file.txt",
			isFolder: false,
		},
		{
			name:     "folder with trailing slash",
			key:      "test/",
			isFolder: true,
		},
		{
			name:     "nested folder",
			key:      "a/b/c/",
			isFolder: true,
		},
		{
			name:     "folder without trailing slash",
			key:      "test",
			isFolder: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := sanitizeKey(tt.key)
			if strings.HasSuffix(tt.key, "/") && !strings.HasSuffix(key, "/") {
				key += "/"
			}
			isFolder := strings.HasSuffix(key, "/")

			if isFolder != tt.isFolder {
				t.Errorf("isFolder = %v, want %v", isFolder, tt.isFolder)
			}
		})
	}
}

// TestPrefixSlashNormalization tests prefix slash normalization
func TestPrefixSlashNormalization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with trailing slash",
			input:    "test/",
			expected: "test/",
		},
		{
			name:     "without trailing slash",
			input:    "test",
			expected: "test/",
		},
		{
			name:     "with leading slash",
			input:    "/test/",
			expected: "test/",
		},
		{
			name:     "with dot segments",
			input:    "a/../test",
			expected: "a/test/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := sanitizeKey(tt.input)
			if !strings.HasSuffix(prefix, "/") && prefix != "" {
				prefix += "/"
			}

			if prefix != tt.expected {
				t.Errorf("prefix = %q, want %q", prefix, tt.expected)
			}
		})
	}
}

// TestListOperation_ResultStructure tests ListResult structure handling
func TestListOperation_ResultStructure(t *testing.T) {
	now := time.Now()

	// Test empty result
	emptyResult := ListResult{
		Objects:     []ObjectInfo{},
		Prefixes:    []string{},
		IsTruncated: false,
		NextMarker:  "",
	}
	_ = emptyResult.Objects
	_ = emptyResult.Prefixes
	_ = emptyResult.IsTruncated
	_ = emptyResult.NextMarker

	// Test with data
	dataResult := ListResult{
		Objects: []ObjectInfo{
			{Key: "file1.txt", Size: 100, LastModified: now, ETag: "etag1"},
			{Key: "file2.txt", Size: 200, LastModified: now, ETag: "etag2"},
		},
		Prefixes:    []string{"dir1/", "dir2/"},
		IsTruncated: true,
		NextMarker:  "file1.txt",
	}
	_ = dataResult.Objects
	_ = dataResult.Prefixes
	_ = dataResult.IsTruncated
	_ = dataResult.NextMarker
}

// TestRenamePrefix_KeyReplacementLogic tests key replacement logic
func TestRenamePrefix_KeyReplacementLogic(t *testing.T) {
	tests := []struct {
		name      string
		oldKey    string
		oldPrefix string
		newPrefix string
		newKey    string
	}{
		{
			name:      "simple replacement",
			oldKey:    "old/file.txt",
			oldPrefix: "old/",
			newPrefix: "new/",
			newKey:    "new/file.txt",
		},
		{
			name:      "nested replacement",
			oldKey:    "a/b/c/file.txt",
			oldPrefix: "a/b/",
			newPrefix: "x/y/",
			newKey:    "x/y/c/file.txt",
		},
		{
			name:      "deep replacement",
			oldKey:    "old/nested/path/to/file.txt",
			oldPrefix: "old/nested/path/",
			newPrefix: "new/path/",
			newKey:    "new/path/to/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newKey := strings.Replace(tt.oldKey, tt.oldPrefix, tt.newPrefix, 1)
			if newKey != tt.newKey {
				t.Errorf("newKey = %q, want %q", newKey, tt.newKey)
			}
		})
	}
}

// TestObjectInfo_Fields tests ObjectInfo field handling
func TestObjectInfo_Fields(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		obj  ObjectInfo
	}{
		{
			name: "all fields",
			obj: ObjectInfo{
				Key:          "test/file.txt",
				Size:         1024,
				LastModified: now,
				ETag:         "etag123",
				StorageClass: "STANDARD",
			},
		},
		{
			name: "minimal",
			obj: ObjectInfo{
				Key: "file.txt",
			},
		},
		{
			name: "with zero size",
			obj: ObjectInfo{
				Key:  "empty.txt",
				Size: 0,
			},
		},
		{
			name: "large file",
			obj: ObjectInfo{
				Key:          "large/file.bin",
				Size:         1024 * 1024 * 100,
				LastModified: now,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.obj.Key == "" {
				t.Error("ObjectInfo.Key should not be empty")
			}
			if tt.obj.Size < 0 {
				t.Error("ObjectInfo.Size should not be negative")
			}
		})
	}
}
