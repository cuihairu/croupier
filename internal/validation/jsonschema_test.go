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
			name: "root array validated",
			schema: map[string]any{
				"type": "array",
			},
			data:    []byte(`[1,2]`),
			wantErr: false,
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
			// SUI-016: unknown fields follow standard JSON Schema semantics —
			// accepted unless the schema says otherwise.
			name: "unknown object field allowed by default",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
			data:    []byte(`{"name":"alice","role":"admin"}`),
			wantErr: false,
		},
		{
			name: "unknown object field rejected when explicit",
			schema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
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
			name: "unknown fields pass through when properties are absent",
			schema: map[string]any{
				"type": "object",
			},
			data:    []byte(`{"anything":1}`),
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

// SUI-016: the structured verdict distinguishes required violations from
// other ones, so both server entries can name the offending fields the same
// way before choosing their own error shape.
func TestSchemaVerdictFields(t *testing.T) {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"playerId": map[string]any{"type": "string"},
			"amount":   map[string]any{"type": "integer"},
		},
		"required": []any{"playerId", "amount"},
	}

	verdict, err := ValidateJSONSchemaVerdict(schema, []byte(`{"playerId":"p-1","extra":1}`))
	if err != nil {
		t.Fatalf("unexpected defect: %v", err)
	}
	if !verdict.HasViolations() {
		t.Fatal("expected violations")
	}
	if len(verdict.MissingRequired) != 1 || verdict.MissingRequired[0] != "amount" {
		t.Fatalf("expected amount missing, got %v", verdict.MissingRequired)
	}
	if len(verdict.OtherMessages) != 1 || verdict.OtherMessages[0] != "additional properties 'extra' not allowed" {
		t.Fatalf("expected extra rejected by additionalProperties:false, got %v", verdict.OtherMessages)
	}
	if verdict.Details["amount"] != "required" {
		t.Fatalf("expected amount detail required, got %v", verdict.Details)
	}

	clean, err := ValidateJSONSchemaVerdict(schema, []byte(`{"playerId":"p-1","amount":1}`))
	if err != nil {
		t.Fatalf("unexpected defect: %v", err)
	}
	if clean.HasViolations() {
		t.Fatalf("expected empty verdict, got %v", clean)
	}
	if msgs := clean.AllMessages(); msgs != nil {
		t.Fatalf("expected nil messages, got %v", msgs)
	}
}

// SUI-016: a schema that does not compile and a payload that is not valid
// JSON are defects, not violations — the error is non-nil and the verdict is
// absent, so callers reject fail-closed instead of treating a declared
// contract as absent.
func TestSchemaVerdictDefects(t *testing.T) {
	// schema compile failure
	verdict, err := ValidateJSONSchemaVerdict(map[string]any{"type": "object", "required": "not-an-array"}, []byte(`{}`))
	if err == nil {
		t.Fatal("expected compile failure")
	}
	if verdict != nil {
		t.Fatalf("expected nil verdict on defect, got %v", verdict)
	}

	// invalid payload JSON
	verdict, err = ValidateJSONSchemaVerdict(map[string]any{"type": "object"}, []byte(`{invalid`))
	if err == nil {
		t.Fatal("expected invalid JSON error")
	}
	if verdict != nil {
		t.Fatalf("expected nil verdict on defect, got %v", verdict)
	}

	// multiple JSON values
	verdict, err = ValidateJSONSchemaVerdict(map[string]any{"type": "object"}, []byte(`{} {}`))
	if err == nil {
		t.Fatal("expected multiple values error")
	}
	if verdict != nil {
		t.Fatalf("expected nil verdict on defect, got %v", verdict)
	}
}

func TestSchemaVerdictAllMessages(t *testing.T) {
	verdict, err := ValidateJSONSchemaVerdict(map[string]any{
		"type":       "object",
		"properties": map[string]any{"b": map[string]any{"type": "integer"}, "a": map[string]any{"type": "integer"}},
		"required":   []any{"b", "a"},
	}, []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected defect: %v", err)
	}
	msgs := verdict.AllMessages()
	if len(msgs) != 2 || msgs[0] != "a: required" || msgs[1] != "b: required" {
		t.Fatalf("expected sorted required messages, got %v", msgs)
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
