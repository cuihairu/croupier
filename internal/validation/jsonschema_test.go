package validation

import "testing"

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
