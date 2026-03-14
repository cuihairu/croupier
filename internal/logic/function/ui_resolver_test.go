package function

import (
	"os"
	"path/filepath"
	"testing"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
)

func TestLoadUIConfigFromFiles(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "functions")
	overrideDir := filepath.Join(root, "functions.override")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdir base dir: %v", err)
	}
	if err := os.MkdirAll(overrideDir, 0o755); err != nil {
		t.Fatalf("mkdir override dir: %v", err)
	}

	basePath := filepath.Join(baseDir, "player.ban.yaml")
	baseContent := "player.ban:\n  x-ui:\n    layout:\n      type: grid\n    fields:\n      reason:\n        widget: textarea\n"
	if err := os.WriteFile(basePath, []byte(baseContent), 0o644); err != nil {
		t.Fatalf("write base config: %v", err)
	}

	overridePath := filepath.Join(overrideDir, "player.ban.yaml")
	overrideContent := "player.ban:\n  x-ui:\n    layout:\n      type: vertical\n    fields:\n      reason:\n        widget: select\n"
	if err := os.WriteFile(overridePath, []byte(overrideContent), 0o644); err != nil {
		t.Fatalf("write override config: %v", err)
	}

	cfg := config.Config{}
	t.Setenv("CROUPIER_UI_CONFIG_DIR", root)
	got := loadUIConfigFromFiles(cfg, "player.ban")
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map ui config, got %T", got)
	}

	layout, ok := m["layout"].(map[string]interface{})
	if !ok || layout["type"] != "vertical" {
		t.Fatalf("expected override layout.type=vertical, got %#v", m["layout"])
	}
}

func TestResolveFunctionUI_SourcePriority(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "functions")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdir base dir: %v", err)
	}
	basePath := filepath.Join(baseDir, "player.ban.yaml")
	baseContent := "player.ban:\n  x-ui:\n    from: file\n"
	if err := os.WriteFile(basePath, []byte(baseContent), 0o644); err != nil {
		t.Fatalf("write base config: %v", err)
	}

	t.Setenv("CROUPIER_UI_CONFIG_DIR", root)
	cfg := config.Config{}
	fn := &model.Function{
		FunctionID: "player.ban",
		Metadata:   datatypes.JSONMap{"ui": map[string]interface{}{"from": "custom"}},
		OpenAPISpec: datatypes.JSONMap{
			"x-ui": map[string]interface{}{"from": "openapi"},
		},
		Schema: datatypes.JSONMap{"from": "historical"},
	}

	resolved := resolveFunctionUI(cfg, fn)
	schema, ok := resolved.Schema.(map[string]interface{})
	if !ok {
		t.Fatalf("expected resolved schema map, got %T", resolved.Schema)
	}
	if schema["from"] != "custom" {
		t.Fatalf("expected custom metadata ui first, got %#v", schema)
	}
	if resolved.UISource != "custom_metadata" {
		t.Fatalf("expected source custom_metadata, got %s", resolved.UISource)
	}
}

func TestResolveFunctionUI_NoLegacySchemaFallback(t *testing.T) {
	cfg := config.Config{}
	fn := &model.Function{
		FunctionID: "player.ban",
		Schema:     datatypes.JSONMap{"from": "historical"},
	}

	resolved := resolveFunctionUI(cfg, fn)
	if resolved.Schema != nil {
		t.Fatalf("expected schema to be nil without openapi/custom ui, got %#v", resolved.Schema)
	}
	if resolved.HasDefault {
		t.Fatalf("expected hasDefault=false when only historical schema exists")
	}
	if resolved.UISource != "none" {
		t.Fatalf("expected uiSource=none, got %s", resolved.UISource)
	}
}
