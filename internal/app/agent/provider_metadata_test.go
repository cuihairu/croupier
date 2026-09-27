package agent

// #10/#27① 回归：providers.yaml 的 provider 条目 metadata 块随注册上报——
// 用户 KV 原样并入 Instance.Metadata，保留键剥离并告警，scope 固定键不被
// 覆盖。此前 openapi provider 注册只带 gameId/env 两个平台键，demo/生产
// openapi 实例在 sdk-distribution 无元数据可展示。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/agentlocal"
)

func loadProviderWithMetadata(t *testing.T, extraYAML string) *agentlocal.LocalStore {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}))
	t.Cleanup(server.Close)

	tmpDir := t.TempDir()
	configContent := `
providers:
  test_server:
    enabled: true
    type: openapi
` + extraYAML + `    config:
      base_url: "` + server.URL + `"
      methods:
        - name: get_user
          path: /api/user
          method: GET
`
	configPath := filepath.Join(tmpDir, "providers.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	store := agentlocal.NewLocalStore()
	pm := NewProviderManager(store, tmpDir, nil)
	if err := pm.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return store
}

func TestProviderInitCarriesUserMetadata(t *testing.T) {
	store := loadProviderWithMetadata(t, `    game_id: demo_game
    env: dev
    metadata:
      serverId: "demo-1"
      pod: "p-7"
`)

	snapshot := store.List()
	instances, ok := snapshot["test_server.get_user"]
	if !ok || len(instances) == 0 {
		t.Fatal("provider instance not registered")
	}
	meta := instances[0].Metadata
	if meta["serverId"] != "demo-1" || meta["pod"] != "p-7" {
		t.Fatalf("user metadata not carried: %v", meta)
	}
	if meta["gameId"] != "demo_game" || meta["env"] != "dev" {
		t.Fatalf("scope keys missing: %v", meta)
	}
}

func TestProviderInitDropsReservedMetadataKeys(t *testing.T) {
	store := loadProviderWithMetadata(t, `    game_id: demo_game
    env: dev
    metadata:
      gameId: "hijack"
      sdkLanguage: "rust"
      serverId: "keep-me"
`)

	snapshot := store.List()
	instances := snapshot["test_server.get_user"]
	if len(instances) == 0 {
		t.Fatal("provider instance not registered")
	}
	meta := instances[0].Metadata
	if meta["gameId"] != "demo_game" {
		t.Fatalf("reserved gameId must not be overridden: %v", meta)
	}
	if meta["sdkLanguage"] == "rust" {
		t.Fatalf("reserved sdkLanguage must be dropped from user metadata: %v", meta)
	}
	if meta["serverId"] != "keep-me" {
		t.Fatalf("non-reserved user key must survive: %v", meta)
	}
}
