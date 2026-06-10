package hotreload

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stdout, nil))

// --- ConfigHandler.Handle ---

func TestConfigHandler_Handle_JSON(t *testing.T) {
	h := NewConfigHandler(testLogger)
	ctx := context.Background()

	event := ReloadEvent{
		Type:    ReloadTypeConfig,
		Path:    "/config.json",
		Content: []byte(`{"key":"value"}`),
	}

	if err := h.Handle(ctx, event); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	cfg, ok := h.GetConfig("/config.json")
	if !ok {
		t.Fatal("config should exist")
	}
	m := cfg.(map[string]interface{})
	if m["key"] != "value" {
		t.Errorf("expected value, got %v", m["key"])
	}
}

func TestConfigHandler_Handle_YAML(t *testing.T) {
	h := NewConfigHandler(testLogger)
	ctx := context.Background()

	event := ReloadEvent{
		Type:    ReloadTypeConfig,
		Path:    "/config.yaml",
		Content: []byte("key: value\n"),
	}

	if err := h.Handle(ctx, event); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	cfg, ok := h.GetConfig("/config.yaml")
	if !ok {
		t.Fatal("config should exist")
	}
	m := cfg.(map[string]interface{})
	if m["key"] != "value" {
		t.Errorf("expected value, got %v", m["key"])
	}
}

