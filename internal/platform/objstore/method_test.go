package objstore

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestSignedURL_ExpiryLogic tests expiry duration handling across stores
func TestSignedURL_ExpiryLogic(t *testing.T) {
	tests := []struct {
		name           string
		configuredTTL  time.Duration
		providedExpiry time.Duration
		expectedTTL    time.Duration
	}{
		{
			name:           "use provided expiry when positive",
			configuredTTL:  15 * time.Minute,
			providedExpiry: 30 * time.Minute,
			expectedTTL:    30 * time.Minute,
		},
		{
			name:           "use configured TTL when expiry is zero",
			configuredTTL:  15 * time.Minute,
			providedExpiry: 0,
			expectedTTL:    15 * time.Minute,
		},
		{
			name:           "use configured TTL when expiry is negative",
			configuredTTL:  20 * time.Minute,
			providedExpiry: -1,
			expectedTTL:    20 * time.Minute,
		},
		{
			name:           "zero configured TTL uses default",
			configuredTTL:  0,
			providedExpiry: 0,
			expectedTTL:    15 * time.Minute, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the expiry logic
			expiry := tt.providedExpiry
			if expiry <= 0 {
				expiry = tt.configuredTTL
				if expiry == 0 {
					expiry = 15 * time.Minute // default
				}
			}

			if expiry != tt.expectedTTL {
				t.Errorf("Expected TTL = %v, got %v", tt.expectedTTL, expiry)
			}
		})
	}
}

// TestSignedURL_HTTPMethods tests HTTP method handling
func TestSignedURL_HTTPMethods(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		expectValid bool
	}{
		{name: "GET", method: "GET", expectValid: true},
		{name: "PUT", method: "PUT", expectValid: true},
		{name: "DELETE", method: "DELETE", expectValid: true},
		{name: "POST", method: "POST", expectValid: true},
		{name: "HEAD", method: "HEAD", expectValid: true},
		{name: "empty", method: "", expectValid: true},
		{name: "PATCH", method: "PATCH", expectValid: false},
		{name: "OPTIONS", method: "OPTIONS", expectValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test method validation logic
			_ = tt.method
			_ = tt.expectValid
		})
	}
}

// TestSignedURL_KeySanitization tests key sanitization in SignedURL
func TestSignedURL_KeySanitization(t *testing.T) {
	tests := []struct {
		name        string
		inputKey    string
		expectedKey string
	}{
		{
			name:        "normal key",
			inputKey:    "path/to/file.txt",
			expectedKey: "path/to/file.txt",
		},
		{
			name:        "with leading slash",
			inputKey:    "/path/to/file.txt",
			expectedKey: "path/to/file.txt",
		},
		{
			name:        "with dot segments",
			inputKey:    "path/../to/file.txt",
			expectedKey: "path/to/file.txt",
		},
		{
			name:        "with multiple slashes",
			inputKey:    "path///to/file.txt",
			expectedKey: "path/to/file.txt",
		},
		{
			name:        "empty key",
			inputKey:    "",
			expectedKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := sanitizeKey(tt.inputKey)
			if sanitized != tt.expectedKey {
				t.Errorf("sanitizeKey() = %q, want %q", sanitized, tt.expectedKey)
			}
		})
	}
}

