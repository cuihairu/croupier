package alert

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/model"
)

// 告警规则输入校验与小工具：非法 metric/operator/level 必须 400 拦截。

func TestValidateRuleInput(t *testing.T) {
	// 合法。
	require.NoError(t, validateRuleInput("cpu.usagePercent", "gt", "warning"))
	require.NoError(t, validateRuleInput("memory.usedBytes", "gte", "critical"))
	require.NoError(t, validateRuleInput("disk./data.usedPercent", "lt", "info"))
	require.NoError(t, validateRuleInput("custom.foo", "lte", ""))
	require.NoError(t, validateRuleInput("cpu.usagePercent", "gt", ""))

	// 非法 metric。
	err := validateRuleInput("gpu.usage", "gt", "warning")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "非法指标")

	err = validateRuleInput("disk.", "gt", "warning")
	require.Error(t, err)

	err = validateRuleInput("disk./x.temperature", "gt", "warning")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usedPercent/usedBytes")

	err = validateRuleInput("custom.", "gt", "warning")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "custom 指标缺少 key")

	err = validateRuleInput("", "gt", "warning")
	require.Error(t, err)

	// 非法 operator。
	err = validateRuleInput("cpu.usagePercent", "ne", "warning")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "operator")

	// 非法 level。
	err = validateRuleInput("cpu.usagePercent", "gt", "fatal")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "level")
}

func TestMaxOne(t *testing.T) {
	assert.Equal(t, 1, maxOne(0))
	assert.Equal(t, 1, maxOne(-3))
	assert.Equal(t, 1, maxOne(1))
	assert.Equal(t, 30, maxOne(30))
}

func TestDefaultInt(t *testing.T) {
	assert.Equal(t, 10, defaultInt(0, 10))
	assert.Equal(t, 10, defaultInt(-1, 10))
	assert.Equal(t, 5, defaultInt(5, 10))
}

func TestDerefString(t *testing.T) {
	s := "hello"
	assert.Equal(t, "hello", derefString(&s, "def"))
	assert.Equal(t, "def", derefString(nil, "def"))
	assert.Equal(t, "", derefString(nil, ""))
}

func TestValidLevelsTable(t *testing.T) {
	// 约束与 model 常量对齐，防漂移。
	assert.True(t, validLevels[model.AlertRuleLevelInfo])
	assert.True(t, validLevels[model.AlertRuleLevelWarning])
	assert.True(t, validLevels[model.AlertRuleLevelCritical])
	assert.False(t, validLevels["fatal"])
}
