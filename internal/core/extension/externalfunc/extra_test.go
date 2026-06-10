package externalfunc

import (
	"testing"
)

func TestBuildFunctionID_EmptyProvider(t *testing.T) {
	got := BuildFunctionID("", "method")
	if got != "" {
		t.Errorf("Expected empty string for empty provider, got: %s", got)
	}
}

func TestBuildFunctionID_EmptyMethod(t *testing.T) {
	got := BuildFunctionID("provider", "")
	if got != "" {
		t.Errorf("Expected empty string for empty method, got: %s", got)
	}
}

func TestBuildFunctionID_BothEmpty(t *testing.T) {
	got := BuildFunctionID("", "")
	if got != "" {
		t.Errorf("Expected empty string for both empty, got: %s", got)
	}
}

func TestBuildFunctionID_WhitespaceOnly(t *testing.T) {
	got := BuildFunctionID("  ", "  ")
	if got != "" {
		t.Errorf("Expected empty string for whitespace only, got: %s", got)
	}
}

func TestParseFunctionID_NoPrefix(t *testing.T) {
	p, m, ok := ParseFunctionID("not_external.method")
	if ok {
		t.Error("Expected false for non-external prefix")
	}
	if p != "" || m != "" {
		t.Errorf("Expected empty provider and method, got: %s, %s", p, m)
	}
}

func TestParseFunctionID_EmptyString(t *testing.T) {
	p, m, ok := ParseFunctionID("")
	if ok {
		t.Error("Expected false for empty string")
	}
	if p != "" || m != "" {
		t.Errorf("Expected empty provider and method, got: %s, %s", p, m)
	}
}

func TestParseFunctionID_OnlyPrefix(t *testing.T) {
	p, m, ok := ParseFunctionID("external.")
	if ok {
		t.Error("Expected false for only prefix")
	}
	if p != "" || m != "" {
		t.Errorf("Expected empty provider and method, got: %s, %s", p, m)
	}
}

func TestParseFunctionID_PrefixAndProvider(t *testing.T) {
	p, m, ok := ParseFunctionID("external.provider")
	if ok {
		t.Error("Expected false for prefix and provider only")
	}
	if p != "" || m != "" {
		t.Errorf("Expected empty provider and method, got: %s, %s", p, m)
	}
}

func TestParseFunctionID_MultipleDots(t *testing.T) {
	p, m, ok := ParseFunctionID("external.provider.method.extra")
	if !ok {
		t.Error("Expected true for multiple dots")
	}
	if p != "provider" || m != "method.extra" {
		t.Errorf("Expected provider=provider, method=method.extra, got: %s, %s", p, m)
	}
}

func TestParseFunctionID_WhitespacePadding(t *testing.T) {
	p, m, ok := ParseFunctionID("  external.provider.method  ")
	if !ok {
		t.Error("Expected true for whitespace padding")
	}
	if p != "provider" || m != "method" {
		t.Errorf("Expected provider=provider, method=method, got: %s, %s", p, m)
	}
}

func TestSanitizeKey_EmptyString(t *testing.T) {
	got := SanitizeKey("")
	if got != "" {
		t.Errorf("Expected empty string, got: %s", got)
	}
}

func TestSanitizeKey_WhitespaceOnly(t *testing.T) {
	got := SanitizeKey("   ")
	if got != "" {
		t.Errorf("Expected empty string for whitespace only, got: %s", got)
	}
}

func TestSanitizeKey_SpecialCharacters(t *testing.T) {
	got := SanitizeKey("hello@world#test")
	if got != "helloworldtest" {
		t.Errorf("Expected 'helloworldtest', got: %s", got)
	}
}

func TestSanitizeKey_UnderscoresAndHyphens(t *testing.T) {
	got := SanitizeKey("hello_world-test")
	if got != "hello_world-test" {
		t.Errorf("Expected 'hello_world-test', got: %s", got)
	}
}

func TestSanitizeKey_Dots(t *testing.T) {
	got := SanitizeKey("hello.world.test")
	if got != "hello.world.test" {
		t.Errorf("Expected 'hello.world.test', got: %s", got)
	}
}

func TestSanitizeKey_TrimLeadingTrailing(t *testing.T) {
	got := SanitizeKey("._-hello-_.")
	if got != "hello" {
		t.Errorf("Expected 'hello', got: %s", got)
	}
}

