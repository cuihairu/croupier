package openapi

import "testing"

func TestSchemaMapper_EdgeBranches(t *testing.T) {
	mapper := NewSchemaMapper()

	// OpenAPIToJSONSchema(nil) → "{}"
	out, err := mapper.OpenAPIToJSONSchema(nil)
	if err != nil || out != "{}" {
		t.Fatalf("nil schema = %q, %v", out, err)
	}

	// JSONSchemaToOpenAPI("") → nil
	if ref := mapper.JSONSchemaToOpenAPI(""); ref != nil {
		t.Fatalf("empty schema should be nil, got %+v", ref)
	}

	// 非法 JSON → 兜底 object schema
	ref := mapper.JSONSchemaToOpenAPI("{bad json")
	if ref == nil || ref.Value == nil || ref.Value.Type == nil || (*ref.Value.Type)[0] != "object" {
		t.Fatalf("malformed schema should fall back to object, got %+v", ref)
	}

	// 数组根 schema（buildSchemaFromData []interface{} 分支）
	ref = mapper.JSONSchemaToOpenAPI(`[{"type":"string"}]`)
	if ref == nil || ref.Value.Type == nil || (*ref.Value.Type)[0] != "array" {
		t.Fatalf("array root schema, got %+v", ref)
	}
	if ref.Value.Items == nil || ref.Value.Items.Value == nil {
		t.Fatal("array schema must carry items")
	}

	// 空数组根 → 无类型
	ref = mapper.JSONSchemaToOpenAPI(`[]`)
	if ref == nil || ref.Value.Type != nil {
		t.Fatalf("empty array root should have no type, got %+v", ref)
	}

	// 原始标量根（buildSchemaFromData default 分支 → inferType）
	ref = mapper.JSONSchemaToOpenAPI(`42`)
	if ref == nil || ref.Value.Type == nil {
		t.Fatalf("primitive root schema, got %+v", ref)
	}
}
