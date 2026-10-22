package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UpdateLifecycle 生命周期转换一次落库：status + message + details 同语句
// 更新（探测窗口开/合转换消费方，internal/server/probe_lifecycle.go）。
func TestAlertModelUpdateLifecycle(t *testing.T) {
	ctx := context.Background()
	db := setupAllModelsDB(t)
	m := NewAlertModel(db)

	require.NoError(t, m.Create(ctx, &Alert{
		AlertID: "probe:agent-1:redis", Type: "probe", Level: "warning",
		Message: "探针目标 redis 探测不可用（readiness）", Source: "probe",
		Status: "firing", Details: map[string]interface{}{"windowKind": "unavailable"},
		GameID: "demo", Env: "prod",
	}))

	// 开窗 → 闭窗：status/message/details 同步刷新，不产生新行。
	newDetails := map[string]interface{}{
		"windowKind": "recovered", "durationMs": 12000,
		"recentFailures": []map[string]interface{}{{"err": "timeout"}},
	}
	require.NoError(t, m.UpdateLifecycle(ctx, "probe:agent-1:redis", "resolved", "探针目标 redis 恢复可用（故障持续 12000ms）", newDetails))

	row, err := m.FindByAlertID(ctx, "probe:agent-1:redis")
	require.NoError(t, err)
	assert.Equal(t, "resolved", row.Status)
	assert.Equal(t, "探针目标 redis 恢复可用（故障持续 12000ms）", row.Message)
	assert.Equal(t, "recovered", row.Details["windowKind"])

	failures, ok := row.Details["recentFailures"].([]interface{})
	require.True(t, ok, "recentFailures round-trips as array, got %T", row.Details["recentFailures"])
	first, ok := failures[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "timeout", first["err"])

	// 空幂等键拒绝。
	require.Error(t, m.UpdateLifecycle(ctx, " ", "resolved", "x", nil))
}

// details 的 datatypes.JSONMap 包装是序列化的硬要求：裸 map 直更不被驱动接受
// （sqlite 报 unsupported type map）。回归锚：若有人改回裸 map，此测试即红。
func TestAlertModelUpdateLifecycleJSONRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := setupAllModelsDB(t)
	m := NewAlertModel(db)

	require.NoError(t, m.Create(ctx, &Alert{
		AlertID: "probe:agent-1:db", Type: "probe", Level: "warning",
		Message: "m1", Source: "probe", Status: "firing",
	}))
	require.NoError(t, m.UpdateLifecycle(ctx, "probe:agent-1:db", "resolved", "m2",
		map[string]interface{}{"latencyBuckets": []int64{1, 2, 3}}))
	row, err := m.FindByAlertID(ctx, "probe:agent-1:db")
	require.NoError(t, err)
	assert.Equal(t, "resolved", row.Status)
	assert.Equal(t, "m2", row.Message)
	assert.NotNil(t, row.Details["latencyBuckets"])
}
