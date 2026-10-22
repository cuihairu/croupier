package alertrule

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func probeReport(stats ...*opsv1.ProbeStats) *opsv1.MetricsReport {
	return &opsv1.MetricsReport{ProbeStats: stats}
}

func TestExtractMetricProbe(t *testing.T) {
	r := probeReport(
		&opsv1.ProbeStats{Target: "redis:6379", Checked: 100, Failed: 3, SuccessRate: 0.97},
		&opsv1.ProbeStats{Target: "db.primary", Checked: 50, Failed: 0, SuccessRate: 1},
	)

	v, ok := ExtractMetric("probe.redis:6379.successRate", r)
	require.True(t, ok)
	assert.InDelta(t, 0.97, v, 0.0001)

	v, ok = ExtractMetric("probe.db.primary.checked", r)
	require.True(t, ok)
	assert.InDelta(t, 50, v, 0.01)

	v, ok = ExtractMetric("probe.redis:6379.failed", r)
	require.True(t, ok)
	assert.InDelta(t, 3, v, 0.01)

	// 目标不匹配 → 缺失。
	_, ok = ExtractMetric("probe.missing.successRate", r)
	assert.False(t, ok)

	// checked=0（本窗口无样本）视为指标缺失，不把「没测」当「全成功」。
	r2 := probeReport(&opsv1.ProbeStats{Target: "redis:6379", Checked: 0, SuccessRate: 1})
	_, ok = ExtractMetric("probe.redis:6379.successRate", r2)
	assert.False(t, ok)
	// failed/checked 仍可读（0 是有效观测值）。
	v, ok = ExtractMetric("probe.redis:6379.checked", r2)
	require.True(t, ok)
	assert.InDelta(t, 0, v, 0.01)
}

func TestExtractMetricProbeLatencyP95(t *testing.T) {
	cases := []struct {
		name    string
		buckets []int64
		want    float64
		ok      bool
	}{
		{"全在首桶", []int64{100, 0, 0, 0, 0}, 50, true},
		{"95% 落在 200ms 桶", []int64{90, 5, 0, 0, 0}, 200, true},
		{"95% 落在 1000ms 桶", []int64{10, 10, 75, 0, 0}, 1000, true},
		{"95% 落在 5000ms 桶", []int64{10, 10, 10, 65, 0}, 5000, true},
		{"全在溢出桶取下界", []int64{0, 0, 0, 0, 100}, 5000, true},
		{"无样本", []int64{0, 0, 0, 0, 0}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := probeReport(&opsv1.ProbeStats{Target: "t", Checked: 100, LatencyBuckets: tc.buckets})
			v, ok := ExtractMetric("probe.t.latencyP95Ms", r)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				assert.InDelta(t, tc.want, v, 0.01)
			}
		})
	}
}

func TestExtractMetricFromNameProbe(t *testing.T) {
	for _, ok := range []string{
		"probe.redis:6379.successRate",
		"probe.db.primary.checked",
		"probe.t.failed",
		"probe.t.latencyP95Ms",
	} {
		_, err := ExtractMetricFromName(ok)
		assert.NoError(t, err, ok)
	}
	for _, bad := range []string{
		"probe..successRate", // 空目标
		"probe.t.bogus",      // 非法字段
		"probe.nodot",        // 无字段段
		"probe.",             // 空尾
	} {
		_, err := ExtractMetricFromName(bad)
		assert.Error(t, err, bad)
	}
}

func TestEvaluator_ProbeRuleFire(t *testing.T) {
	e, db := setup(t)
	rules := model.NewAlertRuleModel(db)
	require.NoError(t, rules.Create(context.Background(), &model.AlertRule{
		Name: "探测成功率低", Metric: "probe.redis:6379.successRate", Operator: "lt",
		Threshold: 0.99, Level: model.AlertRuleLevelWarning, Enabled: true,
	}))

	// 成功率 0.97 < 0.99 → 触发。
	e.EvaluateAgent(context.Background(), "agent-1",
		probeReport(&opsv1.ProbeStats{Target: "redis:6379", Checked: 100, Failed: 3, SuccessRate: 0.97}))
	var count int64
	require.NoError(t, db.Model(&model.Alert{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)

	var alert model.Alert
	require.NoError(t, db.First(&alert).Error)
	assert.Equal(t, "metric.probe.redis:6379.successRate", alert.Type)
	assert.Equal(t, "firing", alert.Status)

	// 冷却期内再命中 → 不重复触发。
	e.EvaluateAgent(context.Background(), "agent-1",
		probeReport(&opsv1.ProbeStats{Target: "redis:6379", Checked: 100, Failed: 5, SuccessRate: 0.95}))
	require.NoError(t, db.Model(&model.Alert{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)

	// 目标不匹配 → 指标缺失，不触发。
	e.EvaluateAgent(context.Background(), "agent-1",
		probeReport(&opsv1.ProbeStats{Target: "other", Checked: 100, Failed: 99, SuccessRate: 0.01}))
	require.NoError(t, db.Model(&model.Alert{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)

	// checked=0 → 指标缺失，不触发。
	e.EvaluateAgent(context.Background(), "agent-1",
		probeReport(&opsv1.ProbeStats{Target: "redis:6379", Checked: 0, SuccessRate: 1}))
	require.NoError(t, db.Model(&model.Alert{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
