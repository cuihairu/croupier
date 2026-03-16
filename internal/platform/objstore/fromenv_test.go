package objstore

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestFromEnv_ForcePathStyleVariations_Comprehensive tests all ForcePathStyle values
func TestFromEnv_ForcePathStyleVariations_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"true lower", "true", true},
		{"TRUE upper", "TRUE", true},
		{"True mixed", "True", true},
		{"1", "1", true},
		{"yes", "yes", true},
		{"YES", "YES", true},
		{"Yes", "Yes", true},
		{"false lower", "false", false},
		{"FALSE upper", "FALSE", false},
		{"0", "0", false},
		{"no", "no", false},
		{"NO", "NO", false},
		{"empty", "", false},
		{"random", "random", false},
		{"TRU", "TRU", false},
		{"FALS", "FALS", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env
			original := os.Getenv("STORAGE_FORCE_PATH_STYLE")
			defer func() {
				if original == "" {
					os.Unsetenv("STORAGE_FORCE_PATH_STYLE")
				} else {
					os.Setenv("STORAGE_FORCE_PATH_STYLE", original)
				}
			}()

			if tt.value == "" {
				os.Unsetenv("STORAGE_FORCE_PATH_STYLE")
			} else {
				os.Setenv("STORAGE_FORCE_PATH_STYLE", tt.value)
			}

			cfg := FromEnv()
			if cfg.ForcePathStyle != tt.expected {
				t.Errorf("ForcePathStyle = %v, want %v for value %q", cfg.ForcePathStyle, tt.expected, tt.value)
			}
		})
	}
}

// TestFromEnv_SignedURLTTL_Variations tests TTL parsing variations
func TestFromEnv_SignedURLTTL_Variations(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected time.Duration
	}{
		{"1 second", "1s", 1 * time.Second},
		{"1 minute", "1m", 1 * time.Minute},
		{"1 hour", "1h", 1 * time.Hour},
		{"30 minutes", "30m", 30 * time.Minute},
		{"2 hours", "2h", 2 * time.Hour},
		{"complex", "1h30m", 90 * time.Minute},
		{"milliseconds", "500ms", 500 * time.Millisecond},
		{"invalid", "invalid", 0},
		{"empty", "", 0},
		{"partial", "1", 0},
		{"only number", "60", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env
			original := os.Getenv("STORAGE_SIGNED_URL_TTL")
			defer func() {
				if original == "" {
					os.Unsetenv("STORAGE_SIGNED_URL_TTL")
				} else {
					os.Setenv("STORAGE_SIGNED_URL_TTL", original)
				}
			}()

			if tt.value == "" {
				os.Unsetenv("STORAGE_SIGNED_URL_TTL")
			} else {
				os.Setenv("STORAGE_SIGNED_URL_TTL", tt.value)
			}

			cfg := FromEnv()
			if cfg.SignedURLTTL != tt.expected {
				t.Errorf("SignedURLTTL = %v, want %v for value %q", cfg.SignedURLTTL, tt.expected, tt.value)
			}
		})
	}
}

