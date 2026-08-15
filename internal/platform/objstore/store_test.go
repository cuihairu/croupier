package objstore

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestFromEnv 测试从环境变量加载配置
func TestFromEnv(t *testing.T) {
	// 保存原始环境变量
	originalEnv := map[string]string{
		"STORAGE_DRIVER":           os.Getenv("STORAGE_DRIVER"),
		"STORAGE_BUCKET":           os.Getenv("STORAGE_BUCKET"),
		"STORAGE_REGION":           os.Getenv("STORAGE_REGION"),
		"STORAGE_ENDPOINT":         os.Getenv("STORAGE_ENDPOINT"),
		"STORAGE_ACCESS_KEY":       os.Getenv("STORAGE_ACCESS_KEY"),
		"STORAGE_SECRET_KEY":       os.Getenv("STORAGE_SECRET_KEY"),
		"STORAGE_BASE_DIR":         os.Getenv("STORAGE_BASE_DIR"),
		"STORAGE_FORCE_PATH_STYLE": os.Getenv("STORAGE_FORCE_PATH_STYLE"),
		"STORAGE_SIGNED_URL_TTL":   os.Getenv("STORAGE_SIGNED_URL_TTL"),
	}

	// 测试后恢复环境变量
	defer func() {
		for k, v := range originalEnv {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// 设置测试环境变量
	os.Setenv("STORAGE_DRIVER", "s3")
	os.Setenv("STORAGE_BUCKET", "test-bucket")
	os.Setenv("STORAGE_REGION", "us-west-2")
	os.Setenv("STORAGE_ENDPOINT", "https://s3.amazonaws.com")
	os.Setenv("STORAGE_ACCESS_KEY", "test-key")
	os.Setenv("STORAGE_SECRET_KEY", "test-secret")
	os.Setenv("STORAGE_BASE_DIR", "/tmp/storage")
	os.Setenv("STORAGE_FORCE_PATH_STYLE", "true")
	os.Setenv("STORAGE_SIGNED_URL_TTL", "1h")

	cfg := FromEnv()

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
	if cfg.AccessKey != "test-key" {
		t.Errorf("AccessKey = %q, want 'test-key'", cfg.AccessKey)
	}
	if cfg.SecretKey != "test-secret" {
		t.Errorf("SecretKey = %q, want 'test-secret'", cfg.SecretKey)
	}
	if cfg.BaseDir != "/tmp/storage" {
		t.Errorf("BaseDir = %q, want '/tmp/storage'", cfg.BaseDir)
	}
	if !cfg.ForcePathStyle {
		t.Error("ForcePathStyle should be true")
	}
	if cfg.SignedURLTTL != time.Hour {
		t.Errorf("SignedURLTTL = %v, want 1h", cfg.SignedURLTTL)
	}
}

// TestFromEnv_Empty 测试空环境变量
func TestFromEnv_Empty(t *testing.T) {
	// 清除所有相关环境变量
	envVars := []string{
		"STORAGE_DRIVER", "STORAGE_BUCKET", "STORAGE_REGION",
		"STORAGE_ENDPOINT", "STORAGE_ACCESS_KEY", "STORAGE_SECRET_KEY",
		"STORAGE_BASE_DIR", "STORAGE_FORCE_PATH_STYLE", "STORAGE_SIGNED_URL_TTL",
	}
	originalVals := make(map[string]string)
	for _, v := range envVars {
		originalVals[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range originalVals {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	cfg := FromEnv()

	if cfg.Driver != "" {
		t.Errorf("Driver should be empty, got %q", cfg.Driver)
	}
	if cfg.SignedURLTTL != 0 {
		t.Errorf("SignedURLTTL should be 0, got %v", cfg.SignedURLTTL)
	}
}

// TestFromEnv_ForcePathStyleVariations 测试 ForcePathStyle 的各种值
func TestFromEnv_ForcePathStyleVariations(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"true", "true", true},
		{"TRUE", "TRUE", true},
		{"1", "1", true},
		{"yes", "yes", true},
		{"YES", "YES", true},
		{"false", "false", false},
		{"0", "0", false},
		{"no", "no", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("STORAGE_FORCE_PATH_STYLE", tt.value)
			defer os.Unsetenv("STORAGE_FORCE_PATH_STYLE")

			cfg := FromEnv()
			if cfg.ForcePathStyle != tt.expected {
				t.Errorf("ForcePathStyle = %v, want %v", cfg.ForcePathStyle, tt.expected)
			}
		})
	}
}

// TestFromEnv_SignedURLTTL 测试解析 TTL
func TestFromEnv_SignedURLTTL(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected time.Duration
	}{
		{"1 hour", "1h", time.Hour},
		{"30 minutes", "30m", 30 * time.Minute},
		{"300 seconds", "300s", 300 * time.Second},
		{"invalid", "invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("STORAGE_SIGNED_URL_TTL", tt.value)
			defer os.Unsetenv("STORAGE_SIGNED_URL_TTL")

			cfg := FromEnv()
			if cfg.SignedURLTTL != tt.expected {
				t.Errorf("SignedURLTTL = %v, want %v", cfg.SignedURLTTL, tt.expected)
			}
		})
	}
}

