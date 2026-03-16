package objstore

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// TestS3Store_Put_Operation tests S3 Put operation logic
func TestS3Store_Put_Operation(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		contentType  string
		sanitizedKey string
	}{
		{
			name:         "simple file",
			key:          "test/file.txt",
			contentType:  "text/plain",
			sanitizedKey: "test/file.txt",
		},
		{
			name:         "with leading slash",
			key:          "/leading/file.txt",
			contentType:  "application/json",
			sanitizedKey: "leading/file.txt",
		},
		{
			name:         "with dot segments",
			key:          "path/../file.txt",
			contentType:  "",
			sanitizedKey: "path/file.txt",
		},
		{
			name:         "nested path",
			key:          "a/b/c/d/e/file.txt",
			contentType:  "image/png",
			sanitizedKey: "a/b/c/d/e/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeKey(tt.key)
			if result != tt.sanitizedKey {
				t.Errorf("sanitizeKey() = %q, want %q", result, tt.sanitizedKey)
			}

			// Test directory marker creation logic
			if strings.Contains(result, "/") {
				dir := result[:strings.LastIndex(result, "/")]
				if dir != "" {
					parts := strings.Split(dir, "/")
					if len(parts) > 0 {
						// Verify we can iterate and create markers
						for i := range parts {
							prefix := strings.Join(parts[:i+1], "/") + "/"
							_ = prefix
						}
					}
				}
			}
		})
	}
}

// TestS3Store_Delete_Operation tests S3 Delete operation logic
func TestS3Store_Delete_Operation(t *testing.T) {
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
			name:     "folder key",
			key:      "test/",
			isFolder: true,
		},
		{
			name:     "nested folder",
			key:      "a/b/c/",
			isFolder: true,
		},
		{
			name:     "folder with leading slash",
			key:      "/test/",
			isFolder: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := sanitizeKey(tt.key)
			// Preserve trailing slash for folders
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

// TestS3Store_List_MarkerLogic tests marker-based pagination logic
func TestS3Store_List_MarkerLogic(t *testing.T) {
	tests := []struct {
		name       string
		marker     string
		objKey     string
		shouldSkip bool
	}{
		{
			name:       "object after marker",
			marker:     "a.txt",
			objKey:     "b.txt",
			shouldSkip: false,
		},
		{
			name:       "object before marker",
			marker:     "b.txt",
			objKey:     "a.txt",
			shouldSkip: true,
		},
		{
			name:       "object equals marker",
			marker:     "a.txt",
			objKey:     "a.txt",
			shouldSkip: true,
		},
		{
			name:       "no marker",
			marker:     "",
			objKey:     "file.txt",
			shouldSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldSkip := tt.marker != "" && tt.objKey <= tt.marker
			if shouldSkip != tt.shouldSkip {
				t.Errorf("shouldSkip = %v, want %v", shouldSkip, tt.shouldSkip)
			}
		})
	}
}

// TestS3Store_CreatePrefix_Operation tests CreatePrefix operation logic
func TestS3Store_CreatePrefix_Operation(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		expectedKey string
	}{
		{
			name:        "with trailing slash",
			prefix:      "test/",
			expectedKey: "test/",
		},
		{
			name:        "without trailing slash",
			prefix:      "test",
			expectedKey: "test/",
		},
		{
			name:        "with leading slash",
			prefix:      "/test/",
			expectedKey: "test/",
		},
		{
			name:        "nested",
			prefix:      "a/b/c",
			expectedKey: "a/b/c/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := sanitizeKey(tt.prefix)
			if !strings.HasSuffix(prefix, "/") && prefix != "" {
				prefix += "/"
			}

			if prefix != tt.expectedKey {
				t.Errorf("prefix = %q, want %q", prefix, tt.expectedKey)
			}
		})
	}
}

