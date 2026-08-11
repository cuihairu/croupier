package validation

import (
	"encoding/json"
	"testing"
)

func TestValidateJSON(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"player_id": map[string]any{"type": "string"},
			"count":     map[string]any{"type": "integer"},
		},
		"required": []any{"player_id"},
	}
	if err := ValidateJSON(schema, []byte(`{"player_id":"1","count":2}`)); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateJSON(schema, []byte(`{"count":2}`)); err == nil {
		t.Fatalf("expected error for missing player_id")
	}
}

func TestValidateJSON_DataValidation(t *testing.T) {
	tests := []struct {
		name    string
		schema  map[string]any
		data    []byte
		wantErr bool
	}{
		{
			name: "empty payload is empty object",
			schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
			data:    []byte{},
			wantErr: false,
		},
		{
			name: "invalid JSON",
			schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
			data:    []byte(`{invalid}`),
			wantErr: true,
		},
		{
			name: "payload must be object",
			schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
			data:    []byte(`"string"`),
			wantErr: true,
		},
		{
			name: "unsupported root schema type",
			schema: map[string]any{
				"type": "array",
			},
			data:    []byte(`{}`),
			wantErr: true,
		},
		{
			name: "wrong field type",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"age": map[string]any{"type": "integer"},
				},
			},
			data:    []byte(`{"age":"not_a_number"}`),
			wantErr: true,
		},
		{
			name: "nested object",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"profile": map[string]any{"type": "object"},
				},
			},
			data:    []byte(`{"profile":{"name":"alice"}}`),
			wantErr: false,
		},
		{
			name: "unknown object field rejected by default",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
			data:    []byte(`{"name":"alice","role":"admin"}`),
			wantErr: true,
		},
		{
			name: "additional object field allowed explicitly",
			schema: map[string]any{
				"type":                 "object",
				"additionalProperties": true,
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
			data:    []byte(`{"name":"alice","role":"admin"}`),
			wantErr: false,
		},
		{
			name: "array items validated",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ids": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
				},
			},
			data:    []byte(`{"ids":["p1",2]}`),
			wantErr: true,
		},
		{
			name: "enum validated",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{
						"type": "string",
						"enum": []any{"active", "banned"},
					},
				},
			},
			data:    []byte(`{"status":"deleted"}`),
			wantErr: true,
		},
		{
			name: "local ref validated",
			schema: map[string]any{
				"type": "object",
				"$defs": map[string]any{
					"id": map[string]any{"type": "string"},
				},
				"properties": map[string]any{
					"playerId": map[string]any{"$ref": "#/$defs/id"},
				},
			},
			data:    []byte(`{"playerId":1}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJSON(tt.schema, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateJSONRaw(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"playerId": {"type": "string"}
		},
		"required": ["playerId"]
	}`)

	if err := ValidateJSONRaw(schema, []byte(`{"playerId":"p1"}`)); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateJSONRaw(schema, []byte(`{"playerId":1}`)); err == nil {
		t.Fatalf("expected type validation error")
	}
}

func TestCheckType(t *testing.T) {
	tests := []struct {
		name    string
		typ     string
		value   any
		wantErr bool
	}{
		{"string", "string", "hello", false},
		{"string mismatch", "string", 123, true},
		{"boolean true", "boolean", true, false},
		{"boolean mismatch", "boolean", "true", true},
		{"number float64", "number", float64(123.45), false},
		{"number json.Number", "number", json.Number("123.45"), false},
		{"number mismatch", "number", "123", true},
		{"integer float64", "integer", float64(42), false},
		{"integer fractional", "integer", float64(42.5), true},
		{"integer json.Number", "integer", json.Number("42"), false},
		{"object", "object", map[string]any{}, false},
		{"object mismatch", "object", "not_a_map", true},
		{"unknown types pass through", "unknown_type", 123, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkType(tt.typ, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSchemaDeclaresObjectProperties(t *testing.T) {
	tests := []struct {
		name   string
		schema map[string]any
		want   bool
	}{
		{"no properties", map[string]any{"type": "object"}, false},
		{"properties but no type", map[string]any{"properties": map[string]any{"name": map[string]any{"type": "string"}}}, true},
		{"properties with type object", map[string]any{"type": "object", "properties": map[string]any{}}, true},
		{"properties with type string", map[string]any{"type": "string", "properties": map[string]any{}}, false},
		{"properties with type array containing object", map[string]any{"type": []any{"array", "object"}, "properties": map[string]any{}}, true},
		{"properties with type array not containing object", map[string]any{"type": []any{"array", "string"}, "properties": map[string]any{}}, false},
		{"properties with non-string non-array type", map[string]any{"type": 123, "properties": map[string]any{}}, false},
		{"properties wrong type", map[string]any{"type": "object", "properties": "not_a_map"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := schemaDeclaresObjectProperties(tt.schema)
			if got != tt.want {
				t.Errorf("schemaDeclaresObjectProperties() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkValidateJSON(b *testing.B) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":  map[string]any{"type": "string"},
			"age":   map[string]any{"type": "integer"},
			"email": map[string]any{"type": "string"},
		},
		"required": []any{"name"},
	}
	data := []byte(`{"name":"John Doe","age":30,"email":"john@example.com"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateJSON(schema, data)
	}
}
