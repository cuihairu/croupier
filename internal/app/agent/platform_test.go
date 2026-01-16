// Package agent provides platform integration for Agent.
package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"gopkg.in/yaml.v3"
)

func TestNewPlatformManager(t *testing.T) {
	store := agentlocal.NewLocalStore()
	pm := NewPlatformManager(store, "/tmp", nil)

	if pm == nil {
		t.Fatal("NewPlatformManager() returned nil")
	}
	if pm.providers == nil {
		t.Error("providers map should be initialized")
	}
	if pm.store != store {
		t.Error("store should be set")
	}
	if pm.configDir != "/tmp" {
		t.Errorf("configDir = %q, want %q", pm.configDir, "/tmp")
	}
	if pm.logger == nil {
		t.Error("logger should have default value")
	}
}

func TestPlatformManagerLoadNoConfig(t *testing.T) {
	store := agentlocal.NewLocalStore()
	pm := NewPlatformManager(store, "/nonexistent/path", nil)

	err := pm.Load(context.Background())
	if err != nil {
		t.Errorf("Load() should not error when config doesn't exist: %v", err)
	}
}

func TestPlatformManagerLoadWithConfig(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data":   map[string]string{"id": "123"},
		})
	}))
	defer server.Close()

	// Create temp directory with config
	tmpDir, err := os.MkdirTemp("", "platform-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configContent := `
platforms:
  test_server:
    enabled: true
    type: openapi
    config:
      base_url: "` + server.URL + `"
      methods:
        - name: get_user
          path: /api/user
          method: GET
        - name: create_user
          path: /api/user
          method: POST
`
	configPath := filepath.Join(tmpDir, "platforms.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store := agentlocal.NewLocalStore()
	pm := NewPlatformManager(store, tmpDir, nil)

	if err := pm.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check provider was loaded
	if len(pm.providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(pm.providers))
	}

	if _, ok := pm.providers["test_server"]; !ok {
		t.Error("test_server provider not found")
	}
}

func TestPlatformManagerLoadDisabledPlatform(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "platform-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configContent := `
platforms:
  disabled_server:
    enabled: false
    type: openapi
    config:
      base_url: "http://example.com"
      methods:
        - name: test
          path: /test
          method: GET
`
	configPath := filepath.Join(tmpDir, "platforms.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store := agentlocal.NewLocalStore()
	pm := NewPlatformManager(store, tmpDir, nil)

	if err := pm.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(pm.providers) != 0 {
		t.Errorf("expected 0 providers (disabled), got %d", len(pm.providers))
	}
}

func TestPlatformManagerCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "123",
			"name": "Test User",
		})
	}))
	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "platform-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configContent := `
platforms:
  game_server:
    enabled: true
    type: openapi
    config:
      base_url: "` + server.URL + `"
      methods:
        - name: get_player
          path: /api/player
          method: GET
`
	configPath := filepath.Join(tmpDir, "platforms.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store := agentlocal.NewLocalStore()
	pm := NewPlatformManager(store, tmpDir, nil)

	if err := pm.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Test successful call
	resp, err := pm.Call(context.Background(), "game_server.get_player", nil)
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result["id"] != "123" {
		t.Errorf("result.id = %v, want '123'", result["id"])
	}
}

func TestPlatformManagerCallNotFound(t *testing.T) {
	store := agentlocal.NewLocalStore()
	pm := NewPlatformManager(store, "/tmp", nil)

	_, err := pm.Call(context.Background(), "nonexistent.method", nil)
	if err == nil {
		t.Error("Call() should return error for nonexistent platform")
	}
}

func TestPlatformManagerCallInvalidFunctionID(t *testing.T) {
	store := agentlocal.NewLocalStore()
	pm := NewPlatformManager(store, "/tmp", nil)

	tests := []string{
		"invalid",
		"",
		"no_dot_here",
	}

	for _, funcID := range tests {
		_, err := pm.Call(context.Background(), funcID, nil)
		if err == nil {
			t.Errorf("Call(%q) should return error for invalid function ID", funcID)
		}
	}
}

func TestPlatformManagerIsPlatformFunction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "platform-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configContent := `
platforms:
  game_server:
    enabled: true
    type: openapi
    config:
      base_url: "` + server.URL + `"
      methods:
        - name: test
          path: /test
          method: GET
`
	configPath := filepath.Join(tmpDir, "platforms.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store := agentlocal.NewLocalStore()
	pm := NewPlatformManager(store, tmpDir, nil)

	if err := pm.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		functionID string
		want       bool
	}{
		{"game_server.test", true},
		{"game_server.other", true}, // Platform exists, even if method doesn't
		{"other_platform.test", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.functionID, func(t *testing.T) {
			got := pm.IsPlatformFunction(tt.functionID)
			if got != tt.want {
				t.Errorf("IsPlatformFunction(%q) = %v, want %v", tt.functionID, got, tt.want)
			}
		})
	}
}

func TestPlatformManagerClose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "platform-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configContent := `
platforms:
  test1:
    enabled: true
    type: openapi
    config:
      base_url: "` + server.URL + `"
      methods:
        - name: m1
          path: /m1
          method: GET
  test2:
    enabled: true
    type: openapi
    config:
      base_url: "` + server.URL + `"
      methods:
        - name: m2
          path: /m2
          method: GET
`
	configPath := filepath.Join(tmpDir, "platforms.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store := agentlocal.NewLocalStore()
	pm := NewPlatformManager(store, tmpDir, nil)

	if err := pm.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(pm.providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(pm.providers))
	}

	if err := pm.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if len(pm.providers) != 0 {
		t.Errorf("providers should be cleared after Close(), got %d", len(pm.providers))
	}
}

func TestExpandEnvVars(t *testing.T) {
	store := agentlocal.NewLocalStore()
	pm := NewPlatformManager(store, "/tmp", nil)

	// Set test env var
	os.Setenv("TEST_TOKEN", "secret-token-123")
	defer os.Unsetenv("TEST_TOKEN")

	tests := []struct {
		name  string
		input map[string]interface{}
		want  map[string]interface{}
	}{
		{
			name: "simple env var",
			input: map[string]interface{}{
				"token": "${TEST_TOKEN}",
			},
			want: map[string]interface{}{
				"token": "secret-token-123",
			},
		},
		{
			name: "nested config",
			input: map[string]interface{}{
				"auth": map[string]interface{}{
					"token": "${TEST_TOKEN}",
				},
			},
			want: map[string]interface{}{
				"auth": map[string]interface{}{
					"token": "secret-token-123",
				},
			},
		},
		{
			name: "no env var",
			input: map[string]interface{}{
				"value": "plain-value",
			},
			want: map[string]interface{}{
				"value": "plain-value",
			},
		},
		{
			name: "undefined env var",
			input: map[string]interface{}{
				"value": "${UNDEFINED_VAR}",
			},
			want: map[string]interface{}{
				"value": "", // os.ExpandEnv expands undefined vars to empty string
			},
		},
		{
			name: "non-string values",
			input: map[string]interface{}{
				"number": 123,
				"bool":   true,
				"array":  []interface{}{"a", "b"},
			},
			want: map[string]interface{}{
				"number": 123,
				"bool":   true,
				"array":  []interface{}{"a", "b"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pm.expandEnvVars(tt.input)

			// Simple comparison for top-level values
			for k, wantV := range tt.want {
				gotV := got[k]
				switch wv := wantV.(type) {
				case string:
					if gv, ok := gotV.(string); !ok || gv != wv {
						t.Errorf("expandEnvVars()[%q] = %v, want %v", k, gotV, wantV)
					}
				case map[string]interface{}:
					if gm, ok := gotV.(map[string]interface{}); ok {
						for nk, nv := range wv {
							if gm[nk] != nv {
								t.Errorf("expandEnvVars()[%q][%q] = %v, want %v", k, nk, gm[nk], nv)
							}
						}
					}
				}
			}
		})
	}
}

func TestExpandEnvString(t *testing.T) {
	store := agentlocal.NewLocalStore()
	pm := NewPlatformManager(store, "/tmp", nil)

	os.Setenv("TEST_VAR", "test-value")
	defer os.Unsetenv("TEST_VAR")

	tests := []struct {
		input string
		want  string
	}{
		{"${TEST_VAR}", "test-value"},
		{"plain-string", "plain-string"},
		{"${UNDEFINED}", ""}, // os.ExpandEnv expands undefined to empty
		{"prefix-${TEST_VAR}-suffix", "prefix-test-value-suffix"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := pm.expandEnvString(tt.input)
			if got != tt.want {
				t.Errorf("expandEnvString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPlatformManagerLoadInvalidYAML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "platform-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configContent := `invalid: yaml: content: [[[`
	configPath := filepath.Join(tmpDir, "platforms.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store := agentlocal.NewLocalStore()
	pm := NewPlatformManager(store, tmpDir, nil)

	err = pm.Load(context.Background())
	if err == nil {
		t.Error("Load() should return error for invalid YAML")
	}
}

func TestPlatformManagerLoadUnsupportedType(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "platform-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configContent := `
platforms:
  unsupported:
    enabled: true
    type: unknown_type
    config:
      base_url: "http://example.com"
`
	configPath := filepath.Join(tmpDir, "platforms.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	store := agentlocal.NewLocalStore()
	pm := NewPlatformManager(store, tmpDir, nil)

	// Should not return error, just log warning for unsupported type
	err = pm.Load(context.Background())
	if err != nil {
		t.Errorf("Load() should not fail for unsupported type, got: %v", err)
	}

	// Provider should not be loaded
	if len(pm.providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(pm.providers))
	}
}

func TestConfigParsing(t *testing.T) {
	configYAML := `
platforms:
  server1:
    enabled: true
    type: openapi
    config:
      key: value
  server2:
    enabled: false
    type: openapi
    config:
      key: value2
`
	var config Config
	err := yaml.Unmarshal([]byte(configYAML), &config)
	if err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	if len(config.Platforms) != 2 {
		t.Errorf("expected 2 platforms, got %d", len(config.Platforms))
	}

	if !config.Platforms["server1"].Enabled {
		t.Error("server1 should be enabled")
	}

	if config.Platforms["server2"].Enabled {
		t.Error("server2 should be disabled")
	}

	if config.Platforms["server1"].Type != "openapi" {
		t.Errorf("server1.Type = %q, want 'openapi'", config.Platforms["server1"].Type)
	}
}
