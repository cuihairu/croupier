package registry

// probe_stats 持久化（评审简档 §13.1 统计面；迁移 0044 的模型侧配套）：
// persistToDB 落 ProbeJSON 列、getFromDB 重建 report.ProbeStats；
// 无探测统计的上报不落列（NULL 而非 "[]"）。

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

func TestMetricsStore_PersistsProbeStatsAndRebuildsFromDB(t *testing.T) {
	db := openMetricsTestDB(t)
	store := NewMetricsStore()
	store.SetDB(db)

	report := &opsv1.MetricsReport{
		Cpu: &opsv1.CpuMetrics{UsagePercent: 12.5},
		ProbeStats: []*opsv1.ProbeStats{{
			Target:      "game-db-primary",
			Semantics:   "readiness",
			GameId:      "demo",
			Env:         "prod",
			Checked:     30,
			Failed:      3,
			SuccessRate: 0.9,
			// 5 桶直方图口径（core/report.LatencyBucketBoundaries）。
			LatencyBuckets: []int64{10, 15, 3, 2, 0},
			LastReason:     "dial timeout",
		}},
	}
	store.Add("agent-probe-db", report)

	require.Eventually(t, func() bool {
		var count int64
		require.NoError(t, db.Model(&AgentMetricsHistory{}).Count(&count).Error)
		return count >= 1
	}, 5*time.Second, 10*time.Millisecond)

	// 清内存后走 DB 回读：probe_stats 完整重建（evaluator/ops 历史查询同口径）。
	store.Clear("agent-probe-db")
	history := store.GetHistory("agent-probe-db", time.Now().Add(-time.Minute), 10)
	require.Len(t, history, 1)
	probes := history[0].Report.GetProbeStats()
	require.Len(t, probes, 1)
	assert.Equal(t, "game-db-primary", probes[0].GetTarget())
	assert.Equal(t, "readiness", probes[0].GetSemantics())
	assert.Equal(t, int64(30), probes[0].GetChecked())
	assert.Equal(t, int64(3), probes[0].GetFailed())
	assert.InDelta(t, 0.9, probes[0].GetSuccessRate(), 0.001)
	assert.Equal(t, []int64{10, 15, 3, 2, 0}, probes[0].GetLatencyBuckets())
	assert.Equal(t, "dial timeout", probes[0].GetLastReason())
}

func TestMetricsStore_EmptyProbeStatsLeavesColumnNull(t *testing.T) {
	db := openMetricsTestDB(t)
	store := NewMetricsStore()
	store.SetDB(db)

	// 无探针面的 agent（capture/devops）：probe_stats 空 → 列 NULL（不落 "[]"）。
	store.Add("agent-noprobe", &opsv1.MetricsReport{
		Cpu: &opsv1.CpuMetrics{UsagePercent: 1},
	})

	require.Eventually(t, func() bool {
		var count int64
		require.NoError(t, db.Model(&AgentMetricsHistory{}).Count(&count).Error)
		return count >= 1
	}, 5*time.Second, 10*time.Millisecond)

	var nulls int64
	require.NoError(t, db.Raw(
		`SELECT count(*) FROM agent_metrics_history WHERE probe_json IS NULL`).Scan(&nulls).Error)
	require.EqualValues(t, 1, nulls, "empty probe_stats must persist as NULL, not []")

	store.Clear("agent-noprobe")
	history := store.GetHistory("agent-noprobe", time.Now().Add(-time.Minute), 10)
	require.Len(t, history, 1)
	assert.Empty(t, history[0].Report.GetProbeStats())
}
