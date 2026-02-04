package provider

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
)

// TestNewRegistry 测试创建 Registry
func TestNewRegistry(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := NewRegistry(logger)

	if reg == nil {
		t.Fatal("NewRegistry() should return non-nil Registry")
	}

	if reg.providers == nil {
		t.Error("providers map should be initialized")
	}

	if reg.logger == nil {
		t.Error("logger should be set")
	}
}

// TestNewRegistry_NilLogger 测试使用 nil logger
func TestNewRegistry_NilLogger(t *testing.T) {
	reg := NewRegistry(nil)

	if reg == nil {
		t.Fatal("NewRegistry(nil) should return non-nil Registry")
	}

	if reg.logger == nil {
		t.Error("logger should default to slog.Default()")
	}
}

// TestRegistry_Register 测试注册 Provider
func TestRegistry_Register(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := NewRegistry(logger)

	provider := newMockProvider("test_provider", true, []string{"method1"})
	config := ProviderConfig{
		Enabled: true,
		Type:    "test",
		Config:  make(map[string]interface{}),
	}

	err := reg.Register(ctx, provider, config)
	if err != nil {
		t.Errorf("Register() should not return error, got %v", err)
	}

	// 验证 provider 已注册
	p, exists := reg.Get("test_provider")
	if !exists {
		t.Error("Provider should be registered")
	}

	if p == nil {
		t.Error("Retrieved provider should not be nil")
	}
}

// TestRegistry_Register_Duplicate 测试重复注册
func TestRegistry_Register_Duplicate(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := NewRegistry(logger)

	provider1 := newMockProvider("duplicate", true, []string{})
	provider2 := newMockProvider("duplicate", true, []string{})
	config := ProviderConfig{Enabled: true}

	err := reg.Register(ctx, provider1, config)
	if err != nil {
		t.Errorf("First Register() should succeed, got %v", err)
	}

	err = reg.Register(ctx, provider2, config)
	if err == nil {
		t.Error("Second Register() with same name should return error")
	}
}

// TestRegistry_Unregister 测试注销 Provider
func TestRegistry_Unregister(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := NewRegistry(logger)

	provider := newMockProvider("to_unregister", true, []string{})
	config := ProviderConfig{Enabled: true}

	reg.Register(ctx, provider, config)

	err := reg.Unregister(ctx, "to_unregister")
	if err != nil {
		t.Errorf("Unregister() should not return error, got %v", err)
	}

	// 验证 provider 已注销
	_, exists := reg.Get("to_unregister")
	if exists {
		t.Error("Provider should be unregistered")
	}
}

// TestRegistry_Unregister_NotFound 测试注销不存在的 Provider
func TestRegistry_Unregister_NotFound(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := NewRegistry(logger)

	err := reg.Unregister(ctx, "nonexistent")
	if err == nil {
		t.Error("Unregister() non-existent provider should return error")
	}

	if _, ok := err.(*ProviderNotFoundError); !ok {
		t.Errorf("Expected ProviderNotFoundError, got %T", err)
	}
}

// TestRegistry_Get 测试获取 Provider
func TestRegistry_Get(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := NewRegistry(logger)

	// 测试获取不存在的 provider
	_, exists := reg.Get("nonexistent")
	if exists {
		t.Error("Get() non-existent provider should return false")
	}

	// 注册一个 provider 并测试获取
	ctx := context.Background()
	provider := newMockProvider("get_test", true, []string{})
	config := ProviderConfig{Enabled: true}

	reg.Register(ctx, provider, config)

	p, exists := reg.Get("get_test")
	if !exists {
		t.Error("Get() registered provider should return true")
	}

	if p == nil {
		t.Error("Get() should return non-nil provider")
	}
}

// TestRegistry_List 测试列出所有 Providers
func TestRegistry_List(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := NewRegistry(logger)

	// 注册多个 providers
	providers := []struct {
		name    string
		enabled bool
		methods []string
	}{
		{"p1", true, []string{"m1"}},
		{"p2", false, []string{"m2"}},
		{"p3", true, []string{"m3"}},
	}

	for _, p := range providers {
		provider := newMockProvider(p.name, p.enabled, p.methods)
		config := ProviderConfig{Enabled: p.enabled}
		reg.Register(ctx, provider, config)
	}

	list := reg.List()
	if len(list) != 3 {
		t.Errorf("List() should return 3 providers, got %d", len(list))
	}
}

