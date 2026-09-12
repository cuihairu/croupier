package generator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

// ---------------------------------------------------------------------------
// 组 M 补测：RecomputeDefaultOutputs 导出包装、HumanizeKey 空输入、
// inline 行操作 selector 的 replace 失败分支（tilde 转义不对称）、
// rawObject Marshal 失败。
// ---------------------------------------------------------------------------

// RecomputeDefaultOutputs 与生成期 defaultOutputAssignments 共用同一默认推导。
func TestRecomputeDefaultOutputsWrapper(t *testing.T) {
	// 空 schema → nil
	assert.Nil(t, RecomputeDefaultOutputs(spec.BindingUsageQuery, spec.JSONSchema(``)))

	// task 辅助绑定（status/events/result）不经此函数 → nil
	assert.Nil(t, RecomputeDefaultOutputs(spec.BindingUsageTaskStatus,
		spec.JSONSchema(`{"type":"object","properties":{"status":{"type":"string"}}}`)))

	// collection query：items 数组 + total 标量
	assignments := RecomputeDefaultOutputs(spec.BindingUsageQuery, spec.JSONSchema(
		`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object"}},"total":{"type":"integer"}}}`,
	))
	require.Len(t, assignments, 2)
	assert.Equal(t, spec.OutputAssignment{
		StateKey: "items",
		Source:   "/items",
		Shape:    spec.OutputShapeCollection,
	}, assignments[0])
	assert.Equal(t, spec.OutputAssignment{
		StateKey: "total",
		Source:   "/total",
		Shape:    spec.OutputShapeScalar,
	}, assignments[1])
}

// HumanizeKey 对空/纯分隔符输入必须返回空串。
func TestHumanizeKeyEmptyInputs(t *testing.T) {
	for _, key := range []string{"", "   ", "._-", " . _ - ", "\t"} {
		assert.Empty(t, HumanizeKey(key), "key=%q", key)
	}
	assert.Equal(t, "Player Ban", HumanizeKey("player.ban"))
}

// replaceSelectorSource 失败分支：topLevelPointerToken 对 token 做 ~0/~1
// 反转义，而 DefaultSelector 的 target 是转义形态——属性名含 "~" 时两侧
// 形态不一致，CanBind 通过但替换落空，必须返回 false 而不是误配。
func TestBuildInlineResourceActionSelectorReplaceFailsOnTildeField(t *testing.T) {
	semantics := &model.CapabilitySemantics{IdentityField: "a~b"}

	// 纯 identity 操作：schemaCanBindBySingleRequiredField 通过，replace 落空
	_, placement, ok := buildInlineResourceActionSelector(&model.FunctionContract{
		InputSchema: model.JSON(`{"type":"object","properties":{"a~b":{"type":"string"}},"required":["a~b"]}`),
	}, semantics, resourceActionSemantic{Subject: "resource_item", IdentityInput: "/a~b"})
	assert.False(t, ok)
	assert.Empty(t, placement)

	// identity + 附加字段：schemaCanBindIdentityPlusFields 通过，replace 落空
	_, placement, ok = buildInlineResourceActionSelector(&model.FunctionContract{
		InputSchema: model.JSON(`{"type":"object","properties":{"a~b":{"type":"string"},"reason":{"type":"string"}},"required":["a~b","reason"]}`),
	}, semantics, resourceActionSemantic{Subject: "resource_item", IdentityInput: "/a~b"})
	assert.False(t, ok)
	assert.Empty(t, placement)

	// resource_selection：数组必填校验通过，但 identity 输入指针的转义形态
	// 与 DefaultSelector 产物不一致 → 替换落空
	_, placement, ok = buildInlineResourceActionSelector(&model.FunctionContract{
		InputSchema: model.JSON(`{"type":"object","properties":{"i~d":{"type":"array","items":{"type":"string"}}},"required":["i~d"]}`),
	}, &model.CapabilitySemantics{IdentityField: "i~d"}, resourceActionSemantic{Subject: "resource_selection", IdentityInput: "/i~d"})
	assert.False(t, ok)
	assert.Empty(t, placement)
}

// rawObject：值里混入非法 JSON RawMessage 时 Marshal 失败必须返回 nil。
func TestRawObjectMarshalFailure(t *testing.T) {
	assert.Nil(t, rawObject(map[string]json.RawMessage{"a": json.RawMessage(`{invalid`)}))
	assert.Nil(t, rawObject(nil))
	assert.Equal(t, json.RawMessage(`{"a":1}`), rawObject(map[string]json.RawMessage{"a": json.RawMessage(`1`)}))
}
