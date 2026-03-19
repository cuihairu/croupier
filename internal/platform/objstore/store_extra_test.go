package objstore

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestValidate_Comprehensive tests all validation paths
func TestValidate_Comprehensive(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "S3 with only bucket - valid (credentials via env/IAM)",
			cfg: Config{
				Driver: "s3",
				Bucket: "test-bucket",
			},
			wantErr: false,
		},
		{
			name: "S3 without bucket",
			cfg: Config{
				Driver: "s3",
			},
			wantErr: true,
			errMsg:  "bucket required",
		},
		{
			name: "S3 with all options",
			cfg: Config{
				Driver:         "s3",
				Bucket:         "test-bucket",
				Region:         "us-west-2",
				Endpoint:       "https://s3.amazonaws.com",
				ForcePathStyle: true,
			},
			wantErr: false,
		},
		{
			name: "OSS complete config",
			cfg: Config{
				Driver:    "oss",
				Bucket:    "test-bucket",
				Endpoint:  "oss-cn-hangzhou.aliyuncs.com",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: false,
		},
		{
			name: "OSS missing bucket",
			cfg: Config{
				Driver:    "oss",
				Endpoint:  "oss-cn-hangzhou.aliyuncs.com",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: true,
			errMsg:  "bucket required",
		},
		{
			name: "OSS missing endpoint",
			cfg: Config{
				Driver:    "oss",
				Bucket:    "test-bucket",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: true,
			errMsg:  "endpoint required",
		},
		{
			name: "OSS missing credentials",
			cfg: Config{
				Driver:   "oss",
				Bucket:   "test-bucket",
				Endpoint: "oss-cn-hangzhou.aliyuncs.com",
			},
			wantErr: true,
			errMsg:  "access_key/secret_key required",
		},
		{
			name: "COS with region",
			cfg: Config{
				Driver:    "cos",
				Bucket:    "test-bucket",
				Region:    "ap-guangzhou",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: false,
		},
		{
			name: "COS with endpoint",
			cfg: Config{
				Driver:    "cos",
				Bucket:    "test-bucket",
				Endpoint:  "https://cos.ap-guangzhou.myqcloud.com",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: false,
		},
		{
			name: "COS missing both region and endpoint",
			cfg: Config{
				Driver:    "cos",
				Bucket:    "test-bucket",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: true,
			errMsg:  "region or endpoint required",
		},
		{
			name: "File with base dir",
			cfg: Config{
				Driver:  "file",
				BaseDir: "/tmp/test",
			},
			wantErr: false,
		},
		{
			name:    "Empty driver",
			cfg:     Config{},
			wantErr: true,
			errMsg:  "STORAGE_DRIVER not set",
		},
		{
			name: "Unknown driver",
			cfg: Config{
				Driver: "unknown-driver",
			},
			wantErr: true,
			errMsg:  "unknown storage driver",
		},
		{
			name: "Driver case insensitive",
			cfg: Config{
				Driver: "S3",
				Bucket: "test-bucket",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errMsg)) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}

// TestFromEnv_PublicURL tests PUBLIC_URL environment variable
func TestFromEnv_PublicURL(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		expected  string
	}{
		{
			name:      "with public URL",
			publicURL: "https://cdn.example.com",
			expected:  "https://cdn.example.com",
		},
		{
			name:      "with public URL and path",
			publicURL: "https://cdn.example.com/uploads",
			expected:  "https://cdn.example.com/uploads",
		},
		{
			name:      "empty public URL",
			publicURL: "",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set just the public URL env
			original := setEnv("STORAGE_PUBLIC_URL", tt.publicURL)
			defer restoreEnv(original)

			cfg := FromEnv()
			if cfg.PublicURL != tt.expected {
				t.Errorf("PublicURL = %q, want %q", cfg.PublicURL, tt.expected)
			}
		})
	}
}

// TestStore_InterfaceImplementation verifies all stores implement Store interface
func TestStore_InterfaceImplementation(t *testing.T) {
	// Verify all store types implement the Store interface
	var _ Store = &fileStore{}
	var _ Store = &s3Store{}
	var _ Store = &ossStore{}
	var _ Store = &cosStore{}
}

// TestObjectInfo_Structure tests ObjectInfo structure fields
func TestObjectInfo_Structure(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		obj  ObjectInfo
	}{
		{
			name: "all fields set",
			obj: ObjectInfo{
				Key:          "test/file.txt",
				Size:         1024,
				LastModified: now,
				ETag:         "abc123",
				StorageClass: "STANDARD",
			},
		},
		{
			name: "minimal object",
			obj: ObjectInfo{
				Key: "file.txt",
			},
		},
		{
			name: "object with zero size",
			obj: ObjectInfo{
				Key:  "empty.txt",
				Size: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.obj.Key == "" {
				t.Error("ObjectInfo.Key should not be empty")
			}
			// Verify fields can be accessed
			_ = tt.obj.Size
			_ = tt.obj.LastModified
			_ = tt.obj.ETag
			_ = tt.obj.StorageClass
		})
	}
}

// TestListResult_Structure tests ListResult structure
func TestListResult_Structure(t *testing.T) {
	tests := []struct {
		name   string
		result ListResult
	}{
		{
			name: "empty result",
			result: ListResult{
				Objects:     []ObjectInfo{},
				Prefixes:    []string{},
				IsTruncated: false,
				NextMarker:  "",
			},
		},
		{
			name: "result with objects",
			result: ListResult{
				Objects: []ObjectInfo{
					{Key: "file1.txt", Size: 100},
					{Key: "file2.txt", Size: 200},
				},
				Prefixes:    []string{"dir1/"},
				IsTruncated: true,
				NextMarker:  "file2.txt",
			},
		},
		{
			name: "result with only prefixes",
			result: ListResult{
				Objects:     []ObjectInfo{},
				Prefixes:    []string{"a/", "b/"},
				IsTruncated: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify structure can be accessed
			_ = len(tt.result.Objects)
			_ = len(tt.result.Prefixes)
			_ = tt.result.IsTruncated
			_ = tt.result.NextMarker
		})
	}
}

// TestReadSeeker_Interface verifies ReadSeeker interface requirements
func TestReadSeeker_Interface(t *testing.T) {
	// strings.NewReader implements ReadSeeker
	var rs ReadSeeker = strings.NewReader("test")

	// Verify required methods exist
	_, _ = rs.Read(nil)
	_, _ = rs.Seek(0, 0)
}

// TestConfig_AllFields tests Config field handling
func TestConfig_AllFields(t *testing.T) {
	cfg := Config{
		Driver:         "s3",
		Bucket:         "test-bucket",
		Region:         "us-west-2",
		Endpoint:       "https://s3.amazonaws.com",
		AccessKey:      "key",
		SecretKey:      "secret",
		ForcePathStyle: true,
		BaseDir:        "/tmp/storage",
		SignedURLTTL:   time.Hour,
		PublicURL:      "https://cdn.example.com",
	}

	// Verify all fields are set
	if cfg.Driver != "s3" {
		t.Errorf("Driver = %q, want 's3'", cfg.Driver)
	}
	if cfg.Bucket != "test-bucket" {
		t.Errorf("Bucket = %q, want 'test-bucket'", cfg.Bucket)
	}
	if cfg.Region != "us-west-2" {
		t.Errorf("Region = %q, want 'us-west-2'", cfg.Region)
	}
	if cfg.Endpoint != "https://s3.amazonaws.com" {
		t.Errorf("Endpoint = %q, want 'https://s3.amazonaws.com'", cfg.Endpoint)
	}
	if cfg.AccessKey != "key" {
		t.Errorf("AccessKey = %q, want 'key'", cfg.AccessKey)
	}
	if cfg.SecretKey != "secret" {
		t.Errorf("SecretKey = %q, want 'secret'", cfg.SecretKey)
	}
	if !cfg.ForcePathStyle {
		t.Error("ForcePathStyle should be true")
	}
	if cfg.BaseDir != "/tmp/storage" {
		t.Errorf("BaseDir = %q, want '/tmp/storage'", cfg.BaseDir)
	}
	if cfg.SignedURLTTL != time.Hour {
		t.Errorf("SignedURLTTL = %v, want 1h", cfg.SignedURLTTL)
	}
	if cfg.PublicURL != "https://cdn.example.com" {
		t.Errorf("PublicURL = %q, want 'https://cdn.example.com'", cfg.PublicURL)
	}
}

// TestBuildS3URL_AllOptions tests buildS3URL with various option combinations
func TestBuildS3URL_AllOptions(t *testing.T) {
	tests := []struct {
		name   string
		cfg    Config
		checks map[string]bool
	}{
		{
			name: "with region only",
			cfg: Config{
				Bucket: "my-bucket",
				Region: "us-west-2",
			},
			checks: map[string]bool{
				"region": true,
			},
		},
		{
			name: "with endpoint only",
			cfg: Config{
				Bucket:   "my-bucket",
				Endpoint: "https://s3.amazonaws.com",
			},
			checks: map[string]bool{
				"endpoint": true,
			},
		},
		{
			name: "with force path style",
			cfg: Config{
				Bucket:         "my-bucket",
				ForcePathStyle: true,
			},
			checks: map[string]bool{
				"forcePathStyle": true,
			},
		},
		{
			name: "with all options",
			cfg: Config{
				Bucket:         "my-bucket",
				Region:         "us-west-2",
				Endpoint:       "https://custom.endpoint.com",
				ForcePathStyle: true,
			},
			checks: map[string]bool{
				"region":         true,
				"endpoint":       true,
				"forcePathStyle": true,
			},
		},
		{
			name: "bucket name with dots",
			cfg: Config{
				Bucket: "my.bucket.name",
				Region: "us-west-2",
			},
			checks: map[string]bool{
				"has_bucket": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := buildS3URL(tt.cfg)

			// Basic URL structure check
			if !strings.HasPrefix(url, "s3://") && !strings.HasPrefix(url, "s3:") {
				t.Errorf("URL should start with s3://, got %q", url)
			}

			// Check for bucket name
			if tt.checks["has_bucket"] && !strings.Contains(url, tt.cfg.Bucket) {
				t.Errorf("URL should contain bucket name %q", tt.cfg.Bucket)
			}

			// Check query parameters
			if tt.checks["region"] && !strings.Contains(url, "region=") {
				t.Error("URL should contain region parameter")
			}
			if tt.checks["endpoint"] && !strings.Contains(url, "endpoint=") {
				t.Error("URL should contain endpoint parameter")
			}
			if tt.checks["forcePathStyle"] && !strings.Contains(url, "s3ForcePathStyle") {
				t.Error("URL should contain s3ForcePathStyle parameter")
			}
		})
	}
}

// TestSanitizeKey_EdgeCases tests edge cases for key sanitization
func TestSanitizeKey_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "only dots - not filtered by sanitizeKey",
			key:      "...",
			expected: "...",
		},
		{
			name:     "only slashes",
			key:      "///",
			expected: "",
		},
		{
			name:     "mixed dots and slashes",
			key:      "./././",
			expected: "",
		},
		{
			name:     "dot segments only",
			key:      "./.././",
			expected: "",
		},
		{
			name:     "single dot",
			key:      ".",
			expected: "",
		},
		{
			name:     "double dot",
			key:      "..",
			expected: "",
		},
		{
			name:     "valid segments with dot segments",
			key:      "a/./b/../c",
			expected: "a/b/c",
		},
		{
			name:     "unicode characters",
			key:      "test/文件/路径",
			expected: "test/文件/路径",
		},
		{
			name:     "spaces in path",
			key:      "test/my file/name",
			expected: "test/my file/name",
		},
		{
			name:     "special characters",
			key:      "test/file-name_123.txt",
			expected: "test/file-name_123.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeKey(tt.key)
			if result != tt.expected {
				t.Errorf("sanitizeKey(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

// TestFromEnv_AllCombinations tests environment variable combinations
func TestFromEnv_AllCombinations(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		check func(*testing.T, Config)
	}{
		{
			name: "driver only",
			env: map[string]string{
				"STORAGE_DRIVER": "s3",
			},
			check: func(t *testing.T, c Config) {
				if c.Driver != "s3" {
					t.Errorf("Driver = %q, want 's3'", c.Driver)
				}
			},
		},
		{
			name: "S3 config",
			env: map[string]string{
				"STORAGE_DRIVER":   "s3",
				"STORAGE_BUCKET":   "test-bucket",
				"STORAGE_REGION":   "us-west-2",
				"STORAGE_ENDPOINT": "https://s3.amazonaws.com",
			},
			check: func(t *testing.T, c Config) {
				if c.Driver != "s3" {
					t.Errorf("Driver = %q, want 's3'", c.Driver)
				}
				if c.Bucket != "test-bucket" {
					t.Errorf("Bucket = %q, want 'test-bucket'", c.Bucket)
				}
				if c.Region != "us-west-2" {
					t.Errorf("Region = %q, want 'us-west-2'", c.Region)
				}
			},
		},
		{
			name: "File config",
			env: map[string]string{
				"STORAGE_DRIVER":   "file",
				"STORAGE_BASE_DIR": "/tmp/storage",
			},
			check: func(t *testing.T, c Config) {
				if c.Driver != "file" {
					t.Errorf("Driver = %q, want 'file'", c.Driver)
				}
				if c.BaseDir != "/tmp/storage" {
					t.Errorf("BaseDir = %q, want '/tmp/storage'", c.BaseDir)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env
			saved := saveAllEnv()
			defer restoreAllEnv(saved)

			// Set test env vars
			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			cfg := FromEnv()
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

// Helper functions for env management
type envSnapshot map[string]string

func saveAllEnv() envSnapshot {
	envVars := []string{
		"STORAGE_DRIVER", "STORAGE_BUCKET", "STORAGE_REGION",
		"STORAGE_ENDPOINT", "STORAGE_ACCESS_KEY", "STORAGE_SECRET_KEY",
		"STORAGE_BASE_DIR", "STORAGE_FORCE_PATH_STYLE", "STORAGE_SIGNED_URL_TTL",
		"STORAGE_PUBLIC_URL",
	}
	saved := make(envSnapshot)
	for _, v := range envVars {
		saved[v] = os.Getenv(v)
	}
	return saved
}

func restoreAllEnv(saved envSnapshot) {
	for k, v := range saved {
		if v == "" {
			os.Unsetenv(k)
		} else {
			os.Setenv(k, v)
		}
	}
}

func setEnv(key, value string) envSnapshot {
	original := os.Getenv(key)
	if value == "" {
		os.Unsetenv(key)
	} else {
		os.Setenv(key, value)
	}
	return envSnapshot{key: original}
}

func restoreEnv(saved envSnapshot) {
	for k, v := range saved {
		if v == "" {
			os.Unsetenv(k)
		} else {
			os.Setenv(k, v)
		}
	}
}
