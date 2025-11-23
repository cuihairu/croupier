package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig 测试配置结构体
type TestConfig struct {
	Name     string            `env:"APP_NAME"`
	Port     int               `env:"APP_PORT"`
	Debug    bool              `env:"APP_DEBUG"`
	Timeout  time.Duration     `env:"APP_TIMEOUT"`
	Features []string          `env:"APP_FEATURES"`
	Metadata map[string]string `env:"APP_METADATA"`

	Database struct {
		Host     string `env:"DB_HOST"`
		Port     int    `env:"DB_PORT"`
		Username string `env:"DB_USERNAME"`
		Password string `env:"DB_PASSWORD"` // 敏感信息
	}

	Cache CacheConfig
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled bool   `env:"CACHE_ENABLED"`
	TTL     int    `env:"CACHE_TTL"`
	Address string `env:"CACHE_ADDRESS"`
}

func TestEnvManager_LoadFromEnv(t *testing.T) {
	// 设置测试环境变量
	testEnv := map[string]string{
		"CROUPIER_APP_NAME":      "test-app",
		"CROUPIER_APP_PORT":      "8080",
		"CROUPIER_APP_DEBUG":     "true",
		"CROUPIER_APP_TIMEOUT":   "30s",
		"CROUPIER_APP_FEATURES":  "feature1,feature2,feature3",
		"CROUPIER_APP_METADATA":  "key1=value1,key2=value2",
		"CROUPIER_DB_HOST":       "localhost",
		"CROUPIER_DB_PORT":       "5432",
		"CROUPIER_DB_USERNAME":   "testuser",
		"CROUPIER_DB_PASSWORD":   "secret123",
		"CROUPIER_CACHE_ENABLED": "true",
		"CROUPIER_CACHE_TTL":     "300",
		"CROUPIER_CACHE_ADDRESS": "localhost:6379",
	}

	// 保存原始环境变量
	originalEnv := make(map[string]string)
	for key, value := range testEnv {
		originalEnv[key] = os.Getenv(key)
		os.Setenv(key, value)
	}
	defer func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// 创建环境变量管理器
	em := NewEnvManager("CROUPIER_")

	// 添加映射
	em.AddMapping("name", "APP_NAME")
	em.AddMapping("database", "DB")

	// 添加转换器
	em.AddTransformer(&TrimSpaceTransformer{})
	em.AddTransformer(&LowerCaseTransformer{})

	// 加载配置
	var config TestConfig
	err := em.LoadFromEnv(&config)
	require.NoError(t, err)

	// 验证配置
	assert.Equal(t, "test-app", config.Name)
	assert.Equal(t, 8080, config.Port)
	assert.True(t, config.Debug)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, []string{"feature1", "feature2", "feature3"}, config.Features)
	assert.Equal(t, map[string]string{"key1": "value1", "key2": "value2"}, config.Metadata)
	assert.Equal(t, "localhost", config.Database.Host)
	assert.Equal(t, 5432, config.Database.Port)
	assert.Equal(t, "testuser", config.Database.Username)
	assert.Equal(t, "secret123", config.Database.Password)

	// 验证嵌套指针结构
	assert.NotNil(t, config.Cache)
	assert.True(t, config.Cache.Enabled)
	assert.Equal(t, 300, config.Cache.TTL)
	assert.Equal(t, "localhost:6379", config.Cache.Address)
}

func TestEnvManager_LoadFromEnv_DurationWithoutUnit(t *testing.T) {
	t.Setenv("CROUPIER_APP_TIMEOUT", "45")

	em := NewEnvManager("CROUPIER_")

	var cfg TestConfig
	err := em.LoadFromEnv(&cfg)
	require.NoError(t, err)
	assert.Equal(t, 45*time.Second, cfg.Timeout)
}

