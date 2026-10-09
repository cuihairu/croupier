package manifest

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestManifestProviderBlockFieldSet 锁死 provider 块字段闭集（#66 P0 验收）：
// 恰好 type/operations/permissions 三键；任何扩键必须同步 binding spec 同构
// 约束（设计 §3.2）后在此放行，防止两侧漂移。
func TestManifestProviderBlockFieldSet(t *testing.T) {
	got := append([]string(nil), providerBlockFields...)
	sort.Strings(got)
	want := []string{"operations", "permissions", "type"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("providerBlockFields = %v, want %v", providerBlockFields, want)
	}
}

func TestParseProviderBlockAbsent(t *testing.T) {
	for name, m := range map[string]map[string]any{
		"nil manifest":   nil,
		"empty manifest": {},
		"no provider":    {"pages": []any{}},
		"null provider":  {"provider": nil},
	} {
		pb, err := ParseProviderBlock(m)
		if err != nil {
			t.Fatalf("%s: ParseProviderBlock: %v", name, err)
		}
		if pb.Type != "" || len(pb.Operations) != 0 || len(pb.Permissions) != 0 {
			t.Fatalf("%s: expected zero block, got %+v", name, pb)
		}
	}
}

func TestParseProviderBlockValid(t *testing.T) {
	pb, err := ParseProviderBlock(map[string]any{
		"provider": map[string]any{
			"type":       "openapi",
			"operations": []any{"day_report", "user_live", "order_list"},
			"permissions": map[string]any{
				"operate": "external-platform.operate",
			},
		},
	})
	if err != nil {
		t.Fatalf("ParseProviderBlock: %v", err)
	}
	if pb.Type != "openapi" {
		t.Fatalf("Type = %q", pb.Type)
	}
	if !reflect.DeepEqual(pb.Operations, []string{"day_report", "user_live", "order_list"}) {
		t.Fatalf("Operations = %v", pb.Operations)
	}
	if pb.Permissions["operate"] != "external-platform.operate" {
		t.Fatalf("Permissions = %v", pb.Permissions)
	}
}

func TestParseProviderBlockDefaultsType(t *testing.T) {
	pb, err := ParseProviderBlock(map[string]any{
		"provider": map[string]any{"operations": []any{"get"}},
	})
	if err != nil {
		t.Fatalf("ParseProviderBlock: %v", err)
	}
	if pb.Type != DefaultProviderType {
		t.Fatalf("Type = %q, want default %q", pb.Type, DefaultProviderType)
	}
}

func TestParseProviderBlockInvalid(t *testing.T) {
	cases := map[string]map[string]any{
		"provider not object":   {"provider": "openapi"},
		"unknown field":         {"provider": map[string]any{"operations": []any{"get"}, "config": map[string]any{}}},
		"type not string":       {"provider": map[string]any{"type": 7, "operations": []any{"get"}}},
		"type outside closure":  {"provider": map[string]any{"type": "grpc", "operations": []any{"get"}}},
		"operations missing":    {"provider": map[string]any{"type": "openapi"}},
		"operations not array":  {"provider": map[string]any{"operations": "get"}},
		"operations empty":      {"provider": map[string]any{"operations": []any{}}},
		"operations bad item":   {"provider": map[string]any{"operations": []any{""}}},
		"operations non string": {"provider": map[string]any{"operations": []any{7}}},
		"operations duplicate":  {"provider": map[string]any{"operations": []any{"get", "get"}}},
		"permissions not obj":   {"provider": map[string]any{"operations": []any{"get"}, "permissions": []any{}}},
		"permissions bad tier":  {"provider": map[string]any{"operations": []any{"get"}, "permissions": map[string]any{"write": "x"}}},
		"permissions bad value": {"provider": map[string]any{"operations": []any{"get"}, "permissions": map[string]any{"read": 3}}},
	}
	for name, m := range cases {
		_, err := ParseProviderBlock(m)
		if err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
}

func TestParseProviderBlockErrorMentionsFieldSet(t *testing.T) {
	_, err := ParseProviderBlock(map[string]any{
		"provider": map[string]any{"operations": []any{"get"}, "unknown": 1},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("err = %v, want unknown field message", err)
	}
}