func TestCapability_EmptyProvider(t *testing.T) {
	got := Capability("")
	if got != "" {
		t.Errorf("Expected empty string for empty provider, got: %s", got)
	}
}

func TestCapability_WhitespaceOnly(t *testing.T) {
	got := Capability("   ")
	if got != "" {
		t.Errorf("Expected empty string for whitespace only, got: %s", got)
	}
}

func TestOperation_EmptyMethod(t *testing.T) {
	got := Operation("")
	if got != "" {
		t.Errorf("Expected empty string for empty method, got: %s", got)
	}
}

func TestCapabilityOperationFromFunctionID_EmptyString(t *testing.T) {
	capability, operation, ok := CapabilityOperationFromFunctionID("")
	if ok {
		t.Error("Expected false for empty string")
	}
	if capability != "" || operation != "" {
		t.Errorf("Expected empty capability and operation, got: %s, %s", capability, operation)
	}
}

func TestCapabilityOperationFromFunctionID_InvalidFormat(t *testing.T) {
	capability, operation, ok := CapabilityOperationFromFunctionID("not_external.method")
	if ok {
		t.Error("Expected false for invalid format")
	}
	if capability != "" || operation != "" {
		t.Errorf("Expected empty capability and operation, got: %s, %s", capability, operation)
	}
}

func TestCapabilityOperationFromFunctionID_EmptyProvider(t *testing.T) {
	capability, operation, ok := CapabilityOperationFromFunctionID("external..method")
	if ok {
		t.Error("Expected false for empty provider")
	}
	if capability != "" || operation != "" {
		t.Errorf("Expected empty capability and operation, got: %s, %s", capability, operation)
	}
}

func TestCapabilityOperationFromFunctionID_EmptyMethod(t *testing.T) {
	capability, operation, ok := CapabilityOperationFromFunctionID("external.provider.")
	if ok {
		t.Error("Expected false for empty method")
	}
	if capability != "" || operation != "" {
		t.Errorf("Expected empty capability and operation, got: %s, %s", capability, operation)
	}
}

func TestDiscoverProviderOperations_EmptyBindings(t *testing.T) {
	got := DiscoverProviderOperations([]Binding{})
	if len(got) != 0 {
		t.Errorf("Expected empty map, got: %v", got)
	}
}

func TestDiscoverProviderOperations_InvalidBindingType(t *testing.T) {
	got := DiscoverProviderOperations([]Binding{
		{
			BindingType: "invalid",
			BindingKey:  "test",
		},
	})
	if len(got) != 0 {
		t.Errorf("Expected empty map for invalid binding type, got: %v", got)
	}
}

func TestDiscoverProviderOperations_InvalidProviderBinding(t *testing.T) {
	got := DiscoverProviderOperations([]Binding{
		{
			BindingType: "provider",
			BindingKey:  "",
			Spec:        map[string]any{},
		},
	})
	if len(got) != 0 {
		t.Errorf("Expected empty map for invalid provider binding, got: %v", got)
	}
}

func TestDiscoverProviderOperations_InvalidFunctionBinding(t *testing.T) {
	got := DiscoverProviderOperations([]Binding{
		{
			BindingType: "function",
			BindingKey:  "not_external.method",
		},
	})
	if len(got) != 0 {
		t.Errorf("Expected empty map for invalid function binding, got: %v", got)
	}
}

func TestDiscoverProviderOperations_DeduplicateOperations(t *testing.T) {
	got := DiscoverProviderOperations([]Binding{
		{
			BindingType: "provider",
			BindingKey:  "test",
			Spec: map[string]any{
				"provider":   "test",
				"operations": []any{"op1", "op2", "op1"}, // Duplicate op1
			},
		},
	})
	if len(got) != 1 {
		t.Errorf("Expected 1 provider, got: %d", len(got))
	}
	if len(got["test"]) != 2 {
		t.Errorf("Expected 2 operations (deduplicated), got: %d", len(got["test"]))
	}
}

func TestDiscoverProviderOperations_EmptyOperations(t *testing.T) {
	got := DiscoverProviderOperations([]Binding{
		{
			BindingType: "provider",
			BindingKey:  "test",
			Spec: map[string]any{
				"provider":   "test",
				"operations": []any{},
			},
		},
	})
	if len(got) != 1 {
		t.Errorf("Expected 1 provider, got: %d", len(got))
	}
	// Should have default "invoke" operation
	if len(got["test"]) != 1 || got["test"][0] != "invoke" {
		t.Errorf("Expected ['invoke'], got: %v", got["test"])
	}
}

