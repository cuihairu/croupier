package console

// 设计债回归测试：TransformDefault 的 Params["value"] 原先直接作为
// json.RawMessage 返回，不做 JSON 合法性校验；DB 直改/绕过 spec 校验的非法
// 值会延迟到最终 Marshal 报 500 而非语义化 422。已定案修法：value 存在且
// 非空时校验 json.Valid，非法则返回 ValidationError；缺失/空值维持原状
// （validDefaultParams 在 spec 校验层把关）。

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 非法 JSON 兜底值 → ValidationError（422），错误信息定位到 default transform。
func TestResolveSelectorValue_DefaultTransformInvalidJSON(t *testing.T) {
	for _, invalid := range []string{`{"a":`, `not-json`, `{"a":1,`} {
		_, _, err := resolveSelectorValue(spec.ValueSource{
			Kind: spec.SourcePageState,
			Key:  "missing",
			Transform: &spec.TransformSpec{
				Type:   spec.TransformDefault,
				Params: map[string]json.RawMessage{"value": json.RawMessage(invalid)},
			},
		}, ConsoleBindingExecutionContext{PageState: map[string]json.RawMessage{}})
		require.Error(t, err, "invalid value %q must be rejected", invalid)
		codeErr, ok := err.(*errorx.CodeError)
		require.True(t, ok)
		assert.Equal(t, 422, codeErr.Code)
		assert.Contains(t, err.Error(), "default transform value is not valid JSON")
	}
}

// 合法 JSON 兜底值 → 原样返回（不重编码）。
func TestResolveSelectorValue_DefaultTransformValidJSONPassthrough(t *testing.T) {
	val, found, err := resolveSelectorValue(spec.ValueSource{
		Kind: spec.SourcePageState,
		Key:  "missing",
		Transform: &spec.TransformSpec{
			Type:   spec.TransformDefault,
			Params: map[string]json.RawMessage{"value": json.RawMessage(`{"page":1,"size":20}`)},
		},
	}, ConsoleBindingExecutionContext{PageState: map[string]json.RawMessage{}})
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `{"page":1,"size":20}`, string(val))
	assert.Equal(t, `{"page":1,"size":20}`, string(val), "合法兜底值必须原样透传")
}

// value 缺失或为空 → 维持现状（返回空 RawMessage，交给 spec 校验层把关），
// 不得误报 JSON 非法。
func TestResolveSelectorValue_DefaultTransformMissingValueKept(t *testing.T) {
	for name, params := range map[string]map[string]json.RawMessage{
		"missing": {},
		"empty":   {"value": json.RawMessage(``)},
	} {
		val, found, err := resolveSelectorValue(spec.ValueSource{
			Kind: spec.SourcePageState,
			Key:  "missing",
			Transform: &spec.TransformSpec{
				Type:   spec.TransformDefault,
				Params: params,
			},
		}, ConsoleBindingExecutionContext{PageState: map[string]json.RawMessage{}})
		require.NoError(t, err, "%s: 缺失/空 value 不在校验范围", name)
		assert.True(t, found)
		assert.Empty(t, val)
	}
}