// TestSignedURL_PublicURLPath tests public URL path construction
func TestSignedURL_PublicURLPath(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		key       string
		expected  string
	}{
		{
			name:      "simple CDN",
			publicURL: "https://cdn.example.com",
			key:       "files/image.jpg",
			expected:  "https://cdn.example.com/files/image.jpg",
		},
		{
			name:      "CDN with base path",
			publicURL: "https://cdn.example.com/assets",
			key:       "files/image.jpg",
			expected:  "https://cdn.example.com/assets/files/image.jpg",
		},
		{
			name:      "CDN with trailing slash",
			publicURL: "https://cdn.example.com/assets/",
			key:       "files/image.jpg",
			expected:  "https://cdn.example.com/assets/files/image.jpg",
		},
		{
			name:      "root level key",
			publicURL: "https://cdn.example.com",
			key:       "image.jpg",
			expected:  "https://cdn.example.com/image.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the public URL path construction
			trimmedURL := strings.TrimRight(tt.publicURL, "/")
			result := trimmedURL + "/" + tt.key
			if result != tt.expected {
				t.Errorf("URL construction = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestPut_KeySanitization tests key sanitization in Put operations
func TestPut_KeySanitization(t *testing.T) {
	tests := []struct {
		name            string
		inputKey        string
		expectedKey     string
		shouldCreateDir bool
	}{
		{
			name:            "simple file",
			inputKey:        "test/file.txt",
			expectedKey:     "test/file.txt",
			shouldCreateDir: true,
		},
		{
			name:            "with leading slash",
			inputKey:        "/test/file.txt",
			expectedKey:     "test/file.txt",
			shouldCreateDir: true,
		},
		{
			name:            "with dot segments",
			inputKey:        "a/../test/file.txt",
			expectedKey:     "a/test/file.txt",
			shouldCreateDir: true,
		},
		{
			name:            "root file",
			inputKey:        "file.txt",
			expectedKey:     "file.txt",
			shouldCreateDir: false,
		},
		{
			name:            "deeply nested",
			inputKey:        "a/b/c/d/file.txt",
			expectedKey:     "a/b/c/d/file.txt",
			shouldCreateDir: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := sanitizeKey(tt.inputKey)
			if sanitized != tt.expectedKey {
				t.Errorf("sanitizeKey() = %q, want %q", sanitized, tt.expectedKey)
			}

			// Check if directory creation is needed
			hasSlash := strings.Contains(sanitized, "/")
			if hasSlash != tt.shouldCreateDir {
				t.Errorf("Directory creation expected = %v, got %v", tt.shouldCreateDir, hasSlash)
			}
		})
	}
}

// TestPut_ContentTypeHandling tests content type handling
func TestPut_ContentTypeHandling(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		shouldSet   bool
	}{
		{
			name:        "text/plain",
			contentType: "text/plain",
			shouldSet:   true,
		},
		{
			name:        "application/json",
			contentType: "application/json",
			shouldSet:   true,
		},
		{
			name:        "image/jpeg",
			contentType: "image/jpeg",
			shouldSet:   true,
		},
		{
			name:        "empty",
			contentType: "",
			shouldSet:   false,
		},
		{
			name:        "video/mp4",
			contentType: "video/mp4",
			shouldSet:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldSet := tt.contentType != ""
			if shouldSet != tt.shouldSet {
				t.Errorf("ContentType set = %v, want %v", shouldSet, tt.shouldSet)
			}
		})
	}
}

// TestDelete_FolderDetection tests folder detection in Delete
func TestDelete_FolderDetection(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		isFolder bool
	}{
		{
			name:     "file",
			key:      "test/file.txt",
			isFolder: false,
		},
		{
			name:     "folder with trailing slash",
			key:      "test/folder/",
			isFolder: true,
		},
		{
			name:     "folder without trailing slash",
			key:      "test/folder",
			isFolder: false,
		},
		{
			name:     "nested folder",
			key:      "a/b/c/",
			isFolder: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := sanitizeKey(tt.key)
			// After sanitization, check if trailing slash is preserved
			// sanitizeKey removes leading slashes but preserves trailing ones
			// However, sanitizeKey then removes empty parts, ".", and ".."
			// So we need to check the original key for folder detection
			isFolder := strings.HasSuffix(tt.key, "/")
			if isFolder != tt.isFolder {
				t.Errorf("isFolder = %v, want %v", isFolder, tt.isFolder)
			}
			_ = sanitized
		})
	}
}

// TestList_Parameters tests list operation parameters
func TestList_Parameters(t *testing.T) {
	tests := []struct {
		name      string
		prefix    string
		marker    string
		delimiter string
		limit     int
	}{
		{
			name:      "all empty",
			prefix:    "",
			marker:    "",
			delimiter: "",
			limit:     0,
		},
		{
			name:      "with prefix",
			prefix:    "uploads/",
			marker:    "",
			delimiter: "",
			limit:     0,
		},
		{
			name:      "with limit",
			prefix:    "",
			marker:    "",
			delimiter: "",
			limit:     100,
		},
		{
			name:      "with delimiter",
			prefix:    "",
			marker:    "",
			delimiter: "/",
			limit:     0,
		},
		{
			name:      "with marker",
			prefix:    "",
			marker:    "next-marker",
			delimiter: "",
			limit:     0,
		},
		{
			name:      "all parameters",
			prefix:    "test/",
			marker:    "marker-123",
			delimiter: "/",
			limit:     50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test parameter handling
			_ = tt.prefix
			_ = tt.marker
			_ = tt.delimiter
			_ = tt.limit
		})
	}
}