func TestDiscoverProviderOperations_SingleOperation(t *testing.T) {
	got := DiscoverProviderOperations([]Binding{
		{
			BindingType: "provider",
			BindingKey:  "test",
			Spec: map[string]any{
				"provider":  "test",
				"operation": "single_op",
			},
		},
	})
	if len(got) != 1 {
		t.Errorf("Expected 1 provider, got: %d", len(got))
	}
	if len(got["test"]) != 1 || got["test"][0] != "single_op" {
		t.Errorf("Expected ['single_op'], got: %v", got["test"])
	}
}

func TestDiscoverProviderOperations_OpenAPIBindingType(t *testing.T) {
	got := DiscoverProviderOperations([]Binding{
		{
			BindingType: "openapi",
			BindingKey:  "test",
			Spec: map[string]any{
				"provider": "test",
			},
		},
	})
	if len(got) != 1 {
		t.Errorf("Expected 1 provider, got: %d", len(got))
	}
}

func TestParseProviderBinding_WithEnabled(t *testing.T) {
	binding, ok := ParseProviderBinding("test", map[string]any{
		"enabled": false,
	})
	if !ok {
		t.Error("Expected true for valid binding")
	}
	if binding.Enabled {
		t.Error("Expected enabled to be false")
	}
}

func TestParseProviderBinding_WithStringEnabled(t *testing.T) {
	binding, ok := ParseProviderBinding("test", map[string]any{
		"enabled": "true",
	})
	if !ok {
		t.Error("Expected true for valid binding")
	}
	if !binding.Enabled {
		t.Error("Expected enabled to be true")
	}
}

func TestParseProviderBinding_WithConfig(t *testing.T) {
	binding, ok := ParseProviderBinding("test", map[string]any{
		"config": map[string]any{
			"key1": "value1",
			"key2": "value2",
		},
	})
	if !ok {
		t.Error("Expected true for valid binding")
	}
	if binding.Config["key1"] != "value1" || binding.Config["key2"] != "value2" {
		t.Errorf("Expected config with key1=value1, key2=value2, got: %v", binding.Config)
	}
}

func TestParseProviderBinding_WithExtraFields(t *testing.T) {
	binding, ok := ParseProviderBinding("test", map[string]any{
		"extra_field": "value",
		"another":     123,
	})
	if !ok {
		t.Error("Expected true for valid binding")
	}
	if binding.Config["extra_field"] != "value" || binding.Config["another"] != 123 {
		t.Errorf("Expected extra fields in config, got: %v", binding.Config)
	}
}

func TestParseProviderBinding_NilSpec(t *testing.T) {
	binding, ok := ParseProviderBinding("test", nil)
	if !ok {
		t.Error("Expected true for valid binding with nil spec")
	}
	if binding.Provider != "test" {
		t.Errorf("Expected provider 'test', got: %s", binding.Provider)
	}
}

func TestStringValue_NilMap(t *testing.T) {
	got := stringValue(nil, "key")
	if got != "" {
		t.Errorf("Expected empty string for nil map, got: %s", got)
	}
}

func TestStringValue_MissingKey(t *testing.T) {
	got := stringValue(map[string]any{}, "key")
	if got != "" {
		t.Errorf("Expected empty string for missing key, got: %s", got)
	}
}

func TestStringValue_NilValue(t *testing.T) {
	got := stringValue(map[string]any{"key": nil}, "key")
	if got != "" {
		t.Errorf("Expected empty string for nil value, got: %s", got)
	}
}

func TestStringSliceValue_NilMap(t *testing.T) {
	got := stringSliceValue(nil, "key")
	if got != nil {
		t.Errorf("Expected nil for nil map, got: %v", got)
	}
}

func TestStringSliceValue_MissingKey(t *testing.T) {
	got := stringSliceValue(map[string]any{}, "key")
	if got != nil {
		t.Errorf("Expected nil for missing key, got: %v", got)
	}
}

func TestStringSliceValue_NilValue(t *testing.T) {
	got := stringSliceValue(map[string]any{"key": nil}, "key")
	if got != nil {
		t.Errorf("Expected nil for nil value, got: %v", got)
	}
}

