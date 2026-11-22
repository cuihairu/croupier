package incident

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

// AUD-020：事故详情更新使用数据库可序列化的 JSON 类型。
//
// 模型 Details 列是 JSON 类型（datatypes.JSONMap），此前 UpdateIncident 把裸
// map 直接塞进 updates map——gorm 的 map 更新绕过列类型的序列化器，普通详情
// 更新一律失败：sql: unsupported type map[string]interface {}。本用例覆盖
// 正常对象、嵌套值、空对象的保存与回读。
func TestUpdateIncident_DetailsJSONRoundtrip(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	inc, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "详情", CategoryID: qa.ID,
		Details: map[string]interface{}{"reason": "first"},
	}, "tester")
	require.NoError(t, err)
	require.Equal(t, "first", inc.Details["reason"])

	// 正常对象 + 嵌套值（列表与对象）。
	upd, err := s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{
		Details: map[string]interface{}{
			"reason": "second",
			"steps":  []interface{}{"step-1", "step-2"},
			"meta":   map[string]interface{}{"owner": "ops"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "second", upd.Details["reason"])

	steps, ok := upd.Details["steps"].([]interface{})
	require.True(t, ok, "list value should round-trip")
	require.Equal(t, []interface{}{"step-1", "step-2"}, steps)

	nested, ok := upd.Details["meta"].(map[string]interface{})
	require.True(t, ok, "nested object should round-trip")
	require.Equal(t, "ops", nested["owner"])

	// 独立读回（非 Update 返回值，走 GetIncident 再查库）。
	got, err := s.GetIncident(ctx, inc.ID)
	require.NoError(t, err)
	require.Equal(t, "second", got.Details["reason"])
	require.Len(t, got.Details, 3)

	// 空对象：保存为 {} 并回读，不得静默丢弃或报错。
	upd, err = s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{Details: map[string]interface{}{}})
	require.NoError(t, err)
	require.NotNil(t, upd.Details)
	require.Empty(t, upd.Details)
	require.Len(t, upd.Details, 0)

	got, err = s.GetIncident(ctx, inc.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Details)
	require.Len(t, got.Details, 0)
}
