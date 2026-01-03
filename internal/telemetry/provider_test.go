package telemetry

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

// TestTelemetryConfig_Defaults 测试默认配置
func TestTelemetryConfig_Defaults(t *testing.T) {
	config := TelemetryConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		CollectorURL:   "localhost:4318",
		GameID:         "test-game",
		EnableTracing:  true,
		EnableMetrics:  true,
		SamplingRatio:  1.0,
	}

	if config.ServiceName != "test-service" {
		t.Errorf("Expected ServiceName 'test-service', got %s", config.ServiceName)
	}
	if config.SamplingRatio != 1.0 {
		t.Errorf("Expected SamplingRatio 1.0, got %f", config.SamplingRatio)
	}
}

// TestLoadConfigFromEnv 测试从环境变量加载配置
func TestLoadConfigFromEnv(t *testing.T) {
	// 设置环境变量
	os.Setenv("OTEL_SERVICE_NAME", "test-from-env")
	os.Setenv("OTEL_SERVICE_VERSION", "2.0.0")
	os.Setenv("OTEL_ENVIRONMENT", "production")
	os.Setenv("GAME_ID", "my-game")
	defer func() {
		os.Unsetenv("OTEL_SERVICE_NAME")
		os.Unsetenv("OTEL_SERVICE_VERSION")
		os.Unsetenv("OTEL_ENVIRONMENT")
		os.Unsetenv("GAME_ID")
	}()

	config := LoadConfigFromEnv()

	if config.ServiceName != "test-from-env" {
		t.Errorf("Expected ServiceName 'test-from-env', got %s", config.ServiceName)
	}
	if config.ServiceVersion != "2.0.0" {
		t.Errorf("Expected ServiceVersion '2.0.0', got %s", config.ServiceVersion)
	}
	if config.Environment != "production" {
		t.Errorf("Expected Environment 'production', got %s", config.Environment)
	}
	if config.GameID != "my-game" {
		t.Errorf("Expected GameID 'my-game', got %s", config.GameID)
	}
}

// TestLoadConfigFromEnv_Defaults 测试默认值
func TestLoadConfigFromEnv_Defaults(t *testing.T) {
	// 确保环境变量未设置
	os.Unsetenv("OTEL_SERVICE_NAME")
	os.Unsetenv("OTEL_SERVICE_VERSION")
	os.Unsetenv("OTEL_ENVIRONMENT")

	config := LoadConfigFromEnv()

	if config.ServiceName != "croupier-server" {
		t.Errorf("Expected default ServiceName 'croupier-server', got %s", config.ServiceName)
	}
	if config.ServiceVersion != "1.0.0" {
		t.Errorf("Expected default ServiceVersion '1.0.0', got %s", config.ServiceVersion)
	}
	if config.Environment != "development" {
		t.Errorf("Expected default Environment 'development', got %s", config.Environment)
	}
}

// TestGetEnvOrDefault 测试环境变量获取
func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		setValue     string
		defaultValue string
		expected     string
	}{
		{
			name:         "环境变量已设置",
			key:          "TEST_VAR",
			setValue:     "test-value",
			defaultValue: "default",
			expected:     "test-value",
		},
		{
			name:         "环境变量未设置",
			key:          "NON_EXISTENT_VAR",
			setValue:     "",
			defaultValue: "default",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setValue != "" {
				os.Setenv(tt.key, tt.setValue)
				defer os.Unsetenv(tt.key)
			}

			result := getEnvOrDefault(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("getEnvOrDefault(%q, %q) = %q, want %q",
					tt.key, tt.defaultValue, result, tt.expected)
			}
		})
	}
}

// TestParseFloatOrDefault 测试浮点数解析
func TestParseFloatOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"1.0", "1.0", 1.0},
		{"0.1", "0.1", 0.1},
		{"无效值", "invalid", 1.0}, // 默认值
		{"空字符串", "", 1.0},       // 默认值
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFloatOrDefault(tt.input)
			if result != tt.expected {
				t.Errorf("parseFloatOrDefault(%q) = %f, want %f",
					tt.input, result, tt.expected)
			}
		})
	}
}

// TestParseIntOrDefault 测试整数解析
func TestParseIntOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"0", "0", 0},
		{"100", "100", 100},
		{"168", "168", 168},
		{"无效值", "invalid", 0}, // 默认值
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseIntOrDefault(tt.input)
			if result != tt.expected {
				t.Errorf("parseIntOrDefault(%q) = %d, want %d",
					tt.input, result, tt.expected)
			}
		})
	}
}

// TestParseDurationOrDefault 测试时间间隔解析
func TestParseDurationOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{"30秒", "30s", 30 * time.Second},
		{"60秒", "60s", 60 * time.Second},
		{"5分钟", "5m", 5 * time.Minute},
		{"无效值", "invalid", 30 * time.Second}, // 默认值
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDurationOrDefault(tt.input)
			if result != tt.expected {
				t.Errorf("parseDurationOrDefault(%q) = %v, want %v",
					tt.input, result, tt.expected)
			}
		})
	}
}

// TestAnalyticsBridgeConfig_Defaults 测试 Analytics 桥接配置默认值
func TestAnalyticsBridgeConfig_Defaults(t *testing.T) {
	config := LoadConfigFromEnv()

	// Analytics 配置应该有默认值
	if config.Analytics.RedisAddr != "localhost:6379" {
		t.Errorf("Expected default RedisAddr 'localhost:6379', got %s", config.Analytics.RedisAddr)
	}
	if config.Analytics.TopicPrefix != "game:events" {
		t.Errorf("Expected default TopicPrefix 'game:events', got %s", config.Analytics.TopicPrefix)
	}
	if config.Analytics.RetentionHours != 168 {
		t.Errorf("Expected default RetentionHours 168, got %d", config.Analytics.RetentionHours)
	}
	if config.Analytics.BatchSize != 100 {
		t.Errorf("Expected default BatchSize 100, got %d", config.Analytics.BatchSize)
	}
	if config.Analytics.FlushInterval != 30*time.Second {
		t.Errorf("Expected default FlushInterval 30s, got %v", config.Analytics.FlushInterval)
	}
}

