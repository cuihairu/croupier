package validation

import (
	"encoding/json"
	"testing"
)

func TestValidateJSONNilSchema(t *testing.T) {
	if err := ValidateJSON(nil, []byte(`{"a":1}`)); err != nil {
		t.Fatalf("nil schema should accept any payload: %v", err)
	}
}

func TestValidateJSONRawEmptySchema(t *testing.T) {
	if err := ValidateJSONRaw(json.RawMessage("   "), []byte(`{}`)); err != nil {
		t.Fatalf("empty raw schema should accept payload: %v", err)
	}
}

func TestValidateJSONRawInvalidSchemaJSON(t *testing.T) {
	err := ValidateJSONRaw(json.RawMessage(`{broken`), []byte(`{}`))
	if err == nil {
		t.Fatal("invalid schema JSON should error")
	}
}

func TestValidateJSONSchemaAddResourceError(t *testing.T) {
	// 不可序列化的 schema 值 → AddResource 失败
	bad := map[string]any{"type": make(chan int)}
	if err := ValidateJSONSchema(bad, []byte(`{}`)); err == nil {
		t.Fatal("unserializable schema should error")
	}
}

func TestValidateJSONSchemaCompileError(t *testing.T) {
	// draft7 非法 schema：required 必须是数组
	schema := map[string]any{"type": "object", "required": "not-an-array"}
	err := ValidateJSONSchema(schema, []byte(`{}`))
	if err == nil {
		t.Fatal("invalid schema should fail compilation")
	}
}

func TestDecodeJSONPayloadMultipleValues(t *testing.T) {
	err := ValidateJSONSchema(map[string]any{}, []byte(`{"a":1} {"b":2}`))
	if err == nil {
		t.Fatal("multiple JSON values should error")
	}
}