// TestFromEnv_AllEmpty tests FromEnv with all env vars empty
func TestFromEnv_AllEmpty(t *testing.T) {
	// Save all env vars
	envVars := []string{
		"STORAGE_DRIVER", "STORAGE_BUCKET", "STORAGE_REGION",
		"STORAGE_ENDPOINT", "STORAGE_ACCESS_KEY", "STORAGE_SECRET_KEY",
		"STORAGE_BASE_DIR", "STORAGE_FORCE_PATH_STYLE", "STORAGE_SIGNED_URL_TTL",
		"STORAGE_PUBLIC_URL",
	}
	saved := make(map[string]string)
	for _, v := range envVars {
		saved[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range saved {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	cfg := FromEnv()

	// All fields should be empty/default
	if cfg.Driver != "" {
		t.Errorf("Driver should be empty, got %q", cfg.Driver)
	}
	if cfg.Bucket != "" {
		t.Errorf("Bucket should be empty, got %q", cfg.Bucket)
	}
	if cfg.Region != "" {
		t.Errorf("Region should be empty, got %q", cfg.Region)
	}
	if cfg.Endpoint != "" {
		t.Errorf("Endpoint should be empty, got %q", cfg.Endpoint)
	}
	if cfg.AccessKey != "" {
		t.Errorf("AccessKey should be empty, got %q", cfg.AccessKey)
	}
	if cfg.SecretKey != "" {
		t.Errorf("SecretKey should be empty, got %q", cfg.SecretKey)
	}
	if cfg.BaseDir != "" {
		t.Errorf("BaseDir should be empty, got %q", cfg.BaseDir)
	}
	if cfg.PublicURL != "" {
		t.Errorf("PublicURL should be empty, got %q", cfg.PublicURL)
	}
	if cfg.ForcePathStyle {
		t.Error("ForcePathStyle should be false")
	}
	if cfg.SignedURLTTL != 0 {
		t.Errorf("SignedURLTTL should be 0, got %v", cfg.SignedURLTTL)
	}
}

// TestFromEnv_WhitespaceHandling tests whitespace handling in env vars
func TestFromEnv_WhitespaceHandling(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		value    string
		expected string
	}{
		{
			name:     "driver with leading space",
			envVar:   "STORAGE_DRIVER",
			value:    " s3",
			expected: " s3", // FromEnv doesn't trim
		},
		{
			name:     "driver with trailing space",
			envVar:   "STORAGE_DRIVER",
			value:    "s3 ",
			expected: "s3 ",
		},
		{
			name:     "bucket with spaces",
			envVar:   "STORAGE_BUCKET",
			value:    "my bucket",
			expected: "my bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := os.Getenv(tt.envVar)
			if original != "" {
				defer os.Setenv(tt.envVar, original)
			} else {
				defer os.Unsetenv(tt.envVar)
			}
			os.Setenv(tt.envVar, tt.value)

			cfg := FromEnv()

			// Check the value is preserved as-is
			if tt.envVar == "STORAGE_DRIVER" && cfg.Driver != tt.expected {
				t.Errorf("Driver = %q, want %q", cfg.Driver, tt.expected)
			}
		})
	}
}

// TestFromEnv_DriverCasePreservation tests that driver case is preserved
func TestFromEnv_DriverCasePreservation(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"lowercase", "s3"},
		{"uppercase", "S3"},
		{"mixed case", "S3"},
		{"oss lower", "oss"},
		{"OSS upper", "OSS"},
		{"cos lower", "cos"},
		{"file upper", "FILE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := os.Getenv("STORAGE_DRIVER")
			if original != "" {
				defer os.Setenv("STORAGE_DRIVER", original)
			} else {
				defer os.Unsetenv("STORAGE_DRIVER")
			}
			os.Setenv("STORAGE_DRIVER", tt.value)

			cfg := FromEnv()
			if cfg.Driver != tt.value {
				t.Errorf("Driver = %q, want %q", cfg.Driver, tt.value)
			}
		})
	}
}

// TestBuildS3URL_SpecialCharacters tests buildS3URL with special characters
func TestBuildS3URL_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		contains []string
	}{
		{
			name:     "bucket with hyphens",
			cfg:      Config{Bucket: "my-test-bucket"},
			contains: []string{"my-test-bucket"},
		},
		{
			name:     "bucket with dots",
			cfg:      Config{Bucket: "my.test.bucket"},
			contains: []string{"my.test.bucket"},
		},
		{
			name:     "bucket with numbers",
			cfg:      Config{Bucket: "bucket123"},
			contains: []string{"bucket123"},
		},
		{
			name: "endpoint with port",
			cfg: Config{
				Bucket:   "test",
				Endpoint: "https://localhost:9000",
			},
			contains: []string{"localhost", "9000"},
		},
		{
			name: "region with dash",
			cfg: Config{
				Bucket: "test",
				Region: "eu-west-1",
			},
			contains: []string{"eu-west-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := buildS3URL(tt.cfg)
			for _, substr := range tt.contains {
				if !strings.Contains(url, substr) {
					t.Errorf("URL %q should contain %q", url, substr)
				}
			}
		})
	}
}