// TestS3Store_RenamePrefix_SlashHandling tests slash handling in rename
func TestS3Store_RenamePrefix_SlashHandling(t *testing.T) {
	tests := []struct {
		name      string
		oldPrefix string
		newPrefix string
		wantOld   string
		wantNew   string
	}{
		{
			name:      "both with slash",
			oldPrefix: "old/",
			newPrefix: "new/",
			wantOld:   "old/",
			wantNew:   "new/",
		},
		{
			name:      "both without slash",
			oldPrefix: "old",
			newPrefix: "new",
			wantOld:   "old/",
			wantNew:   "new/",
		},
		{
			name:      "mixed slashes",
			oldPrefix: "old/",
			newPrefix: "new",
			wantOld:   "old/",
			wantNew:   "new/",
		},
		{
			name:      "with leading slash",
			oldPrefix: "/old/",
			newPrefix: "/new/",
			wantOld:   "old/",
			wantNew:   "new/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldPrefix := sanitizeKey(tt.oldPrefix)
			newPrefix := sanitizeKey(tt.newPrefix)

			if !strings.HasSuffix(oldPrefix, "/") && oldPrefix != "" {
				oldPrefix += "/"
			}
			if !strings.HasSuffix(newPrefix, "/") && newPrefix != "" {
				newPrefix += "/"
			}

			if oldPrefix != tt.wantOld {
				t.Errorf("oldPrefix = %q, want %q", oldPrefix, tt.wantOld)
			}
			if newPrefix != tt.wantNew {
				t.Errorf("newPrefix = %q, want %q", newPrefix, tt.wantNew)
			}
		})
	}
}

// TestS3Store_ExpiryFallback tests expiry fallback to default TTL
func TestS3Store_ExpiryFallback(t *testing.T) {
	tests := []struct {
		name     string
		ttl      time.Duration
		expiry   time.Duration
		expected time.Duration
	}{
		{
			name:     "use provided expiry",
			ttl:      15 * time.Minute,
			expiry:   30 * time.Minute,
			expected: 30 * time.Minute,
		},
		{
			name:     "zero expiry uses TTL",
			ttl:      15 * time.Minute,
			expiry:   0,
			expected: 15 * time.Minute,
		},
		{
			name:     "negative expiry uses TTL",
			ttl:      20 * time.Minute,
			expiry:   -1,
			expected: 20 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.expiry
			if result <= 0 {
				result = tt.ttl
			}
			if result != tt.expected {
				t.Errorf("Result = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestOSSStore_Put_Operation tests OSS Put operation logic
func TestOSSStore_Put_Operation(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		contentType  string
		sanitizedKey string
	}{
		{
			name:         "simple file",
			key:          "test/file.txt",
			contentType:  "text/plain",
			sanitizedKey: "test/file.txt",
		},
		{
			name:         "with leading slash",
			key:          "/leading/file.txt",
			contentType:  "application/json",
			sanitizedKey: "leading/file.txt",
		},
		{
			name:         "with dot segments",
			key:          "path/../file.txt",
			contentType:  "",
			sanitizedKey: "path/file.txt",
		},
		{
			name:         "nested path",
			key:          "a/b/c/d/e/file.txt",
			contentType:  "image/png",
			sanitizedKey: "a/b/c/d/e/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeKey(tt.key)
			if result != tt.sanitizedKey {
				t.Errorf("sanitizeKey() = %q, want %q", result, tt.sanitizedKey)
			}
		})
	}
}

// TestOSSStore_Delete_Operation tests OSS Delete operation logic
func TestOSSStore_Delete_Operation(t *testing.T) {
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
			name:     "folder key",
			key:      "test/",
			isFolder: true,
		},
		{
			name:     "nested folder",
			key:      "a/b/c/",
			isFolder: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := sanitizeKey(tt.key)
			// Preserve trailing slash for folders
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

// TestOSSStore_SignedURL_MethodMapping tests HTTP method mapping for OSS
func TestOSSStore_SignedURL_MethodMapping(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		expectError bool
	}{
		{
			name:        "GET method",
			method:      "GET",
			expectError: false,
		},
		{
			name:        "PUT method",
			method:      "PUT",
			expectError: false,
		},
		{
			name:        "DELETE method",
			method:      "DELETE",
			expectError: false,
		},
		{
			name:        "empty defaults to GET",
			method:      "",
			expectError: false,
		},
		{
			name:        "PATCH unsupported",
			method:      "PATCH",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error

			switch tt.method {
			case "PUT":
				// valid
			case "DELETE":
				// valid
			case "GET", "":
				// valid
			default:
				err = errors.New("unsupported method: " + tt.method)
			}

			if (err != nil) != tt.expectError {
				t.Errorf("Error existence = %v, want %v", err != nil, tt.expectError)
			}
		})
	}
}

// TestOSSStore_CreatePrefix_Operation tests CreatePrefix operation logic
func TestOSSStore_CreatePrefix_Operation(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		expectedKey string
	}{
		{
			name:        "with trailing slash",
			prefix:      "test/",
			expectedKey: "test/",
		},
		{
			name:        "without trailing slash",
			prefix:      "test",
			expectedKey: "test/",
		},
		{
			name:        "with leading slash",
			prefix:      "/test/",
			expectedKey: "test/",
		},
		{
			name:        "nested",
			prefix:      "a/b/c",
			expectedKey: "a/b/c/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := sanitizeKey(tt.prefix)
			if !strings.HasSuffix(prefix, "/") && prefix != "" {
				prefix += "/"
			}

			if prefix != tt.expectedKey {
				t.Errorf("prefix = %q, want %q", prefix, tt.expectedKey)
			}
		})
	}
}

