package schema

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 非法 schema（type 为 NaN 数值）应被 schema 定义校验拒绝。
func TestValidateSchemaDefinition_RejectsInvalidKeywordValue(t *testing.T) {
	schema := map[string]interface{}{"type": math.NaN()}

	err := validateSchemaDefinition(schema)

	require.Error(t, err)
}

// 同一非法 schema 在 payload 校验 helper 中也应返回错误而不是 panic。
func TestValidatePayloadAgainst_RejectsInvalidKeywordValue(t *testing.T) {
	schema := map[string]interface{}{"type": math.NaN()}

	valid, issues, err := validatePayloadAgainst(schema, map[string]interface{}{"a": 1})

	require.Error(t, err)
	assert.False(t, valid)
	assert.Nil(t, issues)
}