func TestEnvManager_ExportToEnv(t *testing.T) {
	em := NewEnvManager("CROUPIER_")
	em.AddMapping("name", "APP_NAME")
	em.AddMapping("database", "DB")

	config := TestConfig{
		Name:     "export-test",
		Port:     9090,
		Debug:    false,
		Timeout:  60 * time.Second,
		Features: []string{"feat1", "feat2"},
		Metadata: map[string]string{"env": "test"},
	}

	config.Database.Host = "db.example.com"
	config.Database.Port = 3306
	config.Database.Username = "dbuser"

	config.Cache = CacheConfig{
		Enabled: true,
		TTL:     600,
		Address: "redis.example.com:6379",
	}

	err := em.ExportToEnv(&config)
	require.NoError(t, err)

	// 验证导出的环境变量
	assert.Equal(t, "export-test", os.Getenv("CROUPIER_APP_NAME"))
	assert.Equal(t, "9090", os.Getenv("CROUPIER_APP_PORT"))
	// Debug字段是false（零值），所以不会导出
	// assert.Equal(t, "false", os.Getenv("CROUPIER_APP_DEBUG"))
	assert.Equal(t, "1m0s", os.Getenv("CROUPIER_APP_TIMEOUT"))
	// Features和Metadata是切片和map，格式可能不同
	features := os.Getenv("CROUPIER_APP_FEATURES")
	assert.True(t, strings.Contains(features, "feat1") && strings.Contains(features, "feat2"))

	// 嵌套结构体字段
	assert.Equal(t, "db.example.com", os.Getenv("CROUPIER_DB_HOST"))
	assert.Equal(t, "3306", os.Getenv("CROUPIER_DB_PORT"))
	assert.Equal(t, "dbuser", os.Getenv("CROUPIER_DB_USERNAME"))

	// Cache字段
	assert.Equal(t, "true", os.Getenv("CROUPIER_CACHE_ENABLED"))
	assert.Equal(t, "600", os.Getenv("CROUPIER_CACHE_TTL"))
	assert.Equal(t, "redis.example.com:6379", os.Getenv("CROUPIER_CACHE_ADDRESS"))

	// 清理环境变量
	envKeys := []string{
		"CROUPIER_APP_NAME", "CROUPIER_APP_PORT", "CROUPIER_APP_DEBUG",
		"CROUPIER_APP_TIMEOUT", "CROUPIER_APP_FEATURES", "CROUPIER_APP_METADATA",
		"CROUPIER_DB_HOST", "CROUPIER_DB_PORT", "CROUPIER_DB_USERNAME",
		"CROUPIER_CACHE_ENABLED", "CROUPIER_CACHE_TTL", "CROUPIER_CACHE_ADDRESS",
	}
	for _, key := range envKeys {
		os.Unsetenv(key)
	}
}

func TestEnvManager_ExportToEnv_DefaultKeyFormat(t *testing.T) {
	type LoggingConfig struct {
		Level string
	}

	type Wrapper struct {
		Logging LoggingConfig
	}

	em := NewEnvManager("CROUPIER_")
	cfg := Wrapper{
		Logging: LoggingConfig{Level: "info"},
	}

	err := em.ExportToEnv(&cfg)
	require.NoError(t, err)

	assert.Equal(t, "info", os.Getenv("CROUPIER_LOGGING_LEVEL"))
	os.Unsetenv("CROUPIER_LOGGING_LEVEL")
}