func TestStringSliceValue_StringSlice(t *testing.T) {
	got := stringSliceValue(map[string]any{"key": []string{"a", "b", "c"}}, "key")
	if len(got) != 3 {
		t.Errorf("Expected 3 items, got: %d", len(got))
	}
}

func TestStringSliceValue_StringSliceWithSpaces(t *testing.T) {
	got := stringSliceValue(map[string]any{"key": []string{"  a  ", "  b  "}}, "key")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("Expected ['a', 'b'], got: %v", got)
	}
}

func TestStringSliceValue_StringValue(t *testing.T) {
	got := stringSliceValue(map[string]any{"key": "single"}, "key")
	if len(got) != 1 || got[0] != "single" {
		t.Errorf("Expected ['single'], got: %v", got)
	}
}

func TestStringSliceValue_EmptyStringValue(t *testing.T) {
	got := stringSliceValue(map[string]any{"key": "  "}, "key")
	if got != nil {
		t.Errorf("Expected nil for empty string value, got: %v", got)
	}
}

func TestStringSliceValue_NilInSlice(t *testing.T) {
	got := stringSliceValue(map[string]any{"key": []any{"a", nil, "b"}}, "key")
	if len(got) != 2 {
		t.Errorf("Expected 2 items (nil filtered), got: %d", len(got))
	}
}

func TestBoolValue_NilMap(t *testing.T) {
	_, ok := boolValue(nil, "key")
	if ok {
		t.Error("Expected false for nil map")
	}
}

func TestBoolValue_MissingKey(t *testing.T) {
	_, ok := boolValue(map[string]any{}, "key")
	if ok {
		t.Error("Expected false for missing key")
	}
}

func TestBoolValue_NilValue(t *testing.T) {
	_, ok := boolValue(map[string]any{"key": nil}, "key")
	if ok {
		t.Error("Expected false for nil value")
	}
}

func TestBoolValue_BoolTrue(t *testing.T) {
	val, ok := boolValue(map[string]any{"key": true}, "key")
	if !ok || !val {
		t.Errorf("Expected true, true, got: %v, %v", val, ok)
	}
}

func TestBoolValue_BoolFalse(t *testing.T) {
	val, ok := boolValue(map[string]any{"key": false}, "key")
	if !ok || val {
		t.Errorf("Expected false, true, got: %v, %v", val, ok)
	}
}

func TestBoolValue_StringTrue(t *testing.T) {
	val, ok := boolValue(map[string]any{"key": "true"}, "key")
	if !ok || !val {
		t.Errorf("Expected true, true, got: %v, %v", val, ok)
	}
}

func TestBoolValue_StringFalse(t *testing.T) {
	val, ok := boolValue(map[string]any{"key": "false"}, "key")
	if !ok || val {
		t.Errorf("Expected false, true, got: %v, %v", val, ok)
	}
}

func TestBoolValue_String0(t *testing.T) {
	val, ok := boolValue(map[string]any{"key": "0"}, "key")
	if !ok || val {
		t.Errorf("Expected false, true, got: %v, %v", val, ok)
	}
}

func TestBoolValue_StringNo(t *testing.T) {
	val, ok := boolValue(map[string]any{"key": "no"}, "key")
	if !ok || val {
		t.Errorf("Expected false, true, got: %v, %v", val, ok)
	}
}

func TestBoolValue_StringOff(t *testing.T) {
	val, ok := boolValue(map[string]any{"key": "off"}, "key")
	if !ok || val {
		t.Errorf("Expected false, true, got: %v, %v", val, ok)
	}
}

func TestBoolValue_StringEmpty(t *testing.T) {
	_, ok := boolValue(map[string]any{"key": ""}, "key")
	if ok {
		t.Error("Expected false for empty string")
	}
}

func TestBoolValue_StringWhitespace(t *testing.T) {
	_, ok := boolValue(map[string]any{"key": "  "}, "key")
	if ok {
		t.Error("Expected false for whitespace string")
	}
}

func TestBoolValue_IntNonZero(t *testing.T) {
	val, ok := boolValue(map[string]any{"key": 42}, "key")
	if !ok || !val {
		t.Errorf("Expected true, true, got: %v, %v", val, ok)
	}
}

func TestBoolValue_IntZero(t *testing.T) {
	val, ok := boolValue(map[string]any{"key": 0}, "key")
	if !ok || val {
		t.Errorf("Expected false, true, got: %v, %v", val, ok)
	}
}