// TestRegistry_ListNames 测试列出所有 Provider 名称
func TestRegistry_ListNames(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := NewRegistry(logger)

	provider := newMockProvider("names_test", true, []string{})
	config := ProviderConfig{Enabled: true}

	reg.Register(ctx, provider, config)

	names := reg.ListNames()
	if len(names) != 1 {
		t.Errorf("ListNames() should return 1 name, got %d", len(names))
	}

	if names[0] != "names_test" {
		t.Errorf("Expected name 'names_test', got '%s'", names[0])
	}
}

// TestRegistry_Call 测试调用 Provider 方法
func TestRegistry_Call(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := NewRegistry(logger)

	provider := newMockProvider("caller", true, []string{"test_method"})
	config := ProviderConfig{Enabled: true}

	reg.Register(ctx, provider, config)

	// 测试成功调用
	result, err := reg.Call(ctx, "caller", "test_method", []byte(`{"test":true}`))
	if err != nil {
		t.Errorf("Call() should not return error, got %v", err)
	}

	if result == nil {
		t.Error("Call() should return non-nil result")
	}
}

// TestRegistry_Call_ProviderNotFound 测试调用不存在的 Provider
func TestRegistry_Call_ProviderNotFound(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := NewRegistry(logger)

	_, err := reg.Call(ctx, "nonexistent", "method", []byte{})
	if err == nil {
		t.Error("Call() non-existent provider should return error")
	}

	if _, ok := err.(*ProviderNotFoundError); !ok {
		t.Errorf("Expected ProviderNotFoundError, got %T", err)
	}
}

// TestRegistry_Call_ProviderDisabled 测试调用禁用的 Provider
func TestRegistry_Call_ProviderDisabled(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := NewRegistry(logger)

	provider := newMockProvider("disabled", false, []string{"method"})
	config := ProviderConfig{Enabled: false}

	reg.Register(ctx, provider, config)

	_, err := reg.Call(ctx, "disabled", "method", []byte{})
	if err == nil {
		t.Error("Call() disabled provider should return error")
	}

	if _, ok := err.(*ProviderDisabledError); !ok {
		t.Errorf("Expected ProviderDisabledError, got %T", err)
	}
}

// TestRegistry_Call_MethodNotSupported 测试不支持的方法
func TestRegistry_Call_MethodNotSupported(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := NewRegistry(logger)

	provider := newMockProvider("limited", true, []string{"supported_method"})
	config := ProviderConfig{Enabled: true}

	reg.Register(ctx, provider, config)

	_, err := reg.Call(ctx, "limited", "unsupported_method", []byte{})
	if err == nil {
		t.Error("Call() unsupported method should return error")
	}

	if _, ok := err.(*MethodNotSupportedError); !ok {
		t.Errorf("Expected MethodNotSupportedError, got %T", err)
	}
}

// TestRegistry_Close 测试关闭 Registry
func TestRegistry_Close(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := NewRegistry(logger)

	ctx := context.Background()
	provider := newMockProvider("close_test", true, []string{})
	config := ProviderConfig{Enabled: true}

	reg.Register(ctx, provider, config)

	err := reg.Close()
	if err != nil {
		t.Errorf("Close() should not return error, got %v", err)
	}

	// 验证所有 providers 已清除
	list := reg.List()
	if len(list) != 0 {
		t.Errorf("After Close(), List() should return empty, got %d providers", len(list))
	}
}

// TestRegistry_ConcurrentOperations 测试并发操作
func TestRegistry_ConcurrentOperations(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := NewRegistry(logger)

	var wg sync.WaitGroup

	// 并发注册
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "concurrent_" + string(rune('a'+idx))
			provider := newMockProvider(name, true, []string{})
			config := ProviderConfig{Enabled: true}
			reg.Register(ctx, provider, config)
		}(i)
	}

	// 并发读取
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reg.List()
			reg.ListNames()
		}()
	}

	wg.Wait()

	// 验证所有 providers 都已注册
	list := reg.List()
	if len(list) != 10 {
		t.Errorf("Expected 10 providers after concurrent registration, got %d", len(list))
	}
}

// TestRegistry_Call_EmptyRequest 测试空请求
func TestRegistry_Call_EmptyRequest(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := NewRegistry(logger)

	provider := newMockProvider("empty_req", true, []string{"test"})
	config := ProviderConfig{Enabled: true}

	reg.Register(ctx, provider, config)

	result, err := reg.Call(ctx, "empty_req", "test", []byte{})
	if err != nil {
		t.Errorf("Call() with empty request should not return error, got %v", err)
	}

	if result == nil {
		t.Error("Call() should return non-nil result even with empty request")
	}
}
