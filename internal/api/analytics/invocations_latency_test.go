package analytics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nearestRankIndex 表格：验收样本量 1、2、20、21 与整百；n=0 钳到 0。
func TestNearestRankIndex(t *testing.T) {
	cases := []struct {
		n    int
		want int
	}{
		{0, 0},
		{1, 0},
		{2, 1},   // 旧 float int(n*0.95)-1 给 0（最小值），nearest-rank 给最大值
		{3, 2},   // ceil(2.85)=3
		{20, 18}, // ceil(19)=19 → 19th
		{21, 19}, // ceil(19.95)=20
		{100, 94},
		{101, 95},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, nearestRankIndex(c.n, 95, 100), "n=%d", c.n)
	}
}

// 验收：P95 在 1、2、20、21 条样本下取精确近秩位——小样本不再偏低。
func TestInvocationsSummary_P95SmallSamples(t *testing.T) {
	// n=1：唯一样本即 p95。
	db := setupInvocationsTestDB(t)
	seedInvocation(t, db, "g", "prod", "fn", "success", 42)
	resp, err := invocationsSummary(context.Background(), setupInvocationsSvcCtx(db), &InvocationsSummaryRequest{GameId: "g", Hours: 24})
	require.NoError(t, err)
	assert.InDelta(t, 42.0, resp.P95DurationMs, 0.001)

	// n=2：nearest-rank 取第 2 条（较大值）；旧算法取第 1 条（偏低）。
	db2 := setupInvocationsTestDB(t)
	seedInvocation(t, db2, "g", "prod", "fn", "success", 10)
	seedInvocation(t, db2, "g", "prod", "fn", "success", 20)
	resp2, err := invocationsSummary(context.Background(), setupInvocationsSvcCtx(db2), &InvocationsSummaryRequest{GameId: "g", Hours: 24})
	require.NoError(t, err)
	assert.InDelta(t, 20.0, resp2.P95DurationMs, 0.001, "n=2 must rank to the 2nd observation, not the minimum")

	// n=20 与既有百分位夹具同分布（10..190），p95 = 190。
	db3 := setupInvocationsTestDB(t)
	for i := 1; i <= 20; i++ {
		seedInvocation(t, db3, "g", "prod", "fn", "success", int64(i*10))
	}
	resp3, err := invocationsSummary(context.Background(), setupInvocationsSvcCtx(db3), &InvocationsSummaryRequest{GameId: "g", Hours: 24})
	require.NoError(t, err)
	assert.InDelta(t, 190.0, resp3.P95DurationMs, 0.001)

	// n=21：ceil(19.95)=20 → 第 20 条 = 200。
	db4 := setupInvocationsTestDB(t)
	for i := 1; i <= 21; i++ {
		seedInvocation(t, db4, "g", "prod", "fn", "success", int64(i*10))
	}
	resp4, err := invocationsSummary(context.Background(), setupInvocationsSvcCtx(db4), &InvocationsSummaryRequest{GameId: "g", Hours: 24})
	require.NoError(t, err)
	assert.InDelta(t, 200.0, resp4.P95DurationMs, 0.001)
}

// 口径披露：method 字符串、缺失样本计数（绝不插补）、未触上限时精确读数。
func TestInvocationsSummary_LatencyMethodAndMissingSamples(t *testing.T) {
	db := setupInvocationsTestDB(t)
	for i := 1; i <= 20; i++ {
		seedInvocation(t, db, "g", "prod", "fn", "success", int64(i*10))
	}
	seedInvocation(t, db, "g", "prod", "fn", "success", 0) // 无时长 → 计缺失

	resp, err := invocationsSummary(context.Background(), setupInvocationsSvcCtx(db), &InvocationsSummaryRequest{GameId: "g", Hours: 24})
	require.NoError(t, err)
	require.NotNil(t, resp.Latency)
	assert.Contains(t, resp.Latency.Method, "nearest-rank")
	sample := resp.Latency.Sample
	assert.Equal(t, int64(20), sample.TotalEvents)
	assert.Equal(t, int64(20), sample.ScannedEvents)
	assert.False(t, sample.Sampled)
	assert.InDelta(t, 1.0, sample.Coverage, 0.001)
	require.NotNil(t, sample.MissingDuration)
	assert.Equal(t, int64(1), *sample.MissingDuration, "missing durations are counted, never imputed")
}

