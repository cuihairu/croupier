package schemadiff

import (
	"encoding/json"
	"testing"
)

// 非法 JSON：diff 静默返回 nil（校验由各自校验器负责）。
func TestDiffSchemasInvalidJSON(t *testing.T) {
	if got := DiffSchemas("t", json.RawMessage(`{broken`), json.RawMessage(`{}`)); got != nil {
		t.Fatalf("old invalid: want nil, got %v", got)
	}
	if got := DiffSchemas("t", json.RawMessage(`{}`), json.RawMessage(`[broken`)); got != nil {
		t.Fatalf("new invalid: want nil, got %v", got)
	}
}

// 类型对比覆盖基础类型全谱（object/array/string/number/boolean/null）。
func TestJSONTypeNameAllCases(t *testing.T) {
	cases := map[string]any{
		"object":  map[string]any{},
		"array":   []any{},
		"string":  "s",
		"number":  float64(1),
		"boolean": true,
		"null":    nil,
	}
	for want, node := range cases {
		if got := jsonTypeName(node); got != want {
			t.Fatalf("jsonTypeName(%v) = %q, want %q", node, got, want)
		}
	}
	if got := jsonTypeName(make(chan int)); got != "" {
		t.Fatalf("jsonTypeName(chan) = %q, want empty", got)
	}
}

// 类型变更 + 结构变更混合时，breaking 置前并按 Source/Path 排序。
func TestSortFindingsOrdering(t *testing.T) {
	old := mustRawX(t, `{
		"type": "object",
		"properties": {
			"a": "hello",
			"b": {"type": "object", "properties": {"x": {"type": "number"}}}
		},
		"required": ["a", "b"]
	}`)
	new := mustRawX(t, `{
		"type": "object",
		"properties": {
			"a": 42,
			"b": {"type": "object", "properties": {"x": {"type": "string"}}, "description": "doc"}
		},
		"required": ["b"]
	}`)
	findings := DiffSchemas("zeta", old, new)
	if len(findings) < 3 {
		t.Fatalf("expected multiple findings, got %+v", findings)
	}
	for i := 1; i < len(findings); i++ {
		prev, cur := findings[i-1], findings[i]
		if prev.Severity != cur.Severity {
			if cur.Severity == SeverityBreaking {
				t.Fatalf("breaking finding must come first: %+v after %+v", cur, prev)
			}
			continue
		}
		if prev.Source > cur.Source || (prev.Source == cur.Source && prev.Path > cur.Path) {
			t.Fatalf("findings not sorted: %+v before %+v", prev, cur)
		}
	}
}

func mustRawX(t *testing.T, s string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(json.RawMessage(s))
	if err != nil {
		t.Fatalf("marshal raw: %v", err)
	}
	return raw
}
