package normalizer

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Normalize：合法 output schema 必须原样进入 FunctionSpec，不产生诊断。
func TestNormalizeValidOutputSchema(t *testing.T) {
	result := Normalize(DescriptorInput{
		ID:          "reward.batch_grant",
		Resource:    "reward",
		Operation:   "batch_grant",
		Capability:  "task",
		Execution:   "task",
		InputSchema: `{"type":"object"}`,
		OutputSchema: `{"type":"object","properties":{"granted":{"type":"boolean"},` +
			`"skipped":{"type":"array","items":{"type":"string"}}}}`,
		Enabled: true,
	})

	for _, diag := range result.Diagnostics {
		assert.NotEqual(t, "output_schema_invalid", diag.Code)
	}
	var canonical map[string]interface{}
	require.NoError(t, json.Unmarshal(result.Function.OutputSchema, &canonical))
	require.Contains(t, canonical, "properties")
	props, ok := canonical["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, props, "granted")
	assert.Contains(t, props, "skipped")
}

// canonicalJSON：标量分支的 json.Compact 校验失败必须返回错误（非 JSON 标量）。
func TestCanonicalJSONScalarCompactError(t *testing.T) {
	if _, err := canonicalJSON(json.RawMessage(`1x`)); err == nil {
		t.Fatal("canonicalJSON of an invalid scalar must return an error")
	}
	if _, err := canonicalJSON(json.RawMessage(`nan`)); err == nil {
		t.Fatal("canonicalJSON of a bare identifier must return an error")
	}
	// 合法标量正常紧凑化
	out, err := canonicalJSON(json.RawMessage(`  42  `))
	if err != nil || string(out) != "42" {
		t.Fatalf("canonicalJSON(42) = %q, %v", out, err)
	}
	// valuesEqual 在一侧非法时判不等（不 panic）
	assert.False(t, valuesEqual(json.RawMessage(`1x`), json.RawMessage(`1`)))
}