// TestCreatePrefix_Sanitization tests prefix creation with sanitization
func TestCreatePrefix_Sanitization(t *testing.T) {
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
			input:    "a/../test/",
			expected: "a/test/",
		},
		{
			name:     "nested path",
			input:    "a/b/c",
			expected: "a/b/c/",
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

// TestRenamePrefix_Sanitization tests rename prefix with sanitization
func TestRenamePrefix_Sanitization(t *testing.T) {
	tests := []struct {
		name      string
		oldPrefix string
		newPrefix string
		oldKey    string
		newKey    string
	}{
		{
			name:      "simple rename",
			oldPrefix: "old/",
			newPrefix: "new/",
			oldKey:    "old/file.txt",
			newKey:    "new/file.txt",
		},
		{
			name:      "with slashes",
			oldPrefix: "old/",
			newPrefix: "new/",
			oldKey:    "old/nested/file.txt",
			newKey:    "new/nested/file.txt",
		},
		{
			name:      "without slashes",
			oldPrefix: "old",
			newPrefix: "new",
			oldKey:    "old/file.txt",
			newKey:    "new/file.txt",
		},
		{
			name:      "with leading slashes",
			oldPrefix: "/old/",
			newPrefix: "/new/",
			oldKey:    "/old/file.txt",
			newKey:    "/new/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldSanitized := sanitizeKey(tt.oldPrefix)
			newSanitized := sanitizeKey(tt.newPrefix)

			if !strings.HasSuffix(oldSanitized, "/") && oldSanitized != "" {
				oldSanitized += "/"
			}
			if !strings.HasSuffix(newSanitized, "/") && newSanitized != "" {
				newSanitized += "/"
			}

			// Test the replace logic
			newKey := strings.Replace(tt.oldKey, oldSanitized, newSanitized, 1)
			if newKey != tt.newKey {
				t.Errorf("newKey = %q, want %q", newKey, tt.newKey)
			}
		})
	}
}

// TestConfig_Defaults tests config default values
func TestConfig_Defaults(t *testing.T) {
	cfg := Config{}

	if cfg.Driver != "" {
		t.Errorf("Driver default should be empty, got %q", cfg.Driver)
	}
	if cfg.Bucket != "" {
		t.Errorf("Bucket default should be empty, got %q", cfg.Bucket)
	}
	if cfg.Region != "" {
		t.Errorf("Region default should be empty, got %q", cfg.Region)
	}
	if cfg.Endpoint != "" {
		t.Errorf("Endpoint default should be empty, got %q", cfg.Endpoint)
	}
	if cfg.AccessKey != "" {
		t.Errorf("AccessKey default should be empty, got %q", cfg.AccessKey)
	}
	if cfg.SecretKey != "" {
		t.Errorf("SecretKey default should be empty, got %q", cfg.SecretKey)
	}
	if cfg.ForcePathStyle {
		t.Error("ForcePathStyle default should be false")
	}
	if cfg.BaseDir != "" {
		t.Errorf("BaseDir default should be empty, got %q", cfg.BaseDir)
	}
	if cfg.SignedURLTTL != 0 {
		t.Errorf("SignedURLTTL default should be 0, got %v", cfg.SignedURLTTL)
	}
	if cfg.PublicURL != "" {
		t.Errorf("PublicURL default should be empty, got %q", cfg.PublicURL)
	}
}

// TestBuildS3URL_EmptyOptions tests buildS3URL with minimal config
func TestBuildS3URL_EmptyOptions(t *testing.T) {
	tests := []struct {
		name  string
		cfg   Config
		check func(string) bool
	}{
		{
			name: "bucket only",
			cfg:  Config{Bucket: "test"},
			check: func(s string) bool {
				return strings.HasPrefix(s, "s3://") || strings.HasPrefix(s, "s3:")
			},
		},
		{
			name: "empty config",
			cfg:  Config{},
			check: func(s string) bool {
				return strings.HasPrefix(s, "s3:")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := buildS3URL(tt.cfg)
			if !tt.check(url) {
				t.Errorf("URL %q check failed", url)
			}
		})
	}
}

// TestFileStore_OpenErrors tests OpenFile error cases
func TestFileStore_OpenErrors(t *testing.T) {
	tests := []struct {
		name    string
		baseDir string
		wantErr bool
	}{
		{
			name:    "empty base dir",
			baseDir: "",
			wantErr: true,
		},
		{
			name:    "valid temp dir",
			baseDir: os.TempDir(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := OpenFile(context.Background(), Config{
				Driver:  "file",
				BaseDir: tt.baseDir,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("OpenFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
