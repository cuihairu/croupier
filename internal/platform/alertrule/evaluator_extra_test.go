package alertrule

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func mustCreateRule(t *testing.T, m *model.AlertRuleModel, rule *model.AlertRule) *model.AlertRule {
	t.Helper()
	require.NoError(t, m.Create(context.Background(), rule))
	return rule
}

func alertCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&model.Alert{}).Count(&count).Error)
	return count
}

func setRuleColumn(t *testing.T, db *gorm.DB, ruleID uint, column string, value interface{}) {
	t.Helper()
	require.NoError(t, db.Model(&model.AlertRule{}).Where("id = ?", ruleID).Update(column, value).Error)
}

// ---- 冷却语义 ----

func TestEvaluator_CooldownExpiryWithCustomWindow(t *testing.T) {
	e, db := setup(t)
	rule := mustCreateRule(t, model.NewAlertRuleModel(db), &model.AlertRule{
		Name: "cpu", Metric: "cpu.usagePercent", Operator: "gt",
		Threshold: 90, ForCount: 1, CooldownSeconds: 3600, Level: model.AlertRuleLevelCritical, Enabled: true,
	})
	ctx := context.Background()

	// 首次命中触发。
	e.EvaluateAgent(ctx, "a1", report(95, 0, 0))
	assert.EqualValues(t, 1, alertCount(t, db))

	// 冷却期(1h)内再次命中 → 抑制。
	e.EvaluateAgent(ctx, "a1", report(96, 0, 0))
	assert.EqualValues(t, 1, alertCount(t, db))

	// 将 last_fired_at 回拨 2h → 冷却过期 → 再次触发。
	// AlertID 含秒级时间戳且建了唯一索引,两次触发需跨秒否则静默冲突。
	setRuleColumn(t, db, rule.ID, "last_fired_at", time.Now().UTC().Add(-2*time.Hour))
	time.Sleep(1100 * time.Millisecond)
	e.EvaluateAgent(ctx, "a1", report(97, 0, 0))
	assert.EqualValues(t, 2, alertCount(t, db))

	var updated model.AlertRule
	require.NoError(t, db.First(&updated, rule.ID).Error)
	require.NotNil(t, updated.LastFiredAt)
	assert.WithinDuration(t, time.Now().UTC(), *updated.LastFiredAt, time.Minute)
	assert.Zero(t, updated.HitCount, "触发后连续命中计数应清零")
}

func TestEvaluator_ZeroCooldownFallsBackTo5Min(t *testing.T) {
	e, db := setup(t)
	rule := mustCreateRule(t, model.NewAlertRuleModel(db), &model.AlertRule{
		Name: "cpu", Metric: "cpu.usagePercent", Operator: "gt",
		Threshold: 90, ForCount: 1, Enabled: true,
	})
	// 显式落 0,覆盖 cooldown<=0 分支(gorm Create 会跳过零值走列默认值)。
	setRuleColumn(t, db, rule.ID, "cooldown_seconds", 0)

	ctx := context.Background()
	e.EvaluateAgent(ctx, "a1", report(95, 0, 0))
	assert.EqualValues(t, 1, alertCount(t, db))

	// 默认 5 分钟冷却内不重复触发。
	e.EvaluateAgent(ctx, "a1", report(95, 0, 0))
	assert.EqualValues(t, 1, alertCount(t, db))

	setRuleColumn(t, db, rule.ID, "last_fired_at", time.Now().UTC().Add(-6*time.Minute))
	time.Sleep(1100 * time.Millisecond)
	e.EvaluateAgent(ctx, "a1", report(95, 0, 0))
	assert.EqualValues(t, 2, alertCount(t, db))
}

// ---- forCount 连续命中语义 ----

func TestEvaluator_ForCountZeroTreatedAsOne(t *testing.T) {
	e, db := setup(t)
	rule := mustCreateRule(t, model.NewAlertRuleModel(db), &model.AlertRule{
		Name: "cpu", Metric: "cpu.usagePercent", Operator: "gt",
		Threshold: 90, ForCount: 1, Enabled: true,
	})
	setRuleColumn(t, db, rule.ID, "for_count", 0)

	e.EvaluateAgent(context.Background(), "a1", report(95, 0, 0))
	assert.EqualValues(t, 1, alertCount(t, db))
}

func TestEvaluator_MissResetsConsecutiveHits(t *testing.T) {
	e, db := setup(t)
	rule := mustCreateRule(t, model.NewAlertRuleModel(db), &model.AlertRule{
		Name: "cpu", Metric: "cpu.usagePercent", Operator: "gt",
		Threshold: 90, ForCount: 3, Enabled: true,
	})
	ctx := context.Background()

	e.EvaluateAgent(ctx, "a1", report(95, 0, 0)) // hit 1
	e.EvaluateAgent(ctx, "a1", report(50, 0, 0)) // miss → 清零
	e.EvaluateAgent(ctx, "a1", report(95, 0, 0)) // hit 1
	e.EvaluateAgent(ctx, "a1", report(95, 0, 0)) // hit 2
	assert.EqualValues(t, 0, alertCount(t, db), "中断后的连续命中不足 forCount 不应触发")

	var updated model.AlertRule
	require.NoError(t, db.First(&updated, rule.ID).Error)
	assert.Equal(t, 2, updated.HitCount)

	e.EvaluateAgent(ctx, "a1", report(95, 0, 0)) // hit 3 → 触发
	assert.EqualValues(t, 1, alertCount(t, db))
}