// TestValidate 测试配置验证
func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "有效的 S3 配置",
			cfg: Config{
				Driver: "s3",
				Bucket: "test-bucket",
			},
			wantErr: false,
		},
		{
			name: "S3 缺少 bucket",
			cfg: Config{
				Driver: "s3",
			},
			wantErr: true,
		},
		{
			name: "有效的 OSS 配置",
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
			name: "OSS 缺少 endpoint",
			cfg: Config{
				Driver:    "oss",
				Bucket:    "test-bucket",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: true,
		},
		{
			name: "有效的 COS 配置",
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
			name: "有效的 File 配置",
			cfg: Config{
				Driver:  "file",
				BaseDir: os.TempDir(),
			},
			wantErr: false,
		},
		{
			name:    "空 driver",
			cfg:     Config{},
			wantErr: true,
		},
		{
			name: "未知的 driver",
			cfg: Config{
				Driver: "unknown",
			},
			wantErr: true,
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

// TestSanitizeKey 测试键名清理
func TestSanitizeKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"普通键", "path/to/file.txt", "path/to/file.txt"},
		{"带前导斜杠", "/leading/slash", "leading/slash"},
		{"多个前导斜杠", "///multiple/leading", "multiple/leading"},
		{"包含当前目录", "path/./to/./file", "path/to/file"},
		{"包含上级目录", "path/../to/../../file", "file"},
		{"混合情况", "///path/./../to//./file", "to/file"},
		{"只有点和双点", "./..", ""},
		{"空字符串", "", ""},
		{"复杂清理", "a/b/../../c/./d/../e", "c/e"}, // sanitizeKey 按词法解析 ..（回到上级）
		{"带斜杠的空段", "a///b//c", "a/b/c"},
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

// TestBuildS3URL 测试构建 S3 URL
func TestBuildS3URL(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		expected string // 只检查关键部分
	}{
		{
			name: "基本 S3 URL",
			cfg: Config{
				Bucket: "my-bucket",
			},
			expected: "s3://my-bucket", // 无查询参数时不带 ?
		},
		{
			name: "带 region",
			cfg: Config{
				Bucket: "my-bucket",
				Region: "us-west-2",
			},
			expected: "region=us-west-2",
		},
		{
			name: "带 endpoint",
			cfg: Config{
				Bucket:   "my-bucket",
				Endpoint: "https://s3.amazonaws.com",
			},
			expected: "endpoint=https%3A%2F%2Fs3.amazonaws.com", // URL 编码
		},
		{
			name: "强制路径样式",
			cfg: Config{
				Bucket:         "my-bucket",
				ForcePathStyle: true,
			},
			expected: "s3ForcePathStyle=true",
		},
		{
			name: "完整配置",
			cfg: Config{
				Bucket:         "my-bucket",
				Region:         "us-west-2",
				Endpoint:       "https://s3.amazonaws.com",
				ForcePathStyle: true,
			},
			expected: "s3://my-bucket?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildS3URL(tt.cfg)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("buildS3URL() result = %q, should contain %q", result, tt.expected)
			}
		})
	}
}

// TestConfigDefaults 测试配置默认值
func TestConfigDefaults(t *testing.T) {
	cfg := Config{}

	if cfg.SignedURLTTL != 0 {
		t.Errorf("Default SignedURLTTL should be 0, got %v", cfg.SignedURLTTL)
	}
	if cfg.ForcePathStyle {
		t.Error("Default ForcePathStyle should be false")
	}
}

// MockStore 用于测试 Store 接口
type MockStore struct {
	PutCalled        bool
	LastKey          string
	LastContentType  string
	DeleteCalled     bool
	LastDeletedKey   string
	SignedURLCalled  bool
	LastSignedKey    string
	LastSignedMethod string
	LastExpiry       time.Duration
}

func (m *MockStore) Put(ctx context.Context, key string, r ReadSeeker, size int64, contentType string) error {
	m.PutCalled = true
	m.LastKey = key
	m.LastContentType = contentType
	return nil
}

func (m *MockStore) SignedURL(ctx context.Context, key string, method string, expiry time.Duration) (string, error) {
	m.SignedURLCalled = true
	m.LastSignedKey = key
	m.LastSignedMethod = method
	m.LastExpiry = expiry
	return "https://example.com/signed-url", nil
}

func (m *MockStore) Delete(ctx context.Context, key string) error {
	m.DeleteCalled = true
	m.LastDeletedKey = key
	return nil
}

// TestMockStore 测试模拟存储
func TestMockStore(t *testing.T) {
	ctx := context.Background()
	store := &MockStore{}

	// 测试 Put
	data := strings.NewReader("test data")
	err := store.Put(ctx, "test/key.txt", data, 9, "text/plain")
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if !store.PutCalled {
		t.Error("Put() should set PutCalled flag")
	}
	if store.LastKey != "test/key.txt" {
		t.Errorf("LastKey = %q, want 'test/key.txt'", store.LastKey)
	}

	// 测试 SignedURL
	url, err := store.SignedURL(ctx, "test/key.txt", "GET", time.Hour)
	if err != nil {
		t.Fatalf("SignedURL() error = %v", err)
	}

	if !store.SignedURLCalled {
		t.Error("SignedURL() should set SignedURLCalled flag")
	}
	if url != "https://example.com/signed-url" {
		t.Errorf("SignedURL() = %q, want 'https://example.com/signed-url'", url)
	}

	// 测试 Delete
	err = store.Delete(ctx, "test/key.txt")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if !store.DeleteCalled {
		t.Error("Delete() should set DeleteCalled flag")
	}
	if store.LastDeletedKey != "test/key.txt" {
		t.Errorf("LastDeletedKey = %q, want 'test/key.txt'", store.LastDeletedKey)
	}
}

// BenchmarkSanitizeKey 性能基准测试
func BenchmarkSanitizeKey(b *testing.B) {
	key := "path/./to/../file//with/slashes"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sanitizeKey(key)
	}
}

// BenchmarkBuildS3URL 性能基准测试
func BenchmarkBuildS3URL(b *testing.B) {
	cfg := Config{
		Bucket:         "test-bucket",
		Region:         "us-west-2",
		Endpoint:       "https://s3.amazonaws.com",
		ForcePathStyle: true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildS3URL(cfg)
	}
}
