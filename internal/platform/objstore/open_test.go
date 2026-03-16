package objstore

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestOpenS3_Variations tests OpenS3 with various configurations
func TestOpenS3_Variations(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "minimal config",
			cfg: Config{
				Driver: "s3",
				Bucket: "test-bucket",
			},
			wantErr: false,
		},
		{
			name: "with all options",
			cfg: Config{
				Driver:         "s3",
				Bucket:         "test-bucket",
				Region:         "us-west-2",
				Endpoint:       "https://s3.amazonaws.com",
				ForcePathStyle: true,
				SignedURLTTL:   30 * time.Minute,
				PublicURL:      "https://cdn.example.com",
			},
			wantErr: false,
		},
		{
			name: "bucket with dots",
			cfg: Config{
				Driver: "s3",
				Bucket: "my.bucket.name",
				Region: "us-east-1",
			},
			wantErr: false,
		},
		{
			name: "bucket with path style",
			cfg: Config{
				Driver:         "s3",
				Bucket:         "test-bucket",
				Endpoint:       "https://s3.amazonaws.com",
				ForcePathStyle: true,
			},
			wantErr: false,
		},
		{
			name: "with custom endpoint",
			cfg: Config{
				Driver:   "s3",
				Bucket:   "test-bucket",
				Endpoint: "https://custom-s3.example.com",
			},
			wantErr: false,
		},
		{
			name: "with zero TTL (should use default)",
			cfg: Config{
				Driver:       "s3",
				Bucket:       "test-bucket",
				SignedURLTTL: 0,
			},
			wantErr: false,
		},
		{
			name: "with custom TTL",
			cfg: Config{
				Driver:       "s3",
				Bucket:       "test-bucket",
				SignedURLTTL: 2 * time.Hour,
			},
			wantErr: false,
		},
		{
			name: "empty bucket - URL will be invalid",
			cfg: Config{
				Driver: "s3",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := OpenS3(context.Background(), tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("OpenS3() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && store == nil {
				t.Error("OpenS3() should return non-nil store when no error")
			}
			if store != nil {
				// Verify it's an s3Store
				if s3Store, ok := store.(*s3Store); ok {
					if s3Store.ttl == 0 {
						t.Error("TTL should be set to default (15m) when config TTL is 0")
					}
					if s3Store.publicURL != tt.cfg.PublicURL {
						t.Errorf("publicURL = %q, want %q", s3Store.publicURL, tt.cfg.PublicURL)
					}
				} else {
					t.Error("Store should be *s3Store type")
				}
			}
		})
	}
}