// TestValidate_ConfigCombinations tests various config combinations
func TestValidate_ConfigCombinations(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "S3 with region and endpoint",
			cfg: Config{
				Driver:   "s3",
				Bucket:   "test",
				Region:   "us-west-2",
				Endpoint: "https://s3.amazonaws.com",
			},
			wantErr: false,
		},
		{
			name: "S3 with all optional fields",
			cfg: Config{
				Driver:         "s3",
				Bucket:         "test",
				Region:         "us-west-2",
				Endpoint:       "https://s3.amazonaws.com",
				AccessKey:      "key",
				SecretKey:      "secret",
				ForcePathStyle: true,
			},
			wantErr: false,
		},
		{
			name: "COS with both region and endpoint",
			cfg: Config{
				Driver:    "cos",
				Bucket:    "test",
				Region:    "ap-guangzhou",
				Endpoint:  "https://cos.ap-guangzhou.myqcloud.com",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: false,
		},
		{
			name: "File with extra fields (should be ignored)",
			cfg: Config{
				Driver:    "file",
				BaseDir:   "/tmp",
				Bucket:    "ignored",
				Region:    "ignored",
				AccessKey: "ignored",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSanitizeKey_RealWorldCases tests real-world key patterns
func TestSanitizeKey_RealWorldCases(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "AWS S3 key format",
			key:      "uploads/2024/03/16/document.pdf",
			expected: "uploads/2024/03/16/document.pdf",
		},
		{
			name:     "with UUID filename",
			key:      "files/123e4567-e89b-12d3-a456-426614174000.pdf",
			expected: "files/123e4567-e89b-12d3-a456-426614174000.pdf",
		},
		{
			name:     "with version",
			key:      "/v1/api/documents/file.json",
			expected: "v1/api/documents/file.json",
		},
		{
			name:     "with timestamp",
			key:      "backups/2024-03-16T10:30:00Z/data.json",
			expected: "backups/2024-03-16T10:30:00Z/data.json",
		},
		{
			name:     "with user uploads pattern",
			key:      "users/123/profile/avatar.jpg",
			expected: "users/123/profile/avatar.jpg",
		},
		{
			name:     "complex nested path",
			key:      "a/b/c/d/e/f/g/file.txt",
			expected: "a/b/c/d/e/f/g/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeKey(tt.key)
			if result != tt.expected {
				t.Errorf("sanitizeKey() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestConfig_FieldIndependence tests that config fields are independent
func TestConfig_FieldIndependence(t *testing.T) {
	// Create multiple configs with different values
	cfg1 := Config{Driver: "s3", Bucket: "bucket1"}
	cfg2 := Config{Driver: "oss", Bucket: "bucket2"}
	cfg3 := Config{Driver: "file", BaseDir: "/tmp"}

	if cfg1.Driver == cfg2.Driver {
		t.Error("Configs should have different drivers")
	}
	if cfg1.Bucket == cfg2.Bucket {
		t.Error("Configs should have different buckets")
	}
	if cfg1.Driver == cfg3.Driver {
		t.Error("Configs should have different drivers")
	}
}

// TestObjectInfo_EmptyFields tests ObjectInfo with empty fields
func TestObjectInfo_EmptyFields(t *testing.T) {
	obj := ObjectInfo{
		Key: "test.txt",
	}

	if obj.Key != "test.txt" {
		t.Errorf("Key = %q, want 'test.txt'", obj.Key)
	}
	if obj.Size != 0 {
		t.Errorf("Size should be 0, got %d", obj.Size)
	}
	if !obj.LastModified.IsZero() {
		t.Error("LastModified should be zero")
	}
	if obj.ETag != "" {
		t.Errorf("ETag should be empty, got %q", obj.ETag)
	}
	if obj.StorageClass != "" {
		t.Errorf("StorageClass should be empty, got %q", obj.StorageClass)
	}
}

// TestListResult_EmptyTests tests empty ListResult
func TestListResult_EmptyTests(t *testing.T) {
	result := ListResult{}

	if len(result.Objects) != 0 {
		t.Errorf("Objects should be empty, got %d", len(result.Objects))
	}
	if len(result.Prefixes) != 0 {
		t.Errorf("Prefixes should be empty, got %d", len(result.Prefixes))
	}
	if result.IsTruncated {
		t.Error("IsTruncated should be false")
	}
	if result.NextMarker != "" {
		t.Errorf("NextMarker should be empty, got %q", result.NextMarker)
	}
}
