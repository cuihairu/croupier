package generator

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

func TestBuildFormPresentation_PayloadOnlyForMissingSchema(t *testing.T) {
	fp := buildFormPresentation(spec.OperationSpec{FunctionID: "players.player.list"}, GenerateOptions{
		Functions: map[string]spec.FunctionSpec{},
	})

	// schema：单 payload 字段 + required
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(fp.JSONSchema), &schema); err != nil {
		t.Fatalf("JSONSchema is not valid JSON: %v", err)
	}
	props, _ := schema["properties"].(map[string]interface{})
	if len(props) != 1 {
		t.Fatalf("expected exactly one property, got %v", schema)
	}
	payload, ok := props["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected payload property, got %v", schema)
	}
	if payload["type"] != "string" {
		t.Errorf("payload type = %v, want string", payload["type"])
	}
	required, _ := schema["required"].([]interface{})
	if len(required) != 1 || required[0] != "payload" {
		t.Errorf("payload must be required, got %v", required)
	}

	// fields：JSON 文本控件 + 双语 label/description
	if len(fp.Fields) != 1 {
		t.Fatalf("expected exactly one field, got %d", len(fp.Fields))
	}
	field := fp.Fields[0]
	if field.Key != "payload" {
		t.Errorf("field key = %q, want payload", field.Key)
	}
	if field.Widget != spec.FormWidgetJSON {
		t.Errorf("widget = %q, want JSON (textarea editor)", field.Widget)
	}
	if field.Required == nil || !*field.Required {
		t.Error("field must be marked required")
	}
	if field.Label["zh-CN"] == "" || field.Label["en-US"] == "" {
		t.Errorf("label must be localized zh-CN/en-US, got %v", field.Label)
	}
	if field.Description["zh-CN"] == "" || field.Description["en-US"] == "" {
		t.Errorf("description must be localized zh-CN/en-US, got %v", field.Description)
	}
}

// 有 schema 的函数不受兜底影响：字段由 buildFormFields 从真实 schema 生成。
func TestBuildFormPresentation_RealSchemaUnchanged(t *testing.T) {
	fn := spec.FunctionSpec{
		ID:          "players.player.create",
		InputSchema: spec.JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`),
	}
	fp := buildFormPresentation(spec.OperationSpec{FunctionID: fn.ID}, GenerateOptions{
		Functions: map[string]spec.FunctionSpec{fn.ID: fn},
	})

	if len(fp.Fields) != 1 || fp.Fields[0].Key != "name" {
		t.Fatalf("real schema should drive fields, got %+v", fp.Fields)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(fp.JSONSchema), &schema); err != nil {
		t.Fatalf("JSONSchema is not valid JSON: %v", err)
	}
	props, _ := schema["properties"].(map[string]interface{})
	if _, ok := props["payload"]; ok {
		t.Error("payload fallback must not leak into schema-driven forms")
	}
}