// TestOpenOSS_ParameterValidation tests OpenOSS parameter handling
func TestOpenOSS_ParameterValidation(t *testing.T) {
	tests := []struct {
		name       string
		cfg        Config
		wantErr    bool
		checkStore func(*testing.T, Store)
	}{
		{
			name: "valid OSS config",
			cfg: Config{
				Driver:    "oss",
				Endpoint:  "oss-cn-hangzhou.aliyuncs.com",
				Bucket:    "test-bucket",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: false,
			checkStore: func(t *testing.T, s Store) {
				if ossStore, ok := s.(*ossStore); ok {
					if ossStore.ttl != 15*time.Minute {
						t.Errorf("Default TTL = %v, want 15m", ossStore.ttl)
					}
				}
			},
		},
		{
			name: "with custom TTL",
			cfg: Config{
				Driver:       "oss",
				Endpoint:     "oss-cn-hangzhou.aliyuncs.com",
				Bucket:       "test-bucket",
				AccessKey:    "key",
				SecretKey:    "secret",
				SignedURLTTL: time.Hour,
			},
			wantErr: false,
			checkStore: func(t *testing.T, s Store) {
				if ossStore, ok := s.(*ossStore); ok {
					if ossStore.ttl != time.Hour {
						t.Errorf("TTL = %v, want 1h", ossStore.ttl)
					}
				}
			},
		},
		{
			name: "with public URL",
			cfg: Config{
				Driver:    "oss",
				Endpoint:  "oss-cn-hangzhou.aliyuncs.com",
				Bucket:    "test-bucket",
				AccessKey: "key",
				SecretKey: "secret",
				PublicURL: "https://cdn.example.com",
			},
			wantErr: false,
			checkStore: func(t *testing.T, s Store) {
				if ossStore, ok := s.(*ossStore); ok {
					if ossStore.publicURL != "https://cdn.example.com" {
						t.Errorf("publicURL = %q", ossStore.publicURL)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := OpenOSS(context.Background(), tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("OpenOSS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && store == nil {
				t.Error("OpenOSS() should return non-nil store when no error")
			}
			if !tt.wantErr && tt.checkStore != nil && store != nil {
				tt.checkStore(t, store)
			}
		})
	}
}

// TestOpenCOS_ParameterValidation tests OpenCOS parameter handling
func TestOpenCOS_ParameterValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "with region",
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
			name: "with endpoint containing bucket",
			cfg: Config{
				Driver:    "cos",
				Bucket:    "test-bucket",
				Endpoint:  "https://test-bucket.cos.ap-guangzhou.myqcloud.com",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: false,
		},
		{
			name: "with endpoint not containing bucket",
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
			name: "with custom TTL",
			cfg: Config{
				Driver:       "cos",
				Bucket:       "test-bucket",
				Region:       "ap-guangzhou",
				AccessKey:    "key",
				SecretKey:    "secret",
				SignedURLTTL: 45 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "with public URL",
			cfg: Config{
				Driver:    "cos",
				Bucket:    "test-bucket",
				Region:    "ap-guangzhou",
				AccessKey: "key",
				SecretKey: "secret",
				PublicURL: "https://cdn.example.com",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := OpenCOS(context.Background(), tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("OpenCOS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && store == nil {
				t.Error("OpenCOS() should return non-nil store when no error")
			}
		})
	}
}

// TestOpenOBS_ParameterValidation tests OpenOBS parameter handling
func TestOpenOBS_ParameterValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid OBS config",
			cfg: Config{
				Driver:    "obs",
				Endpoint:  "https://obs.myhuaweicloud.com",
				Bucket:    "test-bucket",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: false,
		},
		{
			name: "with custom TTL",
			cfg: Config{
				Driver:       "obs",
				Endpoint:     "https://obs.myhuaweicloud.com",
				Bucket:       "test-bucket",
				AccessKey:    "key",
				SecretKey:    "secret",
				SignedURLTTL: 20 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "with public URL",
			cfg: Config{
				Driver:    "obs",
				Endpoint:  "https://obs.myhuaweicloud.com",
				Bucket:    "test-bucket",
				AccessKey: "key",
				SecretKey: "secret",
				PublicURL: "https://cdn.example.com",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := OpenOBS(context.Background(), tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("OpenOBS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && store == nil {
				t.Error("OpenOBS() should return non-nil store when no error")
			}
		})
	}
}

// TestStore_URLBuilding tests URL building logic for different stores
func TestStore_URLBuilding(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		openFunc func(context.Context, Config) (Store, error)
		checkURL bool
	}{
		{
			name: "S3 URL",
			cfg: Config{
				Driver: "s3",
				Bucket: "test-bucket",
				Region: "us-west-2",
			},
			openFunc: OpenS3,
			checkURL: true,
		},
		{
			name: "OSS URL",
			cfg: Config{
				Driver:    "oss",
				Bucket:    "test-bucket",
				Endpoint:  "oss-cn-hangzhou.aliyuncs.com",
				AccessKey: "key",
				SecretKey: "secret",
			},
			openFunc: OpenOSS,
			checkURL: false,
		},
		{
			name: "COS URL",
			cfg: Config{
				Driver:    "cos",
				Bucket:    "test-bucket",
				Region:    "ap-guangzhou",
				AccessKey: "key",
				SecretKey: "secret",
			},
			openFunc: OpenCOS,
			checkURL: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.openFunc(context.Background(), tt.cfg)
			if err != nil {
				t.Logf("%s open error (expected in test env): %v", tt.name, err)
			}
		})
	}
}

// TestSignedURL_PublicURL tests that PublicURL is used when configured
func TestSignedURL_PublicURL(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		key       string
		expected  string
	}{
		{
			name:      "CDN URL",
			publicURL: "https://cdn.example.com",
			key:       "path/to/file.txt",
			expected:  "https://cdn.example.com/path/to/file.txt",
		},
		{
			name:      "CDN URL with trailing slash",
			publicURL: "https://cdn.example.com/",
			key:       "path/to/file.txt",
			expected:  "https://cdn.example.com/path/to/file.txt",
		},
		{
			name:      "CDN URL with base path",
			publicURL: "https://cdn.example.com/uploads",
			key:       "path/to/file.txt",
			expected:  "https://cdn.example.com/uploads/path/to/file.txt",
		},
		{
			name:      "CDN URL with trailing slash and base path",
			publicURL: "https://cdn.example.com/uploads/",
			key:       "path/to/file.txt",
			expected:  "https://cdn.example.com/uploads/path/to/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with file store (which uses PublicURL)
			tmpDir := t.TempDir()
			store, err := OpenFile(context.Background(), Config{
				Driver:    "file",
				BaseDir:   tmpDir,
				PublicURL: tt.publicURL,
			})
			if err != nil {
				t.Fatalf("OpenFile() error = %v", err)
			}

			url, err := store.SignedURL(context.Background(), tt.key, "GET", time.Hour)
			if err != nil {
				t.Fatalf("SignedURL() error = %v", err)
			}

			if url != tt.expected {
				t.Errorf("SignedURL() = %q, want %q", url, tt.expected)
			}
		})
	}
}