// TestTelemetryConfig_BooleanParsing 测试布尔值解析
func TestTelemetryConfig_BooleanParsing(t *testing.T) {
	tests := []struct {
		name        string
		envKey      string
		envValue    string
		expectedVal bool
		getValue    func(TelemetryConfig) bool
	}{
		{
			name:        "Tracing 启用",
			envKey:      "OTEL_ENABLE_TRACING",
			envValue:    "true",
			expectedVal: true,
			getValue:    func(c TelemetryConfig) bool { return c.EnableTracing },
		},
		{
			name:        "Tracing 禁用",
			envKey:      "OTEL_ENABLE_TRACING",
			envValue:    "false",
			expectedVal: false,
			getValue:    func(c TelemetryConfig) bool { return c.EnableTracing },
		},
		{
			name:        "Metrics 启用",
			envKey:      "OTEL_ENABLE_METRICS",
			envValue:    "true",
			expectedVal: true,
			getValue:    func(c TelemetryConfig) bool { return c.EnableMetrics },
		},
		{
			name:        "Analytics 启用",
			envKey:      "ANALYTICS_BRIDGE_ENABLED",
			envValue:    "true",
			expectedVal: true,
			getValue:    func(c TelemetryConfig) bool { return c.Analytics.Enabled },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(tt.envKey, tt.envValue)
			defer os.Unsetenv(tt.envKey)

			config := LoadConfigFromEnv()
			if tt.getValue(config) != tt.expectedVal {
				t.Errorf("Expected %v when env is %s, got %v",
					tt.expectedVal, tt.envValue, tt.getValue(config))
			}
		})
	}
}

// TestTelemetryConfig_AnalyticsDefaults 测试 Analytics 默认配置
func TestTelemetryConfig_AnalyticsDefaults(t *testing.T) {
	os.Unsetenv("ANALYTICS_REDIS_ADDR")
	os.Unsetenv("ANALYTICS_REDIS_PASSWORD")
	os.Unsetenv("ANALYTICS_REDIS_DB")

	config := LoadConfigFromEnv()

	if config.Analytics.RedisAddr != "localhost:6379" {
		t.Errorf("Expected default RedisAddr 'localhost:6379', got %s", config.Analytics.RedisAddr)
	}
	if config.Analytics.RedisPassword != "" {
		t.Errorf("Expected default empty RedisPassword, got %s", config.Analytics.RedisPassword)
	}
	if config.Analytics.RedisDB != 0 {
		t.Errorf("Expected default RedisDB 0, got %d", config.Analytics.RedisDB)
	}
}

// TestNewProvider_InvalidConfig 测试无效配置
func TestNewProvider_InvalidConfig(t *testing.T) {
	logger := slog.Default()

	// 测试空服务名称
	config := TelemetryConfig{
		ServiceName:    "",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		CollectorURL:   "localhost:4318",
		GameID:         "test-game",
		EnableTracing:  false,
		EnableMetrics:  false,
	}

	provider, err := NewProvider(context.Background(), config, logger)
	if err != nil {
		t.Errorf("NewProvider with empty ServiceName should succeed when tracing/metrics disabled, got error: %v", err)
	}
	if provider != nil {
		// 应该能创建 provider，只是没有 tracing 和 metrics
		if provider.TracerProvider != nil {
			t.Error("TracerProvider should be nil when EnableTracing is false")
		}
		if provider.MeterProvider != nil {
			t.Error("MeterProvider should be nil when EnableMetrics is false")
		}
	}
}

// TestProvider_Shutdown 测试关闭 Provider
func TestProvider_Shutdown(t *testing.T) {
	logger := slog.Default()
	config := TelemetryConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		CollectorURL:   "localhost:4318",
		GameID:         "test-game",
		EnableTracing:  false, // 禁用以避免实际连接
		EnableMetrics:  false,
	}

	provider, err := NewProvider(context.Background(), config, logger)
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	// 关闭应该成功
	ctx := context.Background()
	err = provider.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// 多次关闭应该是幂等的
	err = provider.Shutdown(ctx)
	if err != nil {
		t.Errorf("Second Shutdown failed: %v", err)
	}
}

// TestProvider_GameMetrics 测试游戏指标
func TestProvider_GameMetrics(t *testing.T) {
	logger := slog.Default()
	config := TelemetryConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		CollectorURL:   "localhost:4318",
		GameID:         "test-game",
		EnableTracing:  false,
		EnableMetrics:  false,
	}

	provider, err := NewProvider(context.Background(), config, logger)
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}
	defer provider.Shutdown(context.Background())

	if provider.GameMetrics == nil {
		t.Error("GameMetrics should not be nil")
	}

	if provider.GameTracer == nil {
		t.Error("GameTracer should not be nil")
	}

	if provider.Bridge == nil {
		t.Error("Bridge should not be nil")
	}
}

// BenchmarkLoadConfigFromEnv 性能基准测试
func BenchmarkLoadConfigFromEnv(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LoadConfigFromEnv()
	}
}

// BenchmarkParseFloatOrDefault 性能基准测试
func BenchmarkParseFloatOrDefault(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseFloatOrDefault("1.0")
	}
}
