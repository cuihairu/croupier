package extension

import (
	"testing"
)

// Split from service_test.go; test bodies moved verbatim.

func TestValidateConfigAgainstSchemaRequired(t *testing.T) {
	schema := map[string]any{
		"required": []any{"enabled"},
		"properties": map[string]any{
			"enabled": map[string]any{"type": "boolean"},
		},
	}

	err := validateConfigAgainstSchema(map[string]any{}, schema)
	if err == nil {
		t.Fatalf("expected missing required error")
	}
}

func TestValidateConfigAgainstSchemaTypeAndEnum(t *testing.T) {
	schema := map[string]any{
		"required": []any{"mode", "retry"},
		"properties": map[string]any{
			"mode":  map[string]any{"type": "string", "enum": []any{"safe", "fast"}},
			"retry": map[string]any{"type": "integer"},
		},
	}

	valid := map[string]any{
		"mode":  "safe",
		"retry": float64(3),
	}
	if err := validateConfigAgainstSchema(valid, schema); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}

	invalidEnum := map[string]any{
		"mode":  "invalid",
		"retry": float64(3),
	}
	if err := validateConfigAgainstSchema(invalidEnum, schema); err == nil {
		t.Fatalf("expected enum validation error")
	}

	invalidType := map[string]any{
		"mode":  "safe",
		"retry": "3",
	}
	if err := validateConfigAgainstSchema(invalidType, schema); err == nil {
		t.Fatalf("expected integer type validation error")
	}
}

func TestValidateConfigField(t *testing.T) {
	// Test enum validation
	schema := map[string]any{"enum": []any{"a", "b", "c"}}
	err := validateConfigField("test", "a", schema)
	if err != nil {
		t.Fatalf("unexpected error for valid enum: %v", err)
	}

	err = validateConfigField("test", "d", schema)
	if err == nil {
		t.Fatalf("expected enum validation error")
	}

	// Test type validation
	stringSchema := map[string]any{"type": "string"}
	err = validateConfigField("test", "hello", stringSchema)
	if err != nil {
		t.Fatalf("unexpected error for string type: %v", err)
	}

	err = validateConfigField("test", 123, stringSchema)
	if err == nil {
		t.Fatalf("expected type error for non-string")
	}

	// Test number type
	numberSchema := map[string]any{"type": "number"}
	err = validateConfigField("test", float64(1.5), numberSchema)
	if err != nil {
		t.Fatalf("unexpected error for number type: %v", err)
	}

	// Test integer type
	intSchema := map[string]any{"type": "integer"}
	err = validateConfigField("test", 123, intSchema)
	if err != nil {
		t.Fatalf("unexpected error for integer type: %v", err)
	}

	err = validateConfigField("test", float64(1.5), intSchema)
	if err == nil {
		t.Fatalf("expected integer type error for float")
	}
}

func TestValidateConfigAgainstSchemaNilConfig(t *testing.T) {
	schema := map[string]any{
		"required": []any{"field1"},
		"properties": map[string]any{
			"field1": map[string]any{"type": "string"},
		},
	}
	// nil config should be treated as empty map
	err := validateConfigAgainstSchema(nil, schema)
	if err == nil {
		t.Fatalf("expected error for missing required field")
	}
}

func TestValidateConfigFieldEmptySchema(t *testing.T) {
	// Empty schema should not error
	err := validateConfigField("test", "anyvalue", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error for empty schema: %v", err)
	}
}

func TestValidateConfigFieldNoType(t *testing.T) {
	schema := map[string]any{} // no type specified
	err := validateConfigField("test", "anyvalue", schema)
	if err != nil {
		t.Fatalf("unexpected error for schema without type: %v", err)
	}
}

func TestValidateConfigFieldBoolean(t *testing.T) {
	schema := map[string]any{"type": "boolean"}
	err := validateConfigField("test", true, schema)
	if err != nil {
		t.Fatalf("unexpected error for boolean: %v", err)
	}

	err = validateConfigField("test", "true", schema)
	if err == nil {
		t.Fatalf("expected error for string as boolean")
	}

	err = validateConfigField("test", 1, schema)
	if err == nil {
		t.Fatalf("expected error for int as boolean")
	}
}

func TestValidateConfigFieldObject(t *testing.T) {
	schema := map[string]any{"type": "object"}
	valid := map[string]any{"key": "value"}
	err := validateConfigField("test", valid, schema)
	if err != nil {
		t.Fatalf("unexpected error for object: %v", err)
	}

	err = validateConfigField("test", "not-an-object", schema)
	if err == nil {
		t.Fatalf("expected error for string as object")
	}
}

func TestValidateConfigFieldArray(t *testing.T) {
	schema := map[string]any{"type": "array"}
	valid := []any{"item1", "item2"}
	err := validateConfigField("test", valid, schema)
	if err != nil {
		t.Fatalf("unexpected error for array: %v", err)
	}

	err = validateConfigField("test", "not-an-array", schema)
	if err == nil {
		t.Fatalf("expected error for string as array")
	}
}

func TestValidateConfigFieldEnum(t *testing.T) {
	schema := map[string]any{"enum": []any{"option1", "option2"}}
	err := validateConfigField("test", "option1", schema)
	if err != nil {
		t.Fatalf("unexpected error for valid enum value: %v", err)
	}

	err = validateConfigField("test", "option3", schema)
	if err == nil {
		t.Fatalf("expected error for invalid enum value")
	}

	// Empty enum should not error
	emptySchema := map[string]any{"enum": []any{}}
	err = validateConfigField("test", "any", emptySchema)
	if err != nil {
		t.Fatalf("unexpected error for empty enum: %v", err)
	}
}

func TestValidateConfigField_Array(t *testing.T) {
	schema := map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "string",
		},
	}
	valid := []any{"item1", "item2"}
	err := validateConfigField("test", valid, schema)
	if err != nil {
		t.Fatalf("unexpected error for array: %v", err)
	}

	// Test non-array value
	err = validateConfigField("test", "not-an-array", schema)
	if err == nil {
		t.Fatalf("expected error for non-array value")
	}
}

func TestValidateConfigField_Object_Nested(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"nested": map[string]any{"type": "string"},
		},
	}
	valid := map[string]any{"nested": "value"}
	err := validateConfigField("test", valid, schema)
	if err != nil {
		t.Fatalf("unexpected error for nested object: %v", err)
	}
}

func TestValidateConfigField_Number(t *testing.T) {
	schema := map[string]any{"type": "number"}
	err := validateConfigField("test", float64(3.14), schema)
	if err != nil {
		t.Fatalf("unexpected error for number: %v", err)
	}

	err = validateConfigField("test", 3, schema)
	if err != nil {
		t.Fatalf("unexpected error for int as number: %v", err)
	}

	err = validateConfigField("test", "3.14", schema)
	if err == nil {
		t.Fatalf("expected error for string as number")
	}
}

func TestValidateConfigField_NullValue(t *testing.T) {
	schema := map[string]any{"type": "string"}
	err := validateConfigField("test", nil, schema)
	// nil is not valid for string type
	if err == nil {
		t.Fatal("expected error for nil value with string type")
	}
}

func TestValidateConfigAgainstSchema_NoProperties(t *testing.T) {
	schema := map[string]any{
		"required": []any{"field1"},
	}
	err := validateConfigAgainstSchema(map[string]any{"field1": "value"}, schema)
	// Schema without properties should validate required fields but not types
	if err != nil {
		t.Fatalf("unexpected error for schema without properties: %v", err)
	}
}