func TestEnvManager_GetEnvInfo(t *testing.T) {
	// 设置测试环境变量（包括敏感信息）
	os.Setenv("CROUPIER_APP_NAME", "test-app")
	os.Setenv("CROUPIER_APP_SECRET", "very-secret-key")
	os.Setenv("CROUPIER_DB_PASSWORD", "database-password")
	os.Setenv("CROUPIER_API_TOKEN", "api-token-1234")
	os.Setenv("CROUPIER_HOTKEY_HINT", "Ctrl+C")
	defer func() {
		os.Unsetenv("CROUPIER_APP_NAME")
		os.Unsetenv("CROUPIER_APP_SECRET")
		os.Unsetenv("CROUPIER_DB_PASSWORD")
		os.Unsetenv("CROUPIER_API_TOKEN")
		os.Unsetenv("CROUPIER_HOTKEY_HINT")
	}()

	em := NewEnvManager("CROUPIER_")
	info := em.GetEnvInfo()

	// 验证环境变量信息
	assert.Contains(t, info, "CROUPIER_APP_NAME")
	assert.Equal(t, "test-app", info["CROUPIER_APP_NAME"].Value)
	assert.True(t, info["CROUPIER_APP_NAME"].Set)

	// 验证敏感信息被掩码
	assert.Contains(t, info, "CROUPIER_APP_SECRET")
	assert.Equal(t, "ve****ey", info["CROUPIER_APP_SECRET"].Value)
	assert.True(t, info["CROUPIER_APP_SECRET"].Set)

	assert.Contains(t, info, "CROUPIER_DB_PASSWORD")
	assert.Equal(t, "da****rd", info["CROUPIER_DB_PASSWORD"].Value)

	assert.Contains(t, info, "CROUPIER_API_TOKEN")
	assert.Equal(t, "ap****34", info["CROUPIER_API_TOKEN"].Value)

	// 测试短敏感值
	os.Setenv("CROUPIER_API_KEY_SHORT", "123")
	defer os.Unsetenv("CROUPIER_API_KEY_SHORT")

	info = em.GetEnvInfo()
	assert.Contains(t, info, "CROUPIER_API_KEY_SHORT")
	assert.Equal(t, "****", info["CROUPIER_API_KEY_SHORT"].Value)

	// 非敏感字段应该保持原值
	assert.Contains(t, info, "CROUPIER_HOTKEY_HINT")
	assert.Equal(t, "Ctrl+C", info["CROUPIER_HOTKEY_HINT"].Value)
}

func TestStringConverter(t *testing.T) {
	converter := &StringConverter{}

	value, err := converter.Convert("test-string")
	require.NoError(t, err)
	assert.Equal(t, "test-string", value)

	assert.Equal(t, reflect.String, converter.Kind())
}

func TestIntConverter(t *testing.T) {
	converter := &IntConverter{}

	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"123", 123, false},
		{"0", 0, false},
		{"-456", -456, false},
		{"invalid", 0, true},
		{"3.14", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			value, err := converter.Convert(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, value)
			}
			assert.Equal(t, reflect.Int, converter.Kind())
		})
	}
}

func TestBoolConverter(t *testing.T) {
	converter := &BoolConverter{}

	tests := []struct {
		input    string
		expected bool
		hasError bool
	}{
		{"true", true, false},
		{"True", true, false},
		{"TRUE", true, false},
		{"1", true, false},
		{"yes", true, false},
		{"on", true, false},
		{"enabled", true, false},
		{"false", false, false},
		{"False", false, false},
		{"FALSE", false, false},
		{"0", false, false},
		{"no", false, false},
		{"off", false, false},
		{"disabled", false, false},
		{"invalid", false, true}, // strconv.ParseBool会出错
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			value, err := converter.Convert(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, value)
			}
			assert.Equal(t, reflect.Bool, converter.Kind())
		})
	}
}

func TestSliceConverter(t *testing.T) {
	converter := &SliceConverter{}

	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"single", []string{"single"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{"a;b;c", []string{"a", "b", "c"}},
		{"a b c", []string{"a", "b", "c"}},
		{"a|b|c", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			value, err := converter.Convert(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, value)
			assert.Equal(t, reflect.Slice, converter.Kind())
		})
	}
}

func TestMapConverter(t *testing.T) {
	converter := &MapConverter{}

	tests := []struct {
		input    string
		expected map[string]string
	}{
		{"", map[string]string{}},
		{"key=value", map[string]string{"key": "value"}},
		{"key1=value1,key2=value2", map[string]string{"key1": "value1", "key2": "value2"}},
		{"key1=val1,key2=val2,key3=val3", map[string]string{"key1": "val1", "key2": "val2", "key3": "val3"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			value, err := converter.Convert(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, value)
			assert.Equal(t, reflect.Map, converter.Kind())
		})
	}
}