// ---- 触发产物 ----

func TestEvaluator_FiredAlertContent(t *testing.T) {
	e, db := setup(t)
	mustCreateRule(t, model.NewAlertRuleModel(db), &model.AlertRule{
		Name: "CPU爆表", Metric: "cpu.usagePercent", Operator: "gt",
		Threshold: 90, ForCount: 1, CooldownSeconds: 300,
		Level: model.AlertRuleLevelCritical, Enabled: true,
	})

	e.EvaluateAgent(context.Background(), "agent-9", report(95.5, 0, 0))

	var alerts []model.Alert
	require.NoError(t, db.Order("id DESC").Find(&alerts).Error)
	require.Len(t, alerts, 1)
	a := alerts[0]
	assert.Contains(t, a.AlertID, "rule-")
	assert.Equal(t, "metric.cpu.usagePercent", a.Type)
	assert.Equal(t, model.AlertRuleLevelCritical, a.Level)
	assert.Equal(t, "firing", a.Status)
	assert.Equal(t, "rule:CPU爆表", a.Source)
	assert.Equal(t, "alertrule", a.CreatedBy)
	assert.Contains(t, a.Message, "agent-9")
	assert.Contains(t, a.Message, "cpu.usagePercent > 90.00")
	assert.Contains(t, a.Message, "95.50")
	assert.Equal(t, json.Number("95.5"), a.Details["value"])
	assert.Equal(t, "agent-9", a.Details["agentId"])
}

// ---- 入口早退与容错 ----

func TestEvaluateAgent_EmptyAgentID(t *testing.T) {
	e, db := setup(t)
	mustCreateRule(t, model.NewAlertRuleModel(db), &model.AlertRule{
		Name: "cpu", Metric: "cpu.usagePercent", Operator: "gt",
		Threshold: 1, Enabled: true,
	})
	e.EvaluateAgent(context.Background(), "", report(99, 0, 0))
	assert.EqualValues(t, 0, alertCount(t, db))
}

func TestEvaluateAgent_RulesListFailureNoPanic(t *testing.T) {
	e, db := setup(t)
	mustCreateRule(t, model.NewAlertRuleModel(db), &model.AlertRule{
		Name: "cpu", Metric: "cpu.usagePercent", Operator: "gt",
		Threshold: 1, Enabled: true,
	})
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	assert.NotPanics(t, func() {
		e.EvaluateAgent(context.Background(), "a1", report(99, 0, 0))
	})
}

// ---- 指标提取边界 ----

func TestExtractMetric_MissingSections(t *testing.T) {
	cpuOnly := &opsv1.MetricsReport{Cpu: &opsv1.CpuMetrics{UsagePercent: 10}}
	_, ok := ExtractMetric("memory.usagePercent", cpuOnly)
	assert.False(t, ok, "无 memory 段")
	_, ok = ExtractMetric("memory.usedBytes", cpuOnly)
	assert.False(t, ok)

	_, ok = ExtractMetric("memory.usagePercent", &opsv1.MetricsReport{
		Memory: &opsv1.MemoryMetrics{UsedBytes: 10, TotalBytes: 0},
	})
	assert.False(t, ok, "total=0 无法计算百分比")

	r := report(0, 800, 1000)
	r.Disks = []*opsv1.DiskMetrics{
		{MountPoint: "/data.nvme0", UsedBytes: 900, TotalBytes: 1000, UsagePercent: 90},
	}
	v, ok := ExtractMetric("disk./data.nvme0.usedBytes", r)
	require.True(t, ok, "挂载点含点号时按最后一段拆字段")
	assert.InDelta(t, 900, v, 0.01)
}

func TestExtractMetricFromName_DiskVariants(t *testing.T) {
	for _, valid := range []string{"disk./a.b/c.usedBytes", "disk./data.usedPercent", "disk./var-log.usedBytes"} {
		_, err := ExtractMetricFromName(valid)
		assert.NoError(t, err, valid)
	}
	for _, bad := range []string{"disk.", "disk./data.", "disk..usedPercent", "disk.usedPercent", "disk/data.usedPercent"} {
		_, err := ExtractMetricFromName(bad)
		assert.Error(t, err, bad)
	}
}

func TestCompare_EqualBoundaries(t *testing.T) {
	assert.False(t, Compare(5, "lt", 5))
	assert.True(t, Compare(5, "lte", 5))
	assert.True(t, Compare(5, "gte", 5))
	assert.False(t, Compare(5, "gt", 5))
}

func TestDescribeOperatorAndLevelPriority(t *testing.T) {
	assert.Equal(t, ">", describeOperator("gt"))
	assert.Equal(t, ">=", describeOperator("gte"))
	assert.Equal(t, "<", describeOperator("lt"))
	assert.Equal(t, "<=", describeOperator("lte"))
	assert.Equal(t, "bogus", describeOperator("bogus"))

	assert.Equal(t, "urgent", levelToPriority(model.AlertRuleLevelCritical))
	assert.Equal(t, "high", levelToPriority(model.AlertRuleLevelWarning))
	assert.Equal(t, "normal", levelToPriority(model.AlertRuleLevelInfo))
	assert.Equal(t, "normal", levelToPriority("other"))
}
