package provider

import (
	"context"
	"testing"
)

// mockProvider 模拟 Provider 实现
type mockProvider struct {
	name             string
	enabled          bool
	supportedMethods []string
}

func newMockProvider(name string, enabled bool, methods []string) *mockProvider {
	return &mockProvider{
		name:             name,
		enabled:          enabled,
		supportedMethods: methods,
	}
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Init(ctx context.Context, config ProviderConfig) error {
	return nil
}

func (m *mockProvider) IsEnabled() bool {
	return m.enabled
}

func (m *mockProvider) SupportedMethods() []string {
	return m.supportedMethods
}

func (m *mockProvider) Call(ctx context.Context, method string, request []byte) ([]byte, error) {
	return []byte(`{"result":"ok"}`), nil
}

func (m *mockProvider) Close() error {
	return nil
}

// TestProviderNotFoundError 测试 ProviderNotFoundError
func TestProviderNotFoundError(t *testing.T) {
	err := &ProviderNotFoundError{Name: "test_provider"}
	expected := "provider not found: test_provider"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

// TestMethodNotSupportedError 测试 MethodNotSupportedError
func TestMethodNotSupportedError(t *testing.T) {
	err := &MethodNotSupportedError{Provider: "test_provider", Method: "test_method"}
	expected := `method "test_method" not supported by provider "test_provider"`
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

// TestProviderDisabledError 测试 ProviderDisabledError
func TestProviderDisabledError(t *testing.T) {
	err := &ProviderDisabledError{Name: "test_provider"}
	expected := `provider "test_provider" is disabled`
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

// TestProviderConfig 测试 ProviderConfig 默认值
func TestProviderConfig(t *testing.T) {
	config := ProviderConfig{
		Enabled: true,
		Type:    "test",
		Config:  make(map[string]interface{}),
	}

	if !config.Enabled {
		t.Error("Expected Enabled to be true")
	}

	if config.Type != "test" {
		t.Errorf("Expected Type 'test', got '%s'", config.Type)
	}

	if config.Config == nil {
		t.Error("Config map should be initialized")
	}
}

// TestProviderConfig_WithRateLimit 测试带限流配置
func TestProviderConfig_WithRateLimit(t *testing.T) {
	config := ProviderConfig{
		Enabled: true,
		Type:    "test",
		RateLimit: &RateLimitConfig{
			RequestsPerMinute: 100,
			BurstSize:         10,
		},
	}

	if config.RateLimit == nil {
		t.Fatal("RateLimit should not be nil")
	}

	if config.RateLimit.RequestsPerMinute != 100 {
		t.Errorf("Expected RequestsPerMinute 100, got %d", config.RateLimit.RequestsPerMinute)
	}

	if config.RateLimit.BurstSize != 10 {
		t.Errorf("Expected BurstSize 10, got %d", config.RateLimit.BurstSize)
	}
}

// TestMockProvider 测试模拟 Provider 实现
func TestMockProvider(t *testing.T) {
	ctx := context.Background()
	provider := newMockProvider("mock", true, []string{"method1", "method2"})

	// 测试 Name
	if provider.Name() != "mock" {
		t.Errorf("Expected name 'mock', got '%s'", provider.Name())
	}

	// 测试 Init
	err := provider.Init(ctx, ProviderConfig{})
	if err != nil {
		t.Errorf("Init() should not return error, got %v", err)
	}

	// 测试 IsEnabled
	if !provider.IsEnabled() {
		t.Error("Provider should be enabled")
	}

	// 测试 SupportedMethods
	methods := provider.SupportedMethods()
	if len(methods) != 2 {
		t.Errorf("Expected 2 supported methods, got %d", len(methods))
	}

	// 测试 Call
	result, err := provider.Call(ctx, "method1", []byte(`{}`))
	if err != nil {
		t.Errorf("Call() should not return error, got %v", err)
	}
	if string(result) != `{"result":"ok"}` {
		t.Errorf("Unexpected result: %s", string(result))
	}

	// 测试 Close
	err = provider.Close()
	if err != nil {
		t.Errorf("Close() should not return error, got %v", err)
	}
}

// TestMockProvider_Disabled 测试禁用的 Provider
func TestMockProvider_Disabled(t *testing.T) {
	provider := newMockProvider("mock_disabled", false, []string{})

	if provider.IsEnabled() {
		t.Error("Provider should be disabled")
	}
}

// TestMockProvider_NoMethods 测试没有方法的 Provider
func TestMockProvider_NoMethods(t *testing.T) {
	provider := newMockProvider("mock_no_methods", true, []string{})

	methods := provider.SupportedMethods()
	if len(methods) != 0 {
		t.Errorf("Expected 0 supported methods, got %d", len(methods))
	}
}
