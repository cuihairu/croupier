package objstore

import (
	"os"
	"testing"
)

// TestValidate_ErrorPaths tests all error validation paths
func TestValidate_ErrorPaths(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		wantErr     bool
		errContains string
	}{
		// S3 validation errors
		{
			name:        "S3 missing bucket",
			cfg:         Config{Driver: "s3"},
			wantErr:     true,
			errContains: "bucket required",
		},
		{
			name:    "S3 with bucket only - valid",
			cfg:     Config{Driver: "s3", Bucket: "test"},
			wantErr: false,
		},

		// OSS validation errors
		{
			name: "OSS missing bucket",
			cfg: Config{
				Driver:    "oss",
				Endpoint:  "oss.aliyuncs.com",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr:     true,
			errContains: "bucket required",
		},
		{
			name: "OSS missing endpoint",
			cfg: Config{
				Driver:    "oss",
				Bucket:    "test",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr:     true,
			errContains: "endpoint required",
		},
		{
			name: "OSS missing access key",
			cfg: Config{
				Driver:    "oss",
				Bucket:    "test",
				Endpoint:  "oss.aliyuncs.com",
				SecretKey: "secret",
			},
			wantErr:     true,
			errContains: "access_key/secret_key required",
		},
		{
			name: "OSS missing secret key",
			cfg: Config{
				Driver:    "oss",
				Bucket:    "test",
				Endpoint:  "oss.aliyuncs.com",
				AccessKey: "key",
			},
			wantErr:     true,
			errContains: "access_key/secret_key required",
		},
		{
			name: "OSS missing both credentials",
			cfg: Config{
				Driver:   "oss",
				Bucket:   "test",
				Endpoint: "oss.aliyuncs.com",
			},
			wantErr:     true,
			errContains: "access_key/secret_key required",
		},

		// COS validation errors
		{
			name: "COS missing bucket",
			cfg: Config{
				Driver:    "cos",
				Region:    "ap-guangzhou",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr:     true,
			errContains: "bucket required",
		},
		{
			name: "COS missing both region and endpoint",
			cfg: Config{
				Driver:    "cos",
				Bucket:    "test",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr:     true,
			errContains: "region or endpoint required",
		},
		{
			name: "COS missing credentials",
			cfg: Config{
				Driver: "cos",
				Bucket: "test",
				Region: "ap-guangzhou",
			},
			wantErr:     true,
			errContains: "access_key/secret_key required",
		},
		{
			name: "COS with endpoint only - valid",
			cfg: Config{
				Driver:    "cos",
				Bucket:    "test",
				Endpoint:  "https://cos.ap-guangzhou.myqcloud.com",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: false,
		},

		// File validation errors
		{
			name:        "File missing base dir",
			cfg:         Config{Driver: "file"},
			wantErr:     true,
			errContains: "base_dir required",
		},
		{
			name:    "File with base dir - valid",
			cfg:     Config{Driver: "file", BaseDir: "/tmp"},
			wantErr: false,
		},
		{
			name:    "File with invalid base dir - mkdir fails",
			cfg:     Config{Driver: "file", BaseDir: "/dev/null/invalid/path\000"},
			wantErr: true,
		},

		// Empty/unknown driver
		{
			name:        "Empty driver",
			cfg:         Config{},
			wantErr:     true,
			errContains: "STORAGE_DRIVER not set",
		},
		{
			name:        "Unknown driver",
			cfg:         Config{Driver: "unknown"},
			wantErr:     true,
			errContains: "unknown storage driver",
		},
		{
			name:        "Unknown driver with bucket",
			cfg:         Config{Driver: "minio", Bucket: "test"},
			wantErr:     true,
			errContains: "unknown storage driver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errContains)
				}
			}
		})
	}
}

// TestValidate_DriverCaseInsensitivity tests that driver names are case-insensitive
func TestValidate_DriverCaseInsensitivity(t *testing.T) {
	tests := []struct {
		name    string
		driver  string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "S3 lowercase",
			driver:  "s3",
			cfg:     Config{Driver: "s3", Bucket: "test"},
			wantErr: false,
		},
		{
			name:    "S3 uppercase",
			driver:  "S3",
			cfg:     Config{Driver: "S3", Bucket: "test"},
			wantErr: false,
		},
		{
			name:    "S3 mixed case",
			driver:  "S3",
			cfg:     Config{Driver: "S3", Bucket: "test"},
			wantErr: false,
		},
		{
			name:   "OSS lowercase",
			driver: "oss",
			cfg: Config{
				Driver:    "oss",
				Bucket:    "test",
				Endpoint:  "oss.aliyuncs.com",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: false,
		},
		{
			name:   "OSS uppercase",
			driver: "OSS",
			cfg: Config{
				Driver:    "OSS",
				Bucket:    "test",
				Endpoint:  "oss.aliyuncs.com",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: false,
		},
		{
			name:   "COS lowercase",
			driver: "cos",
			cfg: Config{
				Driver:    "cos",
				Bucket:    "test",
				Region:    "ap-guangzhou",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: false,
		},
		{
			name:   "COS uppercase",
			driver: "COS",
			cfg: Config{
				Driver:    "COS",
				Bucket:    "test",
				Region:    "ap-guangzhou",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: false,
		},
		{
			name:    "File lowercase",
			driver:  "file",
			cfg:     Config{Driver: "file", BaseDir: "/tmp"},
			wantErr: false,
		},
		{
			name:    "File uppercase",
			driver:  "FILE",
			cfg:     Config{Driver: "FILE", BaseDir: "/tmp"},
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

// TestValidate_ValidConfigurations tests valid configurations that should pass
func TestValidate_ValidConfigurations(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "S3 with bucket only",
			cfg:  Config{Driver: "s3", Bucket: "my-bucket"},
		},
		{
			name: "S3 with bucket and region",
			cfg:  Config{Driver: "s3", Bucket: "my-bucket", Region: "us-west-2"},
		},
		{
			name: "S3 with all options",
			cfg: Config{
				Driver:         "s3",
				Bucket:         "my-bucket",
				Region:         "us-west-2",
				Endpoint:       "https://s3.amazonaws.com",
				ForcePathStyle: true,
			},
		},
		{
			name: "OSS complete",
			cfg: Config{
				Driver:    "oss",
				Bucket:    "my-bucket",
				Endpoint:  "oss-cn-hangzhou.aliyuncs.com",
				AccessKey: "key",
				SecretKey: "secret",
			},
		},
		{
			name: "COS with region",
			cfg: Config{
				Driver:    "cos",
				Bucket:    "my-bucket",
				Region:    "ap-guangzhou",
				AccessKey: "key",
				SecretKey: "secret",
			},
		},
		{
			name: "COS with endpoint",
			cfg: Config{
				Driver:    "cos",
				Bucket:    "my-bucket",
				Endpoint:  "https://cos.ap-guangzhou.myqcloud.com",
				AccessKey: "key",
				SecretKey: "secret",
			},
		},
		{
			name: "File with valid base dir",
			cfg:  Config{Driver: "file", BaseDir: os.TempDir()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if err != nil {
				t.Errorf("Validate() returned unexpected error: %v", err)
			}
		})
	}
}

// TestValidate_BucketNames tests various bucket name formats
func TestValidate_BucketNames(t *testing.T) {
	tests := []struct {
		name    string
		bucket  string
		wantErr bool
	}{
		{
			name:    "simple bucket",
			bucket:  "my-bucket",
			wantErr: false,
		},
		{
			name:    "bucket with numbers",
			bucket:  "my-bucket-123",
			wantErr: false,
		},
		{
			name:    "bucket with dots",
			bucket:  "my.bucket.name",
			wantErr: false,
		},
		{
			name:    "empty bucket",
			bucket:  "",
			wantErr: true,
		},
		{
			name:    "bucket with path separator",
			bucket:  "my/bucket",
			wantErr: false, // Validate doesn't check bucket name format
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Driver: "s3",
				Bucket: tt.bucket,
			}
			err := Validate(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidate_CombinedErrors tests configurations with multiple validation errors
func TestValidate_CombinedErrors(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		wantErr     bool
		errContains []string // At least one of these should be in the error
	}{
		{
			name:        "OSS missing everything except driver",
			cfg:         Config{Driver: "oss"},
			wantErr:     true,
			errContains: []string{"bucket", "endpoint", "access_key"},
		},
		{
			name: "COS missing bucket, region, endpoint",
			cfg: Config{
				Driver:    "cos",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr:     true,
			errContains: []string{"bucket"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && len(tt.errContains) > 0 {
				errMsg := err.Error()
				// Check that at least one of the expected strings is in the error
				found := false
				for _, substr := range tt.errContains {
					if containsIgnoreCase(errMsg, substr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error to contain one of %v, got %q", tt.errContains, errMsg)
				}
			}
		})
	}
}

// TestValidate_FileDirCreation tests file driver base dir validation
func TestValidate_FileDirCreation(t *testing.T) {
	tests := []struct {
		name    string
		baseDir string
		wantErr bool
	}{
		{
			name:    "temp dir",
			baseDir: os.TempDir(),
			wantErr: false,
		},
		{
			name:    "current directory",
			baseDir: ".",
			wantErr: false,
		},
		{
			name:    "parent directory",
			baseDir: "..",
			wantErr: false,
		},
		{
			name:    "nested path under temp",
			baseDir: os.TempDir() + "/test-subdir",
			wantErr: false,
		},
		{
			name:    "absolute path",
			baseDir: "/tmp/objstore-test",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Driver:  "file",
				BaseDir: tt.baseDir,
			}
			err := Validate(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidate_ConfigDefaults tests that configuration defaults work with Validate
func TestValidate_ConfigDefaults(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "S3 minimal with defaults",
			cfg: Config{
				Driver: "s3",
				Bucket: "test",
				// ForcePathStyle defaults to false
				// SignedURLTTL defaults to 0
			},
		},
		{
			name: "File minimal with defaults",
			cfg: Config{
				Driver:  "file",
				BaseDir: os.TempDir(),
				// SignedURLTTL defaults to 0 (will be set to 15m in OpenFile)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if err != nil {
				t.Errorf("Validate() returned unexpected error: %v", err)
			}
		})
	}
}

// Helper function for case-insensitive string contains
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			indexOfIgnoreCase(s, substr) >= 0))
}

func indexOfIgnoreCase(s, substr string) int {
	// Simple case-insensitive search
	sLower := toLower(s)
	substrLower := toLower(substr)
	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return i
		}
	}
	return -1
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + 32
		}
		result[i] = c
	}
	return string(result)
}
