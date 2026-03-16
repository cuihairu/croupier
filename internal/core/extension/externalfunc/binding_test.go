package externalfunc

import "testing"

func TestParseProviderBinding_WithConfigMap(t *testing.T) {
	spec := map[string]any{
		"provider":   "One Panel",
		"type":       "openapi",
		"operations": []any{"Install/App", "status"},
		"enabled":    true,
		"config": map[string]any{
			"base_url": "https://example.com",
		},
	}
	got, ok := ParseProviderBinding("", spec)
	if !ok {
		t.Fatalf("expected parse success")
	}
	if got.Provider != "one_panel" {
		t.Fatalf("unexpected provider: %s", got.Provider)
	}
	if got.Type != "openapi" {
		t.Fatalf("unexpected type: %s", got.Type)
	}
	if len(got.Operations) != 2 || got.Operations[0] != "install_app" || got.Operations[1] != "status" {
		t.Fatalf("unexpected operations: %#v", got.Operations)
	}
	if !got.Enabled {
		t.Fatalf("expected enabled=true")
	}
	if got.Config["base_url"] != "https://example.com" {
		t.Fatalf("unexpected config: %#v", got.Config)
	}
}

func TestParseProviderBinding_FallbackAndDefaults(t *testing.T) {
	spec := map[string]any{
		"name":    "Test Provider",
		"enabled": "0",
		"url":     "https://api.example.com",
	}
	got, ok := ParseProviderBinding("", spec)
	if !ok {
		t.Fatalf("expected parse success")
	}
	if got.Provider != "test_provider" {
		t.Fatalf("unexpected provider: %s", got.Provider)
	}
	if got.Type != "openapi" {
		t.Fatalf("unexpected type: %s", got.Type)
	}
	if len(got.Operations) != 1 || got.Operations[0] != "invoke" {
		t.Fatalf("unexpected operations: %#v", got.Operations)
	}
	if got.Enabled {
		t.Fatalf("expected enabled=false")
	}
	if got.Config["url"] != "https://api.example.com" {
		t.Fatalf("unexpected config fallback: %#v", got.Config)
	}
}

func TestParseProviderBinding_UsesBindingKeyAlias(t *testing.T) {
	spec := map[string]any{
		"operation": "invoke",
	}
	got, ok := ParseProviderBinding("my-provider", spec)
	if !ok {
		t.Fatalf("expected parse success")
	}
	if got.Provider != "my-provider" {
		t.Fatalf("unexpected provider: %s", got.Provider)
	}
}

func TestParseProviderBinding_EmptyProvider(t *testing.T) {
	got, ok := ParseProviderBinding(" ", map[string]any{
		"enabled": true,
	})
	if ok {
		t.Fatalf("expected parse failure, got: %#v", got)
	}
}

// Tests for stringSliceValue edge cases
func TestStringSliceValue_EdgeCases(t *testing.T) {
	// Test with invalid type (should return nil - default case)
	result := stringSliceValue(map[string]any{"operations": 12345}, "operations")
	if result != nil {
		t.Fatalf("expected nil for invalid type, got: %#v", result)
	}

	// Test with []any containing nil values
	result = stringSliceValue(map[string]any{"ops": []any{"a", nil, "b", "<nil>", "c"}}, "ops")
	if len(result) != 3 || result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Fatalf("unexpected result with nil values: %#v", result)
	}

	// Test with single string value (should wrap as slice)
	result = stringSliceValue(map[string]any{"op": "single_op"}, "op")
	if len(result) != 1 || result[0] != "single_op" {
		t.Fatalf("expected single element slice, got: %#v", result)
	}

	// Test with empty string value (should return nil)
	result = stringSliceValue(map[string]any{"op": "  "}, "op")
	if result != nil {
		t.Fatalf("expected nil for empty string, got: %#v", result)
	}

	// Test with []string
	result = stringSliceValue(map[string]any{"ops": []string{" x ", " y "}}, "ops")
	if len(result) != 2 || result[0] != "x" || result[1] != "y" {
		t.Fatalf("unexpected trimmed result: %#v", result)
	}

	// Test with nil map
	result = stringSliceValue(nil, "ops")
	if result != nil {
		t.Fatalf("expected nil for nil map, got: %#v", result)
	}

	// Test with missing key
	result = stringSliceValue(map[string]any{"other": "value"}, "ops")
	if result != nil {
		t.Fatalf("expected nil for missing key, got: %#v", result)
	}
}