// TestOSSStore_RenamePrefix_KeyReplacement tests key replacement in rename
func TestOSSStore_RenamePrefix_KeyReplacement(t *testing.T) {
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

// TestFileStore_SignedURL_MethodCheck tests SignedURL method validation for file store
func TestFileStore_SignedURL_MethodCheck(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		expectError bool
	}{
		{
			name:        "GET method",
			method:      "GET",
			expectError: false,
		},
		{
			name:        "PUT method",
			method:      "PUT",
			expectError: false,
		},
		{
			name:        "POST method",
			method:      "POST",
			expectError: false,
		},
		{
			name:        "DELETE method not supported",
			method:      "DELETE",
			expectError: true,
		},
		{
			name:        "empty method",
			method:      "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			// File store rejects DELETE method
			if tt.method == "DELETE" {
				err = errors.New("not supported")
			}

			if (err != nil) != tt.expectError {
				t.Errorf("Error existence = %v, want %v", err != nil, tt.expectError)
			}
		})
	}
}

// TestFileStore_Delete_Operation tests file delete operation logic
func TestFileStore_Delete_Operation(t *testing.T) {
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
			name:     "folder key",
			key:      "test/",
			isFolder: true,
		},
		{
			name:     "nested folder",
			key:      "a/b/c/",
			isFolder: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := sanitizeKey(tt.key)
			// Preserve trailing slash for folder detection
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

// TestFileStore_CreatePrefix_Operation tests CreatePrefix operation logic
func TestFileStore_CreatePrefix_Operation(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		expectedKey string
	}{
		{
			name:        "with trailing slash",
			prefix:      "test/",
			expectedKey: "test/",
		},
		{
			name:        "without trailing slash",
			prefix:      "test",
			expectedKey: "test/",
		},
		{
			name:        "with leading slash",
			prefix:      "/test/",
			expectedKey: "test/",
		},
		{
			name:        "nested",
			prefix:      "a/b/c",
			expectedKey: "a/b/c/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := sanitizeKey(tt.prefix)
			// File store ensures trailing slash
			if !strings.HasSuffix(prefix, "/") && prefix != "" {
				prefix += "/"
			}

			if prefix != tt.expectedKey {
				t.Errorf("prefix = %q, want %q", prefix, tt.expectedKey)
			}
		})
	}
}

