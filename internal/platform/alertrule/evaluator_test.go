package alertrule

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setup(t *testing.T) (*Evaluator, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/alertrule.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return New(model.NewAlertRuleModel(db), model.NewAlertModel(db), nil), db
}

func report(cpu, memUsed, memTotal float64) *opsv1.MetricsReport {
	return &opsv1.MetricsReport{
		Cpu:    &opsv1.CpuMetrics{UsagePercent: cpu},
		Memory: &opsv1.MemoryMetrics{UsedBytes: uint64(memUsed), TotalBytes: uint64(memTotal)},
	}
}

func TestExtractMetric(t *testing.T) {
	r := report(75.5, 800, 1000)
	r.Disks = []*opsv1.DiskMetrics{
		{MountPoint: "/data", UsedBytes: 900, TotalBytes: 1000, UsagePercent: 90},
	}
	r.Custom = map[string]float64{"queueDepth": 42}

	v, ok := ExtractMetric("cpu.usagePercent", r)
	require.True(t, ok)
	assert.InDelta(t, 75.5, v, 0.01)

	v, ok = ExtractMetric("memory.usagePercent", r)
	require.True(t, ok)
	assert.InDelta(t, 80.0, v, 0.01)

	v, ok = ExtractMetric("memory.usedBytes", r)
	require.True(t, ok)
	assert.InDelta(t, 800, v, 0.01)

	v, ok = ExtractMetric("disk./data.usedPercent", r)
	require.True(t, ok)
	assert.InDelta(t, 90, v, 0.01)

	v, ok = ExtractMetric("custom.queueDepth", r)
	require.True(t, ok)
	assert.InDelta(t, 42, v, 0.01)

	// 缺失指标。
	_, ok = ExtractMetric("disk./missing.usedPercent", r)
	assert.False(t, ok)
	_, ok = ExtractMetric("custom.nope", r)
	assert.False(t, ok)
	_, ok = ExtractMetric("bogus.path", r)
	assert.False(t, ok)
}

func TestExtractMetricFromName(t *testing.T) {
	for _, ok := range []string{
		"cpu.usagePercent", "memory.usagePercent", "memory.usedBytes",
		"disk./data.usedPercent", "disk./data.usedBytes", "custom.queueDepth",
	} {
		_, err := ExtractMetricFromName(ok)
		assert.NoError(t, err, ok)
	}
	for _, bad := range []string{"", "cpu", "disk.onlyfield", "disk./x.bogus", "custom.", "weird"} {
		_, err := ExtractMetricFromName(bad)
		assert.Error(t, err, bad)
	}
}

func TestCompare(t *testing.T) {
	assert.True(t, Compare(10, "gt", 5))
	assert.False(t, Compare(10, "gt", 10))
	assert.True(t, Compare(10, "gte", 10))
	assert.True(t, Compare(1, "lt", 5))
	assert.True(t, Compare(5, "lte", 5))
	assert.False(t, Compare(5, "bogus", 1))
}

func TestEvaluator_FiresAndCooldown(t *testing.T) {
	e, db := setup(t)
	rule := &model.AlertRule{
		Name: "cpu 高负载", Metric: "cpu.usagePercent", Operator: "gt",
		Threshold: 90, Level: model.AlertRuleLevelCritical, Enabled: true,
	}
	require.NoError(t, model.NewAlertRuleModel(db).Create(context.Background(), rule))

	// 第一次命中（ForCount=1）→ 立即触发。
	e.EvaluateAgent(context.Background(), "agent-1", report(95, 0, 0))
	var count int64
	require.NoError(t, db.Model(&model.Alert{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)

	// 冷却期内再命中 → 不重复触发。
	e.EvaluateAgent(context.Background(), "agent-1", report(96, 0, 0))
	require.NoError(t, db.Model(&model.Alert{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)

	// 未命中 → 计数清零（数据库回写）。
	e.EvaluateAgent(context.Background(), "agent-1", report(50, 0, 0))
	var updated model.AlertRule
	require.NoError(t, db.First(&updated, rule.ID).Error)
	assert.Zero(t, updated.HitCount)
}

func TestEvaluator_ForCount(t *testing.T) {
	e, db := setup(t)
	rule := &model.AlertRule{
		Name: "内存持续高", Metric: "memory.usagePercent", Operator: "gte",
		Threshold: 80, ForCount: 3, Level: model.AlertRuleLevelWarning, Enabled: true,
	}
	require.NoError(t, model.NewAlertRuleModel(db).Create(context.Background(), rule))

	e.EvaluateAgent(context.Background(), "a", report(0, 85, 100))
	e.EvaluateAgent(context.Background(), "a", report(0, 85, 100))
	var count int64
	require.NoError(t, db.Model(&model.Alert{}).Count(&count).Error)
	assert.Zero(t, count, "两次命中不应触发（需连续 3 次）")

	e.EvaluateAgent(context.Background(), "a", report(0, 85, 100))
	require.NoError(t, db.Model(&model.Alert{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestEvaluator_AgentFilterAndDisabled(t *testing.T) {
	e, db := setup(t)
	rules := model.NewAlertRuleModel(db)
	require.NoError(t, rules.Create(context.Background(), &model.AlertRule{
		Name: "仅 agent-2", Metric: "cpu.usagePercent", Operator: "gt",
		Threshold: 50, Enabled: true, AgentFilter: "agent-2",
	}))
	require.NoError(t, rules.Create(context.Background(), &model.AlertRule{
		Name: "已禁用", Metric: "cpu.usagePercent", Operator: "gt",
		Threshold: 1, Enabled: false,
	}))

	e.EvaluateAgent(context.Background(), "agent-1", report(99, 0, 0))
	var count int64
	require.NoError(t, db.Model(&model.Alert{}).Count(&count).Error)
	assert.Zero(t, count, "agent 过滤与禁用规则都不应触发")

	e.EvaluateAgent(context.Background(), "agent-2", report(99, 0, 0))
	require.NoError(t, db.Model(&model.Alert{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestEvaluator_NilSafety(t *testing.T) {
	// 全 nil / 空上报不 panic。
	var e *Evaluator
	assert.NotPanics(t, func() { e.EvaluateAgent(context.Background(), "a", report(1, 0, 0)) })
	e2 := New(nil, nil, nil)
	assert.NotPanics(t, func() { e2.EvaluateAgent(context.Background(), "a", nil) })
}
