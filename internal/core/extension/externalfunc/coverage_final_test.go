package externalfunc

import "testing"

// 覆盖 ParseProviderBinding 的 type/operations 兜底分支：非法字符 key 经
// SanitizeKey 归一为空后回落默认值。
func TestParseProviderBindingSanitizeFallbacksV2(t *testing.T) {
	// type 全为非法字符 → SanitizeKey 归一为空 → 回落 openapi。
	got, ok := ParseProviderBinding("p1", map[string]any{"type": "!!!"})
	if !ok {
		t.Fatal("expected binding to parse")
	}
	if got.Type != "openapi" {
		t.Fatalf("type = %q, want openapi fallback", got.Type)
	}

	// operations 全部 sanitize 为空 → 回落 invoke。
	got, ok = ParseProviderBinding("p2", map[string]any{"operations": []any{"!!!", "@@@"}})
	if !ok {
		t.Fatal("expected binding to parse")
	}
	if len(got.Operations) != 1 || got.Operations[0] != "invoke" {
		t.Fatalf("operations = %v, want [invoke] fallback", got.Operations)
	}
}

// 覆盖 addProviderOperations 的 provider 为空分支：function 绑定的 provider
// 段sanitize 后为空时直接丢弃。
func TestDiscoverProviderOperationsSanitizedEmptyProviderV2(t *testing.T) {
	out := DiscoverProviderOperations([]Binding{
		{BindingType: "function", BindingKey: "external.!!!.method"},
	})
	if len(out) != 0 {
		t.Fatalf("expected no providers, got %v", out)
	}
}

// 覆盖 addProviderOperations 中 operation sanitize 为空的 continue 分支。
func TestDiscoverProviderOperationsSanitizedEmptyOperationV2(t *testing.T) {
	out := DiscoverProviderOperations([]Binding{
		{BindingType: "function", BindingKey: "external.provider.!!!"},
	})
	ops, ok := out["provider"]
	if !ok {
		t.Fatalf("expected provider bucket, got %v", out)
	}
	if len(ops) != 0 {
		t.Fatalf("expected empty ops, got %v", ops)
	}
}

// 覆盖 CapabilityOperationFromFunctionID 中 operation sanitize 为空的分支。
func TestCapabilityOperationFromFunctionIDSanitizedEmptyMethodV2(t *testing.T) {
	capability, operation, ok := CapabilityOperationFromFunctionID("external.provider.!!!")
	if ok {
		t.Fatalf("expected ok=false, got capability=%q operation=%q", capability, operation)
	}
	if capability != "" || operation != "" {
		t.Fatalf("expected empty capability/operation, got %q/%q", capability, operation)
	}
}