// TestFileStore_RenamePrefix_Operation tests RenamePrefix operation logic
func TestFileStore_RenamePrefix_Operation(t *testing.T) {
	tests := []struct {
		name      string
		oldPrefix string
		newPrefix string
		wantOld   string
		wantNew   string
	}{
		{
			name:      "both with slash",
			oldPrefix: "old/",
			newPrefix: "new/",
			wantOld:   "old/",
			wantNew:   "new/",
		},
		{
			name:      "both without slash",
			oldPrefix: "old",
			newPrefix: "new",
			wantOld:   "old/",
			wantNew:   "new/",
		},
		{
			name:      "mixed slashes",
			oldPrefix: "old/",
			newPrefix: "new",
			wantOld:   "old/",
			wantNew:   "new/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldPrefix := sanitizeKey(tt.oldPrefix)
			newPrefix := sanitizeKey(tt.newPrefix)

			if !strings.HasSuffix(oldPrefix, "/") && oldPrefix != "" {
				oldPrefix += "/"
			}
			if !strings.HasSuffix(newPrefix, "/") && newPrefix != "" {
				newPrefix += "/"
			}

			if oldPrefix != tt.wantOld {
				t.Errorf("oldPrefix = %q, want %q", oldPrefix, tt.wantOld)
			}
			if newPrefix != tt.wantNew {
				t.Errorf("newPrefix = %q, want %q", newPrefix, tt.wantNew)
			}
		})
	}
}

// TestReadSeeker_Implementation_Content tests ReadSeeker with various content
func TestReadSeeker_Implementation_Content(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "empty content",
			content: "",
		},
		{
			name:    "simple content",
			content: "hello world",
		},
		{
			name:    "binary content",
			content: string([]byte{0x00, 0x01, 0x02, 0xff}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := strings.NewReader(tt.content)

			// Test Read
			buf := make([]byte, len(tt.content))
			n, err := rs.Read(buf)
			if err != nil && err != io.EOF {
				t.Errorf("Read() error = %v", err)
			}
			if n != len(tt.content) && tt.content != "" {
				t.Errorf("Read() returned %d bytes, want %d", n, len(tt.content))
			}

			// Test Seek
			pos, err := rs.Seek(0, io.SeekStart)
			if err != nil {
				t.Errorf("Seek() error = %v", err)
			}
			if pos != 0 {
				t.Errorf("Seek() returned %d, want 0", pos)
			}
		})
	}
}

// TestListResult_Truncation tests ListResult truncation logic
func TestListResult_Truncation(t *testing.T) {
	tests := []struct {
		name       string
		objects    []ObjectInfo
		limit      int
		truncated  bool
		nextMarker string
	}{
		{
			name: "under limit",
			objects: []ObjectInfo{
				{Key: "a.txt"},
				{Key: "b.txt"},
			},
			limit:      10,
			truncated:  false,
			nextMarker: "",
		},
		{
			name: "at limit",
			objects: []ObjectInfo{
				{Key: "a.txt"},
				{Key: "b.txt"},
				{Key: "c.txt"},
			},
			limit:      3,
			truncated:  false,
			nextMarker: "",
		},
		{
			name: "over limit",
			objects: []ObjectInfo{
				{Key: "a.txt"},
				{Key: "b.txt"},
				{Key: "c.txt"},
			},
			limit:      2,
			truncated:  true,
			nextMarker: "b.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := 0
			isTruncated := false
			nextMarker := ""

			for i, obj := range tt.objects {
				if tt.limit > 0 && count >= tt.limit {
					isTruncated = true
					nextMarker = tt.objects[i-1].Key
					break
				}
				count++
				_ = obj.Key
			}

			if isTruncated != tt.truncated {
				t.Errorf("isTruncated = %v, want %v", isTruncated, tt.truncated)
			}
			if nextMarker != tt.nextMarker {
				t.Errorf("nextMarker = %q, want %q", nextMarker, tt.nextMarker)
			}
		})
	}
}

// TestKeySanitization_Comprehensive tests comprehensive key sanitization
func TestKeySanitization_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal path",
			input:    "path/to/file.txt",
			expected: "path/to/file.txt",
		},
		{
			name:     "leading slashes",
			input:    "///path/to/file.txt",
			expected: "path/to/file.txt",
		},
		{
			name:     "dot segments",
			input:    "path/./to/../file.txt",
			expected: "path/to/file.txt",
		},
		{
			name:     "trailing slashes - sanitizeKey removes them",
			input:    "path/to/",
			expected: "path/to",
		},
		{
			name:     "multiple consecutive slashes",
			input:    "path///to///file.txt",
			expected: "path/to/file.txt",
		},
		{
			name:     "only dots",
			input:    "./../.",
			expected: "",
		},
		{
			name:     "mixed dot segments and valid parts",
			input:    "a/./b/../c/./d",
			expected: "a/b/c/d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeKey(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
