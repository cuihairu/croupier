package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// /ops/alerts 是基础设施告警中心：ExcludeSources 过滤业务来源
// （contract 域 schema 破坏性变更），不与基础设施告警混排。
func TestAlertModelListExcludeSources(t *testing.T) {
	ctx := context.Background()
	db := setupAllModelsDB(t)
	m := NewAlertModel(db)

	require.NoError(t, m.Create(ctx, &Alert{AlertID: "infra-1", Type: "disk", Level: "warning", Source: "alertrule", Status: "firing"}))
	require.NoError(t, m.Create(ctx, &Alert{AlertID: "infra-2", Type: "dbmon", Level: "critical", Source: "dbmon", Status: "firing"}))
	require.NoError(t, m.Create(ctx, &Alert{AlertID: "biz-1", Type: "schema_diff", Level: "warning", Source: "contract", Status: "firing"}))

	all, total, err := m.List(ctx, ListAlertsOptions{})
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	assert.Len(t, all, 3)

	infra, total, err := m.List(ctx, ListAlertsOptions{ExcludeSources: []string{"contract"}})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	sources := map[string]bool{}
	for _, a := range infra {
		sources[a.Source] = true
	}
	assert.True(t, sources["alertrule"])
	assert.True(t, sources["dbmon"])
	assert.False(t, sources["contract"], "业务告警不应出现在基础设施告警列表")

	// Source 正向过滤与 ExcludeSources 组合仍生效
	onlyRule, _, err := m.List(ctx, ListAlertsOptions{Source: "alertrule", ExcludeSources: []string{"contract"}})
	require.NoError(t, err)
	assert.Len(t, onlyRule, 1)
	assert.Equal(t, "infra-1", onlyRule[0].AlertID)
}