func TestConfigHandler_Handle_UnsupportedFormat(t *testing.T) {
	h := NewConfigHandler(testLogger)
	ctx := context.Background()

	event := ReloadEvent{
		Type:    ReloadTypeConfig,
		Path:    "/config.toml",
		Content: []byte("key = value"),
	}

	if err := h.Handle(ctx, event); err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestConfigHandler_Handle_InvalidJSON(t *testing.T) {
	h := NewConfigHandler(testLogger)
	ctx := context.Background()

	event := ReloadEvent{
		Type:    ReloadTypeConfig,
		Path:    "/bad.json",
		Content: []byte("{invalid}"),
	}

	if err := h.Handle(ctx, event); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestConfigHandler_Handle_InvalidYAML(t *testing.T) {
	h := NewConfigHandler(testLogger)
	ctx := context.Background()

	event := ReloadEvent{
		Type:    ReloadTypeConfig,
		Path:    "/bad.yaml",
		Content: []byte(":\n  :\n    - }"),
	}

	// YAML is very lenient, this might not error. Just ensure no panic.
	h.Handle(ctx, event)
}

func TestConfigHandler_Handle_IgnoreNonConfig(t *testing.T) {
	h := NewConfigHandler(testLogger)
	ctx := context.Background()

	event := ReloadEvent{Type: ReloadTypeAsset, Path: "/test.png"}
	if err := h.Handle(ctx, event); err != nil {
		t.Errorf("should ignore non-config: %v", err)
	}
}

func TestConfigHandler_RegisterCallback(t *testing.T) {
	h := NewConfigHandler(testLogger)
	ctx := context.Background()

	var callbackCalled bool
	h.RegisterCallback("/cb.json", func(old, new interface{}) error {
		callbackCalled = true
		return nil
	})

	event := ReloadEvent{
		Type:    ReloadTypeConfig,
		Path:    "/cb.json",
		Content: []byte(`{"a":1}`),
	}

	if err := h.Handle(ctx, event); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !callbackCalled {
		t.Error("callback should have been called")
	}
}

func TestConfigHandler_RegisterCallback_Error(t *testing.T) {
	h := NewConfigHandler(testLogger)
	ctx := context.Background()

	h.RegisterCallback("/err.json", func(old, new interface{}) error {
		return errors.New("callback error")
	})

	event := ReloadEvent{
		Type:    ReloadTypeConfig,
		Path:    "/err.json",
		Content: []byte(`{"a":1}`),
	}

	// Should not return error (callback errors are logged, not propagated)
	if err := h.Handle(ctx, event); err != nil {
		t.Errorf("callback errors should be logged, not returned: %v", err)
	}
}

// --- ScriptHandler ---

func TestScriptHandler_Handle_WithInterpreter(t *testing.T) {
	h := NewScriptHandler(testLogger)
	h.RegisterInterpreter(".lua", &mockScriptInterpreter{})
	ctx := context.Background()

	event := ReloadEvent{
		Type:    ReloadTypeScript,
		Path:    "/script.lua",
		Content: []byte("print('hello')"),
	}

	if err := h.Handle(ctx, event); err != nil {
		t.Fatalf("Handle: %v", err)
	}
}

func TestScriptHandler_Handle_NoInterpreter(t *testing.T) {
	h := NewScriptHandler(testLogger)
	ctx := context.Background()

	event := ReloadEvent{
		Type:    ReloadTypeScript,
		Path:    "/script.py",
		Content: []byte("print('hello')"),
	}

	if err := h.Handle(ctx, event); err == nil {
		t.Error("expected error for missing interpreter")
	}
}

func TestScriptHandler_Handle_ValidationError(t *testing.T) {
	h := NewScriptHandler(testLogger)
	h.RegisterInterpreter(".lua", &mockFailValidateInterpreter{})
	ctx := context.Background()

	event := ReloadEvent{
		Type:    ReloadTypeScript,
		Path:    "/bad.lua",
		Content: []byte("invalid"),
	}

	if err := h.Handle(ctx, event); err == nil {
		t.Error("expected validation error")
	}
}

func TestScriptHandler_Handle_IgnoreNonScript(t *testing.T) {
	h := NewScriptHandler(testLogger)
	ctx := context.Background()

	event := ReloadEvent{Type: ReloadTypeAsset, Path: "/test.png"}
	if err := h.Handle(ctx, event); err != nil {
		t.Errorf("should ignore non-script: %v", err)
	}
}

func TestScriptHandler_ExecuteScript(t *testing.T) {
	h := NewScriptHandler(testLogger)
	h.RegisterInterpreter(".lua", &mockScriptInterpreter{})

	// Manually add a script
	h.scripts["/test.lua"] = []byte("return 42")

	result, err := h.ExecuteScript("/test.lua", nil)
	if err != nil {
		t.Fatalf("ExecuteScript: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result from mock, got %v", result)
	}
}

func TestScriptHandler_ExecuteScript_NoInterpreter(t *testing.T) {
	h := NewScriptHandler(testLogger)
	h.scripts["/test.py"] = []byte("print(42)")

	_, err := h.ExecuteScript("/test.py", nil)
	if err == nil {
		t.Error("expected error for missing interpreter")
	}
}

// --- AssetHandler ---

func TestAssetHandler_Handle_WithHook(t *testing.T) {
	h := NewAssetHandler(testLogger)
	ctx := context.Background()

	var hookCalled bool
	h.RegisterHook("/sprite.png", func(path string, content []byte) error {
		hookCalled = true
		return nil
	})

	event := ReloadEvent{
		Type:    ReloadTypeAsset,
		Path:    "/sprite.png",
		Content: []byte("PNG data"),
	}

	if err := h.Handle(ctx, event); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !hookCalled {
		t.Error("hook should have been called")
	}

	asset, ok := h.GetAsset("/sprite.png")
	if !ok || string(asset) != "PNG data" {
		t.Error("asset should be stored")
	}
}

func TestAssetHandler_Handle_HookError(t *testing.T) {
	h := NewAssetHandler(testLogger)
	ctx := context.Background()

	h.RegisterHook("/bad.png", func(path string, content []byte) error {
		return errors.New("hook error")
	})

	event := ReloadEvent{
		Type:    ReloadTypeAsset,
		Path:    "/bad.png",
		Content: []byte("data"),
	}

	// Hook errors are logged, not propagated
	if err := h.Handle(ctx, event); err != nil {
		t.Errorf("hook errors should be logged, not returned: %v", err)
	}
}

func TestAssetHandler_Handle_IgnoreNonAsset(t *testing.T) {
	h := NewAssetHandler(testLogger)
	ctx := context.Background()

	event := ReloadEvent{Type: ReloadTypeConfig, Path: "/config.json"}
	if err := h.Handle(ctx, event); err != nil {
		t.Errorf("should ignore non-asset: %v", err)
	}
}

// --- HandlerManager ---

func TestHandlerManager_Handle_Config(t *testing.T) {
	m := NewHandlerManager(testLogger)
	ctx := context.Background()

	event := ReloadEvent{
		Type:    ReloadTypeConfig,
		Path:    "/test.json",
		Content: []byte(`{"a":1}`),
	}

	if err := m.Handle(ctx, event); err != nil {
		t.Fatalf("Handle config: %v", err)
	}
}

func TestHandlerManager_Handle_Asset(t *testing.T) {
	m := NewHandlerManager(testLogger)
	ctx := context.Background()

	event := ReloadEvent{
		Type:    ReloadTypeAsset,
		Path:    "/img.png",
		Content: []byte("data"),
	}

	if err := m.Handle(ctx, event); err != nil {
		t.Fatalf("Handle asset: %v", err)
	}
}

func TestHandlerManager_Handle_Script(t *testing.T) {
	m := NewHandlerManager(testLogger)
	m.GetScriptHandler().RegisterInterpreter(".lua", &mockScriptInterpreter{})
	ctx := context.Background()

	event := ReloadEvent{
		Type:    ReloadTypeScript,
		Path:    "/s.lua",
		Content: []byte("ok"),
	}

	if err := m.Handle(ctx, event); err != nil {
		t.Fatalf("Handle script: %v", err)
	}
}

func TestHandlerManager_Handle_Plugin_NonSO(t *testing.T) {
	m := NewHandlerManager(testLogger)
	ctx := context.Background()

	event := ReloadEvent{
		Type: ReloadTypePlugin,
		Path: "/plugin.txt",
	}

	if err := m.Handle(ctx, event); err != nil {
		t.Fatalf("Handle plugin non-so: %v", err)
	}
}

func TestHandlerManager_Handle_Unknown(t *testing.T) {
	m := NewHandlerManager(testLogger)
	ctx := context.Background()

	event := ReloadEvent{Type: "unknown", Path: "/test"}

	// Unknown types are warned but not errored
	if err := m.Handle(ctx, event); err != nil {
		t.Errorf("unknown type should not error: %v", err)
	}
}

// --- RegisterGameConfigHandler ---

func TestRegisterGameConfigHandler_JSON(t *testing.T) {
	hr := newTestHotReloader(t)
	var received *GameConfig

	err := RegisterGameConfigHandler(hr, "/game.json", func(gc *GameConfig) error {
		received = gc
		return nil
	})
	if err != nil {
		t.Fatalf("RegisterGameConfigHandler: %v", err)
	}

	// Simulate event
	for _, handler := range hr.handlers["/game.json"] {
		handler(context.Background(), ReloadEvent{
			Type:    ReloadTypeConfig,
			Path:    "/game.json",
			Content: []byte(`{"balance":{"player_hp":100},"rules":{"max_players":4}}`),
		})
	}

	if received == nil {
		t.Fatal("expected config to be received")
	}
	if received.Balance.PlayerHP != 100 {
		t.Errorf("PlayerHP = %d", received.Balance.PlayerHP)
	}
}

func TestRegisterGameConfigHandler_YAML(t *testing.T) {
	hr := newTestHotReloader(t)
	var received *GameConfig

	RegisterGameConfigHandler(hr, "/game.yaml", func(gc *GameConfig) error {
		received = gc
		return nil
	})

	for _, handler := range hr.handlers["/game.yaml"] {
		handler(context.Background(), ReloadEvent{
			Type:    ReloadTypeConfig,
			Path:    "/game.yaml",
			Content: []byte("balance:\n  player_hp: 200\n"),
		})
	}

	if received == nil || received.Balance.PlayerHP != 200 {
		t.Error("expected YAML config to be parsed")
	}
}

func TestRegisterGameConfigHandler_UnsupportedFormat(t *testing.T) {
	hr := newTestHotReloader(t)
	RegisterGameConfigHandler(hr, "/game.toml", func(gc *GameConfig) error { return nil })

	for _, handler := range hr.handlers["/game.toml"] {
		err := handler(context.Background(), ReloadEvent{
			Type:    ReloadTypeConfig,
			Path:    "/game.toml",
			Content: []byte("key=val"),
		})
		if err == nil {
			t.Error("expected error for unsupported format")
		}
	}
}

func TestRegisterGameConfigHandler_IgnoreNonConfig(t *testing.T) {
	hr := newTestHotReloader(t)
	var called bool
	RegisterGameConfigHandler(hr, "/game.json", func(gc *GameConfig) error {
		called = true
		return nil
	})

	for _, handler := range hr.handlers["/game.json"] {
		handler(context.Background(), ReloadEvent{
			Type: ReloadTypeAsset,
			Path: "/game.json",
		})
	}
	if called {
		t.Error("should ignore non-config events")
	}
}

// --- WatchGameAssets ---

func TestWatchGameAssets(t *testing.T) {
	hr := newTestHotReloader(t)
	var receivedPath string
	var receivedContent []byte

	err := WatchGameAssets(hr, "/assets", func(path string, content []byte) error {
		receivedPath = path
		receivedContent = content
		return nil
	})
	if err != nil {
		t.Fatalf("WatchGameAssets: %v", err)
	}

	// Find the registered handler
	pattern := filepath.Join("/assets", "*")
	for _, handler := range hr.handlers[pattern] {
		handler(context.Background(), ReloadEvent{
			Type:    ReloadTypeAsset,
			Path:    "/assets/sprite.png",
			Content: []byte("PNG"),
		})
	}

	if receivedPath != "/assets/sprite.png" {
		t.Errorf("path = %q", receivedPath)
	}
	if string(receivedContent) != "PNG" {
		t.Errorf("content = %q", string(receivedContent))
	}
}

func TestWatchGameAssets_IgnoreNonAsset(t *testing.T) {
	hr := newTestHotReloader(t)
	var called bool
	WatchGameAssets(hr, "/assets", func(path string, content []byte) error {
		called = true
		return nil
	})

	pattern2 := filepath.Join("/assets", "*")
	for _, handler := range hr.handlers[pattern2] {
		handler(context.Background(), ReloadEvent{
			Type: ReloadTypeConfig,
			Path: "/assets/config.json",
		})
	}
	if called {
		t.Error("should ignore non-asset events")
	}
}

// --- helpers ---

type mockFailValidateInterpreter struct{}

func (m *mockFailValidateInterpreter) Execute(script []byte, context map[string]interface{}) (interface{}, error) {
	return nil, nil
}

func (m *mockFailValidateInterpreter) Validate(script []byte) error {
	return errors.New("syntax error")
}

type testHotReloader struct {
	handlers map[string][]ReloadHandler
}

func newTestHotReloader(t *testing.T) *testHotReloader {
	t.Helper()
	return &testHotReloader{handlers: make(map[string][]ReloadHandler)}
}

func (r *testHotReloader) RegisterHandler(pattern string, handler ReloadHandler) error {
	r.handlers[pattern] = append(r.handlers[pattern], handler)
	return nil
}

func (r *testHotReloader) StartWatching(ctx context.Context) error { return nil }
func (r *testHotReloader) Reload(path string) error                { return nil }
func (r *testHotReloader) GetVersion() *VersionInfo                { return &VersionInfo{} }
func (r *testHotReloader) Stop() error                             { return nil }