// 资源上限：超上限扫描最新 cap 条并披露 sampled/coverage；avg/p95 同口径取自被扫描子集。
func TestInvocationsSummary_LatencyCappedScan(t *testing.T) {
	orig := invocationLatencyRowCap
	invocationLatencyRowCap = 5
	t.Cleanup(func() { invocationLatencyRowCap = orig })

	db := setupInvocationsTestDB(t)
	for i := 1; i <= 20; i++ {
		seedInvocation(t, db, "g", "prod", "fn", "success", int64(i*10))
	}

	resp, err := invocationsSummary(context.Background(), setupInvocationsSvcCtx(db), &InvocationsSummaryRequest{GameId: "g", Hours: 24})
	require.NoError(t, err)
	require.NotNil(t, resp.Latency)
	sample := resp.Latency.Sample
	assert.Equal(t, int64(20), sample.TotalEvents)
	assert.Equal(t, int64(5), sample.ScannedEvents)
	assert.True(t, sample.Sampled)
	assert.InDelta(t, 0.25, sample.Coverage, 0.001)
	// 最新 5 条 = 160..200（timestamp DESC, id DESC 确定序）：p95 = 200，avg = 180。
	assert.InDelta(t, 200.0, resp.P95DurationMs, 0.001)
	assert.InDelta(t, 180.0, resp.AvgDurationMs, 0.001)
}

// 大体量样本：趋势扫描越限保留最新前缀（旧实现 Order ASC 静默丢最新）并披露采样。
func TestInvocationsTrend_CappedScanKeepsRecent(t *testing.T) {
	orig := invocationTrendRowCap
	invocationTrendRowCap = 3
	t.Cleanup(func() { invocationTrendRowCap = orig })

	db := setupInvocationsTestDB(t)
	// 5 个互异 UTC 日，各 1 行；cap=3 → 只扫最新 3 天。
	now := time.Now().UTC()
	for d := 5; d >= 1; d-- {
		ts := now.AddDate(0, 0, -d)
		require.NoError(t, db.Exec(
			"INSERT INTO audit_records (audit_id, timestamp, event_type, category, severity, outcome, chain_hash, chain_sequence, created_at) VALUES (?,?,?,?,?,?,?,?,?)",
			fmt.Sprintf("trend-%d", d), ts, "function.invoke", "operational", "info", "success", "h", 900000+d, ts,
		).Error)
	}

	resp, err := invocationsTrend(context.Background(), setupInvocationsSvcCtx(db), &InvocationsTrendRequest{Interval: "day"})
	require.NoError(t, err)
	require.NotNil(t, resp.Sample)
	assert.Equal(t, int64(5), resp.Sample.TotalEvents)
	assert.Equal(t, int64(3), resp.Sample.ScannedEvents)
	assert.True(t, resp.Sample.Sampled)
	assert.InDelta(t, 0.6, resp.Sample.Coverage, 0.001)

	// 时间顺序的 3 个桶 = 最新的 3 天（now-3d、now-2d、now-1d），各 1 条。
	require.Len(t, resp.Points, 3)
	want := []string{
		now.AddDate(0, 0, -3).Format("2006-01-02"),
		now.AddDate(0, 0, -2).Format("2006-01-02"),
		now.AddDate(0, 0, -1).Format("2006-01-02"),
	}
	for i, p := range resp.Points {
		assert.Equal(t, want[i], p.Bucket, "bucket %d must be the newest days, chronological", i)
		assert.Equal(t, int64(1), p.Total)
	}
}