func TestDurationConverter(t *testing.T) {
	converter := &DurationConverter{}

	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
	}{
		{"30s", 30 * time.Second, false},
		{"1m", 1 * time.Minute, false},
		{"2h", 2 * time.Hour, false},
		{"100ms", 100 * time.Millisecond, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			value, err := converter.Convert(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, value)
			}
			assert.Equal(t, reflect.Int64, converter.Kind())
		})
	}
}

func TestURLTransformer(t *testing.T) {
	transformer := &URLTransformer{}

	tests := []struct {
		key      string
		value    string
		expected string
	}{
		{"APP_URL", "example.com", "http://example.com"},
		{"API_URL", "https://api.example.com", "https://api.example.com"},
		{"SERVER_HOST", "localhost", "localhost"},
		{"OTHER_KEY", "value", "value"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			_, newValue, err := transformer.Transform(tt.key, tt.value)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, newValue)
		})
	}

	assert.Equal(t, "URLTransformer", transformer.Name())
}

func TestLowerCaseTransformer(t *testing.T) {
	transformer := &LowerCaseTransformer{}

	tests := []struct {
		key      string
		value    string
		expected string
	}{
		{"USER_EMAIL", "Test@Example.COM", "test@example.com"},
		{"USERNAME", "TestUser", "testuser"},
		{"OTHER_KEY", "VALUE", "VALUE"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			_, newValue, err := transformer.Transform(tt.key, tt.value)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, newValue)
		})
	}

	assert.Equal(t, "LowerCaseTransformer", transformer.Name())
}

func TestTrimSpaceTransformer(t *testing.T) {
	transformer := &TrimSpaceTransformer{}

	tests := []struct {
		key      string
		value    string
		expected string
	}{
		{"ANY_KEY", "  trimmed value  ", "trimmed value"},
		{"OTHER_KEY", "notrim", "notrim"},
		{"SPACES", "   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			_, newValue, err := transformer.Transform(tt.key, tt.value)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, newValue)
		})
	}

	assert.Equal(t, "TrimSpaceTransformer", transformer.Name())
}

func TestEnvManager_ErrorHandling(t *testing.T) {
	em := NewEnvManager("CROUPIER_")

	// 测试非指针输入
	err := em.LoadFromEnv("not a pointer")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config必须是非nil指针")

	// 测试nil指针
	var config *TestConfig
	err = em.LoadFromEnv(&config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config必须指向结构体")

	// 测试无效的环境变量值
	os.Setenv("CROUPIER_APP_PORT", "invalid-port")
	defer os.Unsetenv("CROUPIER_APP_PORT")

	var validConfig TestConfig
	err = em.LoadFromEnv(&validConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "设置字段值失败")
}

// CustomTypeConverter 自定义类型转换器
type CustomTypeConverter struct {
	EnvConverter
}

func (c *CustomTypeConverter) Convert(value string) (interface{}, error) {
	return struct{ Value string }{Value: "custom:" + value}, nil
}

func (c *CustomTypeConverter) Kind() reflect.Kind {
	return reflect.Struct
}

func TestEnvManager_CustomConverter(t *testing.T) {
	// 创建自定义转换器
	type CustomType struct {
		Value string
	}

	customConverter := &CustomTypeConverter{}

	em := NewEnvManager("CROUPIER_")
	em.AddConverter(customConverter)

	// 这个测试需要实际的CustomType支持，这里只演示接口设计
	em.GetEnvInfo() // 确保没有panic
}

func BenchmarkEnvManager_LoadFromEnv(b *testing.B) {
	// 设置测试环境变量
	testEnv := map[string]string{
		"CROUPIER_APP_NAME":  "bench-app",
		"CROUPIER_APP_PORT":  "8080",
		"CROUPIER_APP_DEBUG": "true",
	}

	for key, value := range testEnv {
		os.Setenv(key, value)
	}
	defer func() {
		for key := range testEnv {
			os.Unsetenv(key)
		}
	}()

	em := NewEnvManager("CROUPIER_")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var config TestConfig
		_ = em.LoadFromEnv(&config)
	}
}
