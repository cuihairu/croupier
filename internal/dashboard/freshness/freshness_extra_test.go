package freshness

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// 旧发布快照（digest 机制之前）无 digest 字段——必须视为兼容，
// 不得误判 stale 阻断执行（用户实测：旧页面第一次可执行、刷新后
// 被 binding_stale 阻断）。
func TestEmptyContractDigestIsCompatible(t *testing.T) {
	fn := spec.FunctionSpec{
		ID:           "player.list",
		InputSchema:  spec.JSONSchema(`{"type":"object","properties":{"page":{"type":"integer"}}}`),
		OutputSchema: spec.JSONSchema(`{"type":"object","properties":{"items":{"type":"array"}}}`),
	}
	contract := spec.BindingContractSnapshot{
		FunctionID: "player.list",
		// InputSchemaDigest / OutputSchemaDigest 均为空（旧快照）
	}
	binding := spec.PageFunctionBinding{ID: "list", FunctionID: "player.list"}
	diags := EvaluateBinding(binding, contract, map[string]spec.FunctionSpec{"player.list": fn})
	for _, d := range diags {
		if d.Status == spec.BindingFreshnessInputSchemaStale || d.Status == spec.BindingFreshnessOutputSchemaStale {
			t.Fatalf("empty digest must be compatible, got %s", d.Status)
		}
	}
}

// 同一 schema 的不同 JSON 字节形态（键序/空格差异——六语言 SDK 各自
// 序列化）digest 必须一致：canonical 匹配；旧快照存的原始字节 digest
// 也必须继续命中（dual match）。
func TestDigestMatchAcrossByteForms(t *testing.T) {
	a := []byte(`{"type":"object","properties":{"page":{"type":"integer"},"items":{"type":"array"}}}`)
	b := []byte(`{ "properties" : { "items" : { "type" : "array" } , "page" : { "type" : "integer" } } , "type" : "object" }`)
	if digestRaw(a) != digestRaw(b) {
		t.Fatal("canonical digest must be byte-form independent")
	}
	if !digestMatch(a, digestRaw(a)) || !digestMatch(b, digestRaw(a)) {
		t.Fatal("canonical match failed")
	}
	// 旧快照原始字节 digest 兼容
	if !digestMatch(a, digestRawBytes(a)) {
		t.Fatal("legacy raw digest must still match")
	}
	// 真变化的 schema 不匹配
	c := []byte(`{"type":"object","properties":{"total":{"type":"integer"}}}`)
	if digestMatch(a, digestRaw(c)) {
		t.Fatal("different schema must not match")
	}
}