func TestBoolValue_Int64NonZero(t *testing.T) {
	val, ok := boolValue(map[string]any{"key": int64(42)}, "key")
	if !ok || !val {
		t.Errorf("Expected true, true, got: %v, %v", val, ok)
	}
}

func TestBoolValue_Int64Zero(t *testing.T) {
	val, ok := boolValue(map[string]any{"key": int64(0)}, "key")
	if !ok || val {
		t.Errorf("Expected false, true, got: %v, %v", val, ok)
	}
}

func TestBoolValue_Float64NonZero(t *testing.T) {
	val, ok := boolValue(map[string]any{"key": 3.14}, "key")
	if !ok || !val {
		t.Errorf("Expected true, true, got: %v, %v", val, ok)
	}
}

func TestBoolValue_Float64Zero(t *testing.T) {
	val, ok := boolValue(map[string]any{"key": 0.0}, "key")
	if !ok || val {
		t.Errorf("Expected false, true, got: %v, %v", val, ok)
	}
}

func TestBoolValue_UnknownType(t *testing.T) {
	_, ok := boolValue(map[string]any{"key": []string{"a"}}, "key")
	if ok {
		t.Error("Expected false for unknown type")
	}
}

func TestDedupNonEmptySanitized_EmptySlice(t *testing.T) {
	got := dedupNonEmptySanitized([]string{})
	if len(got) != 0 {
		t.Errorf("Expected empty slice, got: %v", got)
	}
}

func TestDedupNonEmptySanitized_AllEmpty(t *testing.T) {
	got := dedupNonEmptySanitized([]string{"", "  ", ""})
	if len(got) != 0 {
		t.Errorf("Expected empty slice, got: %v", got)
	}
}

func TestDedupNonEmptySanitized_WithDuplicates(t *testing.T) {
	got := dedupNonEmptySanitized([]string{"a", "b", "a", "c", "b"})
	if len(got) != 3 {
		t.Errorf("Expected 3 unique items, got: %d", len(got))
	}
}

func TestExtractConfig_NilSpec(t *testing.T) {
	got := extractConfig(nil)
	if len(got) != 0 {
		t.Errorf("Expected empty map for nil spec, got: %v", got)
	}
}

func TestExtractConfig_WithConfigKey(t *testing.T) {
	spec := map[string]any{
		"config": map[string]any{
			"key1": "value1",
		},
		"other": "value",
	}
	got := extractConfig(spec)
	if len(got) != 1 || got["key1"] != "value1" {
		t.Errorf("Expected config from 'config' key, got: %v", got)
	}
}

func TestExtractConfig_WithoutConfigKey(t *testing.T) {
	spec := map[string]any{
		"provider":    "test",
		"type":        "openapi",
		"enabled":     true,
		"operations":  []string{"op1"},
		"extra_field": "value",
	}
	got := extractConfig(spec)
	// Should include extra_field but not the known keys
	if got["extra_field"] != "value" {
		t.Errorf("Expected extra_field in config, got: %v", got)
	}
	if _, exists := got["provider"]; exists {
		t.Error("Expected provider to be excluded from config")
	}
}

func TestFirstNonEmpty_AllEmpty(t *testing.T) {
	got := firstNonEmpty("", "", "")
	if got != "" {
		t.Errorf("Expected empty string, got: %s", got)
	}
}

func TestFirstNonEmpty_FirstNonEmpty(t *testing.T) {
	got := firstNonEmpty("first", "second", "third")
	if got != "first" {
		t.Errorf("Expected 'first', got: %s", got)
	}
}

func TestFirstNonEmpty_MiddleNonEmpty(t *testing.T) {
	got := firstNonEmpty("", "second", "third")
	if got != "second" {
		t.Errorf("Expected 'second', got: %s", got)
	}
}

func TestFirstNonEmpty_LastNonEmpty(t *testing.T) {
	got := firstNonEmpty("", "", "third")
	if got != "third" {
		t.Errorf("Expected 'third', got: %s", got)
	}
}

func TestFirstNonEmpty_WhitespaceOnly(t *testing.T) {
	got := firstNonEmpty("  ", "  ", "  ")
	if got != "" {
		t.Errorf("Expected empty string for whitespace only, got: %s", got)
	}
}
