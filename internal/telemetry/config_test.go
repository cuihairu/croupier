package telemetry

import (
	"os"
	"testing"
)

func TestParseBool(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"true lowercase", "true", true},
		{"true uppercase", "TRUE", true},
		{"true mixed case", "True", true},
		{"1 as true", "1", true},
		{"yes lowercase", "yes", true},
		{"yes uppercase", "YES", true},
		{"on lowercase", "on", true},
		{"on uppercase", "ON", true},
		{"false lowercase", "false", false},
		{"0 as false", "0", false},
		{"empty string", "", false},
		{"random string", "random", false},
		{"true with spaces", "  true  ", true},
		{"false with spaces", "  false  ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseBool(tt.value)
			if result != tt.expected {
				t.Errorf("parseBool(%q) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestLookupEnv(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		setValue   string
		wantValue  string
		wantExists bool
	}{
		{"env exists", "TEST_TELEMETRY_LOOKUP", "value1", "value1", true},
		{"env does not exist", "TEST_TELEMETRY_LOOKUP_NONEXISTENT", "", "", false},
		{"env with spaces", "TEST_TELEMETRY_LOOKUP_TRIM", "  value2  ", "value2", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setValue != "" {
				os.Setenv(tt.key, tt.setValue)
				defer os.Unsetenv(tt.key)
			}
			value, exists := lookupEnv(tt.key)
			if exists != tt.wantExists {
				t.Errorf("lookupEnv(%q) exists = %v, want %v", tt.key, exists, tt.wantExists)
			}
			if exists && value != tt.wantValue {
				t.Errorf("lookupEnv(%q) = %q, want %q", tt.key, value, tt.wantValue)
			}
		})
	}
}

func TestMergeEnv_OverlaysConfig(t *testing.T) {
	os.Setenv("OTEL_ENABLED", "true")
	os.Setenv("OTEL_SERVICE_NAME", "test-service")
	os.Setenv("OTEL_SERVICE_VERSION", "1.0.0")
	os.Setenv("OTEL_ENVIRONMENT", "test")
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
	os.Setenv("GAME_ID", "test-game")
	defer func() {
		os.Unsetenv("OTEL_ENABLED")
		os.Unsetenv("OTEL_SERVICE_NAME")
		os.Unsetenv("OTEL_SERVICE_VERSION")
		os.Unsetenv("OTEL_ENVIRONMENT")
		os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		os.Unsetenv("GAME_ID")
	}()

	config := TelemetryConfig{
		Enabled: false,
	}
	result := MergeEnv(config)

	if !result.Enabled {
		t.Error("Expected Enabled to be true")
	}
	if result.ServiceName != "test-service" {
		t.Errorf("Expected ServiceName to be 'test-service', got %q", result.ServiceName)
	}
	if result.ServiceVersion != "1.0.0" {
		t.Errorf("Expected ServiceVersion to be '1.0.0', got %q", result.ServiceVersion)
	}
	if result.Environment != "test" {
		t.Errorf("Expected Environment to be 'test', got %q", result.Environment)
	}
	if result.CollectorURL != "http://localhost:4317" {
		t.Errorf("Expected CollectorURL to be 'http://localhost:4317', got %q", result.CollectorURL)
	}
	if result.GameID != "test-game" {
		t.Errorf("Expected GameID to be 'test-game', got %q", result.GameID)
	}
}

func TestMergeEnv_EnablesTracing(t *testing.T) {
	os.Setenv("OTEL_ENABLE_TRACING", "true")
	defer os.Unsetenv("OTEL_ENABLE_TRACING")

	config := TelemetryConfig{Enabled: false}
	result := MergeEnv(config)

	if !result.Enabled {
		t.Error("Expected Enabled to be true when tracing is enabled")
	}
	if !result.EnableTracing {
		t.Error("Expected EnableTracing to be true")
	}
}

func TestMergeEnv_EnablesMetrics(t *testing.T) {
	os.Setenv("OTEL_ENABLE_METRICS", "1")
	defer os.Unsetenv("OTEL_ENABLE_METRICS")

	config := TelemetryConfig{Enabled: false}
	result := MergeEnv(config)

	if !result.Enabled {
		t.Error("Expected Enabled to be true when metrics is enabled")
	}
	if !result.EnableMetrics {
		t.Error("Expected EnableMetrics to be true")
	}
}

func TestMergeEnv_Analytics(t *testing.T) {
	os.Setenv("ANALYTICS_BRIDGE_ENABLED", "true")
	os.Setenv("ANALYTICS_REDIS_ADDR", "localhost:6379")
	os.Setenv("ANALYTICS_REDIS_PASSWORD", "secret")
	os.Setenv("ANALYTICS_REDIS_DB", "1")
	os.Setenv("ANALYTICS_TOPIC_PREFIX", "test:")
	os.Setenv("ANALYTICS_RETENTION_HOURS", "24")
	os.Setenv("ANALYTICS_BATCH_SIZE", "100")
	defer func() {
		os.Unsetenv("ANALYTICS_BRIDGE_ENABLED")
		os.Unsetenv("ANALYTICS_REDIS_ADDR")
		os.Unsetenv("ANALYTICS_REDIS_PASSWORD")
		os.Unsetenv("ANALYTICS_REDIS_DB")
		os.Unsetenv("ANALYTICS_TOPIC_PREFIX")
		os.Unsetenv("ANALYTICS_RETENTION_HOURS")
		os.Unsetenv("ANALYTICS_BATCH_SIZE")
	}()

	config := TelemetryConfig{Enabled: false}
	result := MergeEnv(config)

	if !result.Enabled {
		t.Error("Expected Enabled to be true when analytics is enabled")
	}
	if !result.Analytics.Enabled {
		t.Error("Expected Analytics.Enabled to be true")
	}
	if result.Analytics.RedisAddr != "localhost:6379" {
		t.Errorf("Expected Analytics.RedisAddr to be 'localhost:6379', got %q", result.Analytics.RedisAddr)
	}
	if result.Analytics.RedisPassword != "secret" {
		t.Errorf("Expected Analytics.RedisPassword to be 'secret', got %q", result.Analytics.RedisPassword)
	}
	if result.Analytics.RedisDB != 1 {
		t.Errorf("Expected Analytics.RedisDB to be 1, got %d", result.Analytics.RedisDB)
	}
	if result.Analytics.TopicPrefix != "test:" {
		t.Errorf("Expected Analytics.TopicPrefix to be 'test:', got %q", result.Analytics.TopicPrefix)
	}
	if result.Analytics.RetentionHours != 24 {
		t.Errorf("Expected Analytics.RetentionHours to be 24, got %d", result.Analytics.RetentionHours)
	}
	if result.Analytics.BatchSize != 100 {
		t.Errorf("Expected Analytics.BatchSize to be 100, got %d", result.Analytics.BatchSize)
	}
}

func TestMergeEnv_CroputierEnabled(t *testing.T) {
	os.Setenv("CROUPIER_TELEMETRY_ENABLED", "yes")
	defer os.Unsetenv("CROUPIER_TELEMETRY_ENABLED")

	config := TelemetryConfig{Enabled: false}
	result := MergeEnv(config)

	if !result.Enabled {
		t.Error("Expected Enabled to be true")
	}
}
