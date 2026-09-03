package schemadiff

import (
	"encoding/json"
	"testing"
)

func mustRaw(t *testing.T, v string) json.RawMessage {
	t.Helper()
	return json.RawMessage(v)
}

func findByPath(findings []Finding, path string) (Finding, bool) {
	for _, finding := range findings {
		if finding.Path == path {
			return finding, true
		}
	}
	return Finding{}, false
}

// 1. 新增 required = breaking
func TestDiffRequiredAdded(t *testing.T) {
	oldRaw := mustRaw(t, `{"type":"object","properties":{"a":{"type":"string"}}}`)
	newRaw := mustRaw(t, `{"type":"object","properties":{"a":{"type":"string"}},"required":["a"]}`)
	findings := DiffSchemas("inputSchema", oldRaw, newRaw)
	if !HasBreaking(findings) {
		t.Fatalf("expected breaking finding, got %+v", findings)
	}
	if _, ok := findByPath(findings, "$/required/a"); !ok {
		t.Fatalf("expected required/a finding, got %+v", findings)
	}
}

// 2. 删除已有 properties = breaking
func TestDiffPropertyRemoved(t *testing.T) {
	oldRaw := mustRaw(t, `{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"integer"}}}`)
	newRaw := mustRaw(t, `{"type":"object","properties":{"a":{"type":"string"}}}`)
	findings := DiffSchemas("inputSchema", oldRaw, newRaw)
	if !HasBreaking(findings) {
		t.Fatalf("expected breaking finding, got %+v", findings)
	}
	if finding, ok := findByPath(findings, "$/b"); !ok || finding.Reason == "" {
		t.Fatalf("expected $/b removal finding, got %+v", findings)
	}
}

// 3. 类型变更 = breaking（schema type 关键字）
func TestDiffTypeChanged(t *testing.T) {
	oldRaw := mustRaw(t, `{"type":"object","properties":{"a":{"type":"string"}}}`)
	newRaw := mustRaw(t, `{"type":"object","properties":{"a":{"type":"integer"}}}`)
	findings := DiffSchemas("inputSchema", oldRaw, newRaw)
	if !HasBreaking(findings) {
		t.Fatalf("expected breaking finding, got %+v", findings)
	}
	if _, ok := findByPath(findings, "$/a"); !ok {
		t.Fatalf("expected $/a type finding, got %+v", findings)
	}
}

//  4. enum 方向性（审查修正）：input 收窄 = breaking、扩张 = compatible；
//     output 恰好相反。
func TestDiffEnumDirectional(t *testing.T) {
	oldRaw := mustRaw(t, `{"type":"object","properties":{"level":{"type":"string","enum":["low","high"]}}}`)
	narrowed := mustRaw(t, `{"type":"object","properties":{"level":{"type":"string","enum":["low"]}}}`)
	expanded := mustRaw(t, `{"type":"object","properties":{"level":{"type":"string","enum":["low","high","critical"]}}}`)

	// inputSchema：收窄 = breaking（旧调用发被删值会被拒）
	if findings := DiffSchemas("inputSchema", oldRaw, narrowed); !HasBreaking(findings) {
		t.Fatalf("input enum narrowing should be breaking, got %+v", findings)
	}
	// inputSchema：扩张 = compatible（旧调用方不受影响）
	if findings := DiffSchemas("inputSchema", oldRaw, expanded); HasBreaking(findings) {
		t.Fatalf("input enum expansion should be compatible, got %+v", findings)
	}
	// outputSchema：扩张 = breaking（消费方会见到新值）
	if findings := DiffSchemas("outputSchema", oldRaw, expanded); !HasBreaking(findings) {
		t.Fatalf("output enum expansion should be breaking, got %+v", findings)
	}
	// outputSchema：收窄 = compatible
	if findings := DiffSchemas("outputSchema", oldRaw, narrowed); HasBreaking(findings) {
		t.Fatalf("output enum narrowing should be compatible, got %+v", findings)
	}
}

// 5. 新增可选字段 / 描述变更 = compatible
func TestDiffCompatibleChanges(t *testing.T) {
	oldRaw := mustRaw(t, `{"type":"object","properties":{"a":{"type":"string","title":"A"}},"required":["a"]}`)
	newRaw := mustRaw(t, `{"type":"object","properties":{"a":{"type":"string","title":"A","description":"field a"},"b":{"type":"integer"}},"required":["a"]}`)
	findings := DiffSchemas("inputSchema", oldRaw, newRaw)
	if HasBreaking(findings) {
		t.Fatalf("expected only compatible findings, got %+v", findings)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 compatible findings (new field + description), got %+v", findings)
	}
}

// 结构对比：结构类型变化（object → scalar）breaking
func TestDiffStructureTypeChanged(t *testing.T) {
	oldRaw := mustRaw(t, `{"type":"object","properties":{"profile":{"type":"object"}}}`)
	newRaw := mustRaw(t, `{"type":"object","properties":{"profile":{"type":"string"}}}`)
	findings := DiffSchemas("inputSchema", oldRaw, newRaw)
	if !HasBreaking(findings) {
		t.Fatalf("expected breaking finding, got %+v", findings)
	}
}

// 首次注册（旧 schema 为空）与非法 JSON 不产生差异
func TestDiffEmptyAndInvalid(t *testing.T) {
	if findings := DiffSchemas("inputSchema", nil, mustRaw(t, `{"type":"object"}`)); len(findings) != 0 {
		t.Fatalf("expected no findings for initial registration, got %+v", findings)
	}
	if findings := DiffSchemas("inputSchema", mustRaw(t, `not-json`), mustRaw(t, `{"type":"object"}`)); len(findings) != 0 {
		t.Fatalf("expected no findings for invalid old schema, got %+v", findings)
	}
}

// 嵌套 properties 的破坏性变更可检出
func TestDiffNestedBreaking(t *testing.T) {
	oldRaw := mustRaw(t, `{"type":"object","properties":{"profile":{"type":"object","properties":{"city":{"type":"string"}}}}}`)
	newRaw := mustRaw(t, `{"type":"object","properties":{"profile":{"type":"object","properties":{}}}}`)
	findings := DiffSchemas("inputSchema", oldRaw, newRaw)
	if !HasBreaking(findings) {
		t.Fatalf("expected nested breaking finding, got %+v", findings)
	}
	if _, ok := findByPath(findings, "$/profile/city"); !ok {
		t.Fatalf("expected $/profile/city finding, got %+v", findings)
	}
}

// source 区分 input/output
func TestDiffSourceLabels(t *testing.T) {
	oldRaw := mustRaw(t, `{"type":"object","properties":{"a":{"type":"string"}}}`)
	findings := DiffSchemas("outputSchema", oldRaw, mustRaw(t, `{"type":"object","properties":{}}`))
	if len(findings) != 1 || findings[0].Source != "outputSchema" {
		t.Fatalf("expected single outputSchema finding, got %+v", findings)
	}
}