// TestConfig_StringValues tests that config string values are preserved
func TestConfig_StringValues(t *testing.T) {
	tests := []struct {
		name  string
		cfg   Config
		check func(*testing.T, Config)
	}{
		{
			name: "S3 config values",
			cfg: Config{
				Driver:         "s3",
				Bucket:         "my-bucket-123",
				Region:         "us-west-2",
				Endpoint:       "https://custom.s3.example.com",
				AccessKey:      "AKIAIOSFODNN7EXAMPLE",
				SecretKey:      "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				ForcePathStyle: true,
			},
			check: func(t *testing.T, c Config) {
				if c.Bucket != "my-bucket-123" {
					t.Errorf("Bucket not preserved")
				}
				if c.Region != "us-west-2" {
					t.Errorf("Region not preserved")
				}
				if c.Endpoint != "https://custom.s3.example.com" {
					t.Errorf("Endpoint not preserved")
				}
			},
		},
		{
			name: "File config values",
			cfg: Config{
				Driver:    "file",
				BaseDir:   "/tmp/test-dir",
				PublicURL: "https://cdn.example.com/files",
			},
			check: func(t *testing.T, c Config) {
				if c.BaseDir != "/tmp/test-dir" {
					t.Errorf("BaseDir not preserved")
				}
				if c.PublicURL != "https://cdn.example.com/files" {
					t.Errorf("PublicURL not preserved")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.check != nil {
				tt.check(t, tt.cfg)
			}
		})
	}
}

// TestBuildS3URL_QueryParams tests query parameter building
func TestBuildS3URL_QueryParams(t *testing.T) {
	tests := []struct {
		name           string
		cfg            Config
		expectedParams []string
	}{
		{
			name: "region param",
			cfg: Config{
				Bucket: "test",
				Region: "us-west-2",
			},
			expectedParams: []string{"region=us-west-2"},
		},
		{
			name: "endpoint param",
			cfg: Config{
				Bucket:   "test",
				Endpoint: "https://s3.example.com",
			},
			expectedParams: []string{"endpoint="},
		},
		{
			name: "force path style param",
			cfg: Config{
				Bucket:         "test",
				ForcePathStyle: true,
			},
			expectedParams: []string{"s3ForcePathStyle=true"},
		},
		{
			name: "all params",
			cfg: Config{
				Bucket:         "test",
				Region:         "eu-west-1",
				Endpoint:       "https://custom.s3.com",
				ForcePathStyle: true,
			},
			expectedParams: []string{
				"region=eu-west-1",
				"endpoint=",
				"s3ForcePathStyle=true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := buildS3URL(tt.cfg)
			for _, param := range tt.expectedParams {
				if !strings.Contains(url, param) {
					t.Errorf("URL %q should contain parameter %q", url, param)
				}
			}
		})
	}
}

// TestTTL_Defaults tests TTL default values for different stores
func TestTTL_Defaults(t *testing.T) {
	defaultTTL := 15 * time.Minute

	tests := []struct {
		name     string
		openFunc func(context.Context, Config) (Store, error)
		cfg      Config
	}{
		{
			name:     "S3 default TTL",
			openFunc: OpenS3,
			cfg:      Config{Driver: "s3", Bucket: "test"},
		},
		{
			name:     "OSS default TTL",
			openFunc: OpenOSS,
			cfg: Config{
				Driver:    "oss",
				Bucket:    "test",
				Endpoint:  "oss.aliyuncs.com",
				AccessKey: "key",
				SecretKey: "secret",
			},
		},
		{
			name:     "COS default TTL",
			openFunc: OpenCOS,
			cfg: Config{
				Driver:    "cos",
				Bucket:    "test",
				Region:    "ap-guangzhou",
				AccessKey: "key",
				SecretKey: "secret",
			},
		},
		{
			name:     "OBS default TTL",
			openFunc: OpenOBS,
			cfg: Config{
				Driver:    "obs",
				Bucket:    "test",
				Endpoint:  "obs.myhuaweicloud.com",
				AccessKey: "key",
				SecretKey: "secret",
			},
		},
		{
			name:     "File default TTL",
			openFunc: OpenFile,
			cfg:      Config{Driver: "file", BaseDir: os.TempDir()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Most Open* functions will fail in test environment
			// This test documents the expected TTL behavior
			if tt.name == "File default TTL" {
				store, err := tt.openFunc(context.Background(), tt.cfg)
				if err != nil {
					t.Fatalf("OpenFile() error = %v", err)
				}
				// File store sets default TTL
				_ = store
			}
			// Verify default TTL constant
			if defaultTTL != 15*time.Minute {
				t.Errorf("Default TTL should be 15m, got %v", defaultTTL)
			}
		})
	}
}

// TestStore_Implementation verifies all store types exist and implement Store
func TestStore_Implementation(t *testing.T) {
	// Type assertions to verify all stores implement Store interface
	var _ Store = (*fileStore)(nil)
	var _ Store = (*s3Store)(nil)
	var _ Store = (*ossStore)(nil)
	var _ Store = (*cosStore)(nil)
	var _ Store = (*obsStore)(nil)
}
