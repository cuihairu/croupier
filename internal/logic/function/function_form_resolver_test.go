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
	baseContent := "player.ban:\n  type: object\n  properties:\n    reason:\n      type: string\n      title: Reason\n      x-component: Input.TextArea\n      x-decorator: FormItem\n"
	if err := os.WriteFile(basePath, []byte(baseContent), 0o644); err != nil {
		t.Fatalf("write base config: %v", err)
	}

	overridePath := filepath.Join(overrideDir, "player.ban.yaml")
	overrideContent := "player.ban:\n  type: object\n  properties:\n    reason:\n      type: string\n      title: Reason\n      x-component: Select\n      x-decorator: FormItem\n"
	if err := os.WriteFile(overridePath, []byte(overrideContent), 0o644); err != nil {
		t.Fatalf("write override config: %v", err)
	}

	cfg := config.Config{}
	t.Setenv("CROUPIER_FORM_CONFIG_DIR", root)
	got := loadFormConfigFromFiles(cfg, "player.ban")
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map form config, got %T", got)
	}

	props, ok := m["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties, got %#v", m)
	}
	reason, ok := props["reason"].(map[string]interface{})
	if !ok || reason["x-component"] != "Select" {
		t.Fatalf("expected override reason.x-component=Select, got %#v", props["reason"])
	}
}

func TestResolveFunctionForm_SourcePriority(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "functions")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdir base dir: %v", err)
	}
	basePath := filepath.Join(baseDir, "player.ban.yaml")
	baseContent := "player.ban:\n  type: object\n  properties:\n    reason:\n      type: string\n      title: Reason\n      x-component: Input\n      x-decorator: FormItem\n"
	if err := os.WriteFile(basePath, []byte(baseContent), 0o644); err != nil {
		t.Fatalf("write base config: %v", err)
	}

	t.Setenv("CROUPIER_FORM_CONFIG_DIR", root)
	cfg := config.Config{}
	fn := &model.Function{
		FunctionID: "player.ban",
		Metadata:   datatypes.JSONMap{"form": testFunctionFormilySchema("reason", "Input.TextArea")},
		OpenAPISpec: datatypes.JSONMap{
			"x-ui": testFunctionFormilySchema("reason", "Input"),
		},
		Schema: datatypes.JSONMap{"from": "historical"},
	}

	resolved := resolveFunctionForm(cfg, fn)
	schema, ok := resolved.Schema.(map[string]interface{})
	if !ok {
		t.Fatalf("expected resolved schema map, got %T", resolved.Schema)
	}
	props, _ := schema["properties"].(map[string]interface{})
	reason, _ := props["reason"].(map[string]interface{})
	if reason["x-component"] != "Input.TextArea" {
		t.Fatalf("expected custom metadata form first, got %#v", schema)
	}
	if resolved.FormSource != "custom_metadata" {
		t.Fatalf("expected source custom_metadata, got %s", resolved.FormSource)
	}
}

func TestResolveFunctionForm_NoLegacySchemaFallback(t *testing.T) {
	cfg := config.Config{}
	fn := &model.Function{
		FunctionID: "player.ban",
		Schema:     datatypes.JSONMap{"from": "historical"},
	}

	resolved := resolveFunctionForm(cfg, fn)
	if resolved.Schema == nil {
		t.Fatalf("expected generated schema without openapi/custom form")
	}
	if !resolved.HasDefault {
		t.Fatalf("expected hasDefault=true when generated default form exists")
	}
	if resolved.FormSource != "generated_default" {
		t.Fatalf("expected formSource=generated_default, got %s", resolved.FormSource)
	}
}

func TestResolveFunctionForm_DerivesFromInputSchema(t *testing.T) {
	cfg := config.Config{}
	fn := &model.Function{
		FunctionID: "inventory.grant",
		OpenAPISpec: datatypes.JSONMap{
			"requestBody": map[string]interface{}{
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"playerId": map[string]interface{}{
									"type":        "string",
									"description": "Player ID",
								},
								"itemId": map[string]interface{}{
									"type":        "string",
									"description": "Item ID",
								},
								"amount": map[string]interface{}{
									"type":        "integer",
									"description": "Amount to grant",
									"minimum":     1,
								},
							},
							"required": []interface{}{"playerId", "itemId"},
						},
					},
				},
			},
		},
	}

	resolved := resolveFunctionForm(cfg, fn)
	if resolved.Schema == nil {
		t.Fatalf("expected schema derived from input_schema")
	}

	schema, ok := resolved.Schema.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map schema, got %T", resolved.Schema)
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties in schema")
	}

	// Should have the fields from input_schema, not the hardcoded fallback
	if _, ok := props["playerId"]; !ok {
		t.Fatalf("expected playerId property from input_schema")
	}
	if _, ok := props["itemId"]; !ok {
		t.Fatalf("expected itemId property from input_schema")
	}
	if _, ok := props["amount"]; !ok {
		t.Fatalf("expected amount property from input_schema")
	}

	// Check that amount has min constraint in x-component-props
	amount, _ := props["amount"].(map[string]interface{})
	compProps, _ := amount["x-component-props"].(map[string]interface{})
	if compProps["min"] != 1 {
		t.Fatalf("expected amount.x-component-props.min=1, got %v", compProps["min"])
	}

	// Check that fields have x-component
	if amount["x-component"] != "NumberPicker" {
		t.Fatalf("expected amount.x-component=NumberPicker, got %v", amount["x-component"])
	}
	playerId, _ := props["playerId"].(map[string]interface{})
	if playerId["x-component"] != "Input" {
		t.Fatalf("expected playerId.x-component=Input, got %v", playerId["x-component"])
	}

	// Check required fields
	req, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("expected required to be []string, got %T", schema["required"])
	}
	if len(req) != 2 {
		t.Fatalf("expected 2 required fields, got %d", len(req))
	}

	if resolved.FormSource != "generated_default" {
		t.Fatalf("expected formSource=generated_default, got %s", resolved.FormSource)
	}
}

func TestExtractInputSchema_FromOpenAPISpec(t *testing.T) {
	fn := &model.Function{
		FunctionID: "test.func",
		OpenAPISpec: datatypes.JSONMap{
			"requestBody": map[string]interface{}{
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"id": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
		},
	}

	schema := extractInputSchema(fn)
	if schema == nil {
		t.Fatalf("expected schema from OpenAPISpec")
	}
	if schema["type"] != "object" {
		t.Fatalf("expected type=object, got %v", schema["type"])
	}
}

func TestExtractInputSchema_FromMetadata(t *testing.T) {
	fn := &model.Function{
		FunctionID: "test.func",
		Metadata: datatypes.JSONMap{
			"input_schema": `{"type":"object","properties":{"name":{"type":"string"}}}`,
		},
	}

	schema := extractInputSchema(fn)
	if schema == nil {
		t.Fatalf("expected schema from metadata.input_schema")
	}
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties")
	}
	if _, ok := props["name"]; !ok {
		t.Fatalf("expected name property")
	}
}

func TestDeriveUISchemaFromJSONSchema_Empty(t *testing.T) {
	// nil schema
	if result := deriveFormSchemaFromJSONSchema(nil); result != nil {
		t.Fatalf("expected nil for nil schema")
	}

	// non-object schema
	if result := deriveFormSchemaFromJSONSchema(map[string]interface{}{"type": "string"}); result != nil {
		t.Fatalf("expected nil for non-object schema")
	}

	// object without properties
	if result := deriveFormSchemaFromJSONSchema(map[string]interface{}{"type": "object"}); result != nil {
		t.Fatalf("expected nil for object without properties")
	}
}
