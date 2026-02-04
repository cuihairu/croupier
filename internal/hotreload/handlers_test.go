package hotreload

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

// mockScriptInterpreter 模拟脚本解释器
type mockScriptInterpreter struct{}

func (m *mockScriptInterpreter) Execute(script []byte, context map[string]interface{}) (interface{}, error) {
	return nil, nil
}

func (m *mockScriptInterpreter) Validate(script []byte) error {
	return nil
}

// TestNewGoPluginHandler 测试创建 Go 插件处理器
func TestNewGoPluginHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewGoPluginHandler(logger)

	if handler == nil {
		t.Fatal("NewGoPluginHandler returned nil")
	}

	if handler.plugins == nil {
		t.Error("plugins map should be initialized")
	}

	if handler.logger != logger {
		t.Error("logger not set correctly")
	}
}

// TestGoPluginHandler_GetPlugin_NonExistent 测试获取不存在的插件
func TestGoPluginHandler_GetPlugin_NonExistent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewGoPluginHandler(logger)

	plugin, exists := handler.GetPlugin("/nonexistent/plugin.so")
	if exists {
		t.Error("Expected false for non-existent plugin")
	}

	if plugin != nil {
		t.Error("Expected nil plugin for non-existent path")
	}
}

// TestGoPluginHandler_Handle_IgnoreNonPlugin 测试忽略非插件文件
func TestGoPluginHandler_Handle_IgnoreNonPlugin(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewGoPluginHandler(logger)

	ctx := context.Background()
	event := ReloadEvent{
		Type: ReloadTypeConfig, // 不是插件类型
		Path: "/test/config.yaml",
	}

	err := handler.Handle(ctx, event)
	if err != nil {
		t.Errorf("Expected nil error for non-plugin event, got %v", err)
	}
}

// TestNewConfigHandler 测试创建配置处理器
func TestNewConfigHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewConfigHandler(logger)

	if handler == nil {
		t.Fatal("NewConfigHandler returned nil")
	}

	if handler.configStore == nil {
		t.Error("configStore should be initialized")
	}

	if handler.callbacks == nil {
		t.Error("callbacks should be initialized")
	}
}

// TestConfigHandler_GetConfig_NonExistent 测试获取不存在的配置
func TestConfigHandler_GetConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewConfigHandler(logger)

	config, exists := handler.GetConfig("/nonexistent/config.yaml")
	if exists {
		t.Error("Expected false for non-existent config")
	}

	if config != nil {
		t.Error("Expected nil config for non-existent path")
	}
}

// TestNewScriptHandler 测试创建脚本处理器
func TestNewScriptHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewScriptHandler(logger)

	if handler == nil {
		t.Fatal("NewScriptHandler returned nil")
	}

	if handler.scripts == nil {
		t.Error("scripts should be initialized")
	}

	if handler.interpreters == nil {
		t.Error("interpreters should be initialized")
	}
}

// TestScriptHandler_ExecuteScript_NonExistent 测试执行不存在的脚本
func TestScriptHandler_ExecuteScript_NonExistent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewScriptHandler(logger)

	_, err := handler.ExecuteScript("/nonexistent/script.lua", nil)
	if err == nil {
		t.Error("Expected error for non-existent script")
	}
}

// TestScriptHandler_RegisterInterpreter 测试注册解释器
func TestScriptHandler_RegisterInterpreter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewScriptHandler(logger)

	// 注册一个模拟的解释器
	mockInterpreter := &mockScriptInterpreter{}
	handler.RegisterInterpreter(".lua", mockInterpreter)

	// 验证解释器已注册
	if len(handler.interpreters) != 1 {
		t.Errorf("Expected 1 interpreter, got %d", len(handler.interpreters))
	}
}

// TestNewAssetHandler 测试创建资源处理器
func TestNewAssetHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewAssetHandler(logger)

	if handler == nil {
		t.Fatal("NewAssetHandler returned nil")
	}

	if handler.assets == nil {
		t.Error("assets should be initialized")
	}

	if handler.assetHooks == nil {
		t.Error("assetHooks should be initialized")
	}
}

// TestAssetHandler_GetAsset_NonExistent 测试获取不存在的资源
func TestAssetHandler_GetAsset(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewAssetHandler(logger)

	asset, exists := handler.GetAsset("/nonexistent/asset.png")
	if exists {
		t.Error("Expected false for non-existent asset")
	}

	if asset != nil {
		t.Error("Expected nil asset for non-existent path")
	}
}

// TestAssetHandler_assetHooksField 测试 assetHooks 字段初始化
func TestAssetHandler_assetHooksField(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewAssetHandler(logger)

	if handler.assetHooks == nil {
		t.Error("assetHooks should be initialized")
	}

	if len(handler.assetHooks) != 0 {
		t.Errorf("Expected empty assetHooks map, got %d entries", len(handler.assetHooks))
	}
}

// TestNewHandlerManager 测试创建处理器管理器
func TestNewHandlerManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewHandlerManager(logger)

	if manager == nil {
		t.Fatal("NewHandlerManager returned nil")
	}

	if manager.pluginHandler == nil {
		t.Error("pluginHandler should be initialized")
	}

	if manager.configHandler == nil {
		t.Error("configHandler should be initialized")
	}

	if manager.scriptHandler == nil {
		t.Error("scriptHandler should be initialized")
	}

	if manager.assetHandler == nil {
		t.Error("assetHandler should be initialized")
	}
}

// TestHandlerManager_GetHandlers 测试获取各个处理器
func TestHandlerManager_GetHandlers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewHandlerManager(logger)

	pluginHandler := manager.GetPluginHandler()
	if pluginHandler == nil {
		t.Error("GetPluginHandler should not return nil")
	}

	configHandler := manager.GetConfigHandler()
	if configHandler == nil {
		t.Error("GetConfigHandler should not return nil")
	}

	scriptHandler := manager.GetScriptHandler()
	if scriptHandler == nil {
		t.Error("GetScriptHandler should not return nil")
	}

	assetHandler := manager.GetAssetHandler()
	if assetHandler == nil {
		t.Error("GetAssetHandler should not return nil")
	}
}

// TestGoPluginHandler_Handle_NonSoFile 测试处理非 .so 文件
func TestGoPluginHandler_Handle_NonSoFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewGoPluginHandler(logger)

	ctx := context.Background()
	event := ReloadEvent{
		Type: ReloadTypePlugin,
		Path: "/test/plugin.txt", // 不是 .so 文件
	}

	err := handler.Handle(ctx, event)
	if err != nil {
		t.Errorf("Expected nil error for non-.so file, got %v", err)
	}

	// 验证插件没有被加载
	_, exists := handler.GetPlugin("/test/plugin.txt")
	if exists {
		t.Error("Plugin should not be loaded for non-.so file")
	}
}
