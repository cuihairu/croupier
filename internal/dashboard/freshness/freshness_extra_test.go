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
