package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- ToOpenAPIOperation: Schema.UnmarshalJSON 失败分支 ----

// InputSchema 为合法 JSON 但不是合法 OpenAPI Schema（type 非字符串/数组）
// → openapi3.Schema.UnmarshalJSON 报错。
func TestToOpenAPIOperation_InputSchemaInvalidSchemaV5(t *testing.T) {
	t.Parallel()
	_, err := ToOpenAPIOperation(ProviderFunctionDescriptorDesc{
		OperationID: "player.get",
		Summary:     "Get Player",
		InputSchema: `{"type":123}`,
	})
	require.Error(t, err)
}

// ---- OutputSchema 同上 ----
func TestToOpenAPIOperation_OutputSchemaInvalidSchemaV5(t *testing.T) {
	t.Parallel()
	_, err := ToOpenAPIOperation(ProviderFunctionDescriptorDesc{
		OperationID:  "player.get",
		OutputSchema: `{"type":123}`,
	})
	require.Error(t, err)
}

// ---- Risk 扩展：Resource 为空时 Extensions 尚未初始化的 make 分支 ----
func TestToOpenAPIOperation_RiskWithoutResourceV5(t *testing.T) {
	t.Parallel()
	op, err := ToOpenAPIOperation(ProviderFunctionDescriptorDesc{
		OperationID: "player.ban",
		Risk:        "high",
	})
	require.NoError(t, err)
	require.NotNil(t, op.Extensions)
	assert.Equal(t, "high", op.Extensions["x-risk"])
	assert.NotContains(t, op.Extensions, "x-resource")
}

// ---- Operation 扩展：Resource/Risk 均为空时的 make 分支 ----
func TestToOpenAPIOperation_OperationOnlyV5(t *testing.T) {
	t.Parallel()
	op, err := ToOpenAPIOperation(ProviderFunctionDescriptorDesc{
		OperationID: "player.kick",
		Operation:   "kick",
	})
	require.NoError(t, err)
	require.NotNil(t, op.Extensions)
	assert.Equal(t, "kick", op.Extensions["x-operation"])
	assert.NotContains(t, op.Extensions, "x-resource")
	assert.NotContains(t, op.Extensions, "x-risk")
}