// Tests for boolValue edge cases
func TestBoolValue_EdgeCases(t *testing.T) {
	// Test with bool true
	val, ok := boolValue(map[string]any{"enabled": true}, "enabled")
	if !ok || !val {
		t.Fatalf("expected true, got: val=%v, ok=%v", val, ok)
	}

	// Test with bool false
	val, ok = boolValue(map[string]any{"enabled": false}, "enabled")
	if !ok || val {
		t.Fatalf("expected false, got: val=%v, ok=%v", val, ok)
	}

	// Test with string "true"
	val, ok = boolValue(map[string]any{"enabled": "true"}, "enabled")
	if !ok || !val {
		t.Fatalf("expected true for 'true', got: val=%v, ok=%v", val, ok)
	}

	// Test with string "false"
	val, ok = boolValue(map[string]any{"enabled": "false"}, "enabled")
	if !ok || val {
		t.Fatalf("expected false for 'false', got: val=%v, ok=%v", val, ok)
	}

	// Test with string "1"
	val, ok = boolValue(map[string]any{"enabled": "1"}, "enabled")
	if !ok || !val {
		t.Fatalf("expected true for '1', got: val=%v, ok=%v", val, ok)
	}

	// Test with string "0"
	val, ok = boolValue(map[string]any{"enabled": "0"}, "enabled")
	if !ok || val {
		t.Fatalf("expected false for '0', got: val=%v, ok=%v", val, ok)
	}

	// Test with int non-zero
	val, ok = boolValue(map[string]any{"count": 42}, "count")
	if !ok || !val {
		t.Fatalf("expected true for int 42, got: val=%v, ok=%v", val, ok)
	}

	// Test with int zero
	val, ok = boolValue(map[string]any{"count": 0}, "count")
	if !ok || val {
		t.Fatalf("expected false for int 0, got: val=%v, ok=%v", val, ok)
	}

	// Test with int64
	val, ok = boolValue(map[string]any{"count": int64(100)}, "count")
	if !ok || !val {
		t.Fatalf("expected true for int64 100, got: val=%v, ok=%v", val, ok)
	}

	// Test with float64
	val, ok = boolValue(map[string]any{"count": 1.5}, "count")
	if !ok || !val {
		t.Fatalf("expected true for float64 1.5, got: val=%v, ok=%v", val, ok)
	}

	// Test with float64 zero
	val, ok = boolValue(map[string]any{"count": 0.0}, "count")
	if !ok || val {
		t.Fatalf("expected false for float64 0.0, got: val=%v, ok=%v", val, ok)
	}

	// Test with nil map
	val, ok = boolValue(nil, "enabled")
	if ok {
		t.Fatalf("expected ok=false for nil map, got: val=%v, ok=%v", val, ok)
	}

	// Test with missing key
	val, ok = boolValue(map[string]any{"other": "value"}, "enabled")
	if ok {
		t.Fatalf("expected ok=false for missing key, got: val=%v, ok=%v", val, ok)
	}

	// Test with nil value
	val, ok = boolValue(map[string]any{"enabled": nil}, "enabled")
	if ok {
		t.Fatalf("expected ok=false for nil value, got: val=%v, ok=%v", val, ok)
	}

	// Test with invalid type (should return false, false - default case)
	val, ok = boolValue(map[string]any{"enabled": []string{"a"}}, "enabled")
	if ok {
		t.Fatalf("expected ok=false for invalid type, got: val=%v, ok=%v", val, ok)
	}

	// Test with string "no"
	val, ok = boolValue(map[string]any{"enabled": "no"}, "enabled")
	if !ok || val {
		t.Fatalf("expected false for 'no', got: val=%v, ok=%v", val, ok)
	}

	// Test with string "off"
	val, ok = boolValue(map[string]any{"enabled": "off"}, "enabled")
	if !ok || val {
		t.Fatalf("expected false for 'off', got: val=%v, ok=%v", val, ok)
	}

	// Test with empty string
	val, ok = boolValue(map[string]any{"enabled": "  "}, "enabled")
	if ok {
		t.Fatalf("expected ok=false for empty string, got: val=%v, ok=%v", val, ok)
	}
}
