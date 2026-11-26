package analytics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

// DATA-027: rolling WAU and pooled N-day retention read chain. Retention is
// computed from behavior activity pairs (the cohort table has no production
// writer) and every figure carries an explicit status so a real zero, a
// window without mature cohorts and an absent source stay distinguishable.

func TestComputeRetentionPoolsMatureCohortsV27(t *testing.T) {
	t.Parallel()

	// Date-only end expands to cover the whole 2026-03-20 day, so the
	// 2026-03-19 cohort with a 2026-03-20 return day is mature.
	end := time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC)
	rows := []model.RetentionPairRow{
		{Day: "2026-03-19", UserID: "u1"},
		{Day: "2026-03-19", UserID: "u2"},
		{Day: "2026-03-19", UserID: "u2"}, // duplicate pair collapses
		{Day: "2026-03-20", UserID: "u1"},
	}
	got := computeRetention(rows, end, 1, false)
	require.Equal(t, "ok", got.Status)
	require.NotNil(t, got.Value)
	require.InDelta(t, 50.0, *got.Value, 1e-9)
	require.Equal(t, 1, got.Cohorts)
	require.False(t, got.Sampled)
}

func TestComputeRetentionRealZeroIsNotMissingV27(t *testing.T) {
	t.Parallel()

	// Mature cohort day but nobody returned on day+N: 0 is a real zero, not
	// a missing value.
	end := time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC)
	rows := []model.RetentionPairRow{
		{Day: "2026-03-19", UserID: "u1"},
		{Day: "2026-03-19", UserID: "u2"},
		{Day: "2026-03-20", UserID: "u3"}, // u3 is new, not a return
	}
	got := computeRetention(rows, end, 1, false)
	require.Equal(t, "ok", got.Status)
	require.NotNil(t, got.Value)
	require.InDelta(t, 0.0, *got.Value, 1e-9)
}

func TestComputeRetentionPendingBeforeMaturityV27(t *testing.T) {
	t.Parallel()

	// A "now" end only yields fully elapsed return days: the 2026-03-19
	// cohort's D1 return day (2026-03-20) is not complete at noon, so no
	// cohort day qualifies even though activity exists.
	end := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	rows := []model.RetentionPairRow{
		{Day: "2026-03-19", UserID: "u1"},
		{Day: "2026-03-20", UserID: "u1"},
	}
	got := computeRetention(rows, end, 1, false)
	require.Equal(t, "pending", got.Status)
	require.Nil(t, got.Value)
}

func TestComputeRetentionNoSourceV27(t *testing.T) {
	t.Parallel()

	got := computeRetention(nil, time.Now(), 1, false)
	require.Equal(t, "no_source", got.Status)
	require.Nil(t, got.Value)
}

func TestComputeRetentionPoolsAcrossDaysV27(t *testing.T) {
	t.Parallel()

	end := time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC)
	rows := []model.RetentionPairRow{
		{Day: "2026-03-19", UserID: "u1"},
		{Day: "2026-03-19", UserID: "u2"},
		{Day: "2026-03-20", UserID: "u1"},
		{Day: "2026-03-20", UserID: "u3"},
		{Day: "2026-03-21", UserID: "u3"},
	}
	got := computeRetention(rows, end, 1, false)
	// Cohort 03-19: 1/2 retained (u1); cohort 03-20: 1/2 retained (u3);
	// cohort 03-21 is immature (return day 03-22 + 24h > end). Pooled 2/4.
	require.Equal(t, "ok", got.Status)
	require.NotNil(t, got.Value)
	require.InDelta(t, 50.0, *got.Value, 1e-9)
	require.Equal(t, 2, got.Cohorts)
}

func TestComputeRetentionSameRowsDifferentWindowsV27(t *testing.T) {
	t.Parallel()

	end := time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC)
	rows := []model.RetentionPairRow{
		{Day: "2026-03-19", UserID: "u1"},
		{Day: "2026-03-26", UserID: "u1"},
	}
	d1 := computeRetention(rows, end, 1, false)
	require.Equal(t, "ok", d1.Status)
	require.NotNil(t, d1.Value)
	require.InDelta(t, 0.0, *d1.Value, 1e-9) // nobody returned the next day
	d7 := computeRetention(rows, end, 7, false)
	require.Equal(t, "ok", d7.Status)
	require.NotNil(t, d7.Value)
	require.InDelta(t, 100.0, *d7.Value, 1e-9) // u1 returned exactly 7 days later
}

func TestComputeRetentionSampledFlagPassthroughV27(t *testing.T) {
	t.Parallel()

	rows := []model.RetentionPairRow{{Day: "2026-03-19", UserID: "u1"}}
	end := time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC)
	for _, sampled := range []bool{false, true} {
		got := computeRetention(rows, end, 1, sampled)
		require.Equal(t, sampled, got.Sampled)
	}
}

func TestShiftUTCDayV27(t *testing.T) {
	t.Parallel()

	day, ok := shiftUTCDay("2026-03-19", 7)
	require.True(t, ok)
	require.Equal(t, "2026-03-26", day.Format("2006-01-02"))

	_, ok = shiftUTCDay("garbage", 1)
	require.False(t, ok)
}

func TestOverviewReturnsWAUAndRetentionStatusesV27(t *testing.T) {
	db := setupServiceTestDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	ctx := context.Background()

	day := func(d int) time.Time { return time.Date(2026, 3, d, 10, 0, 0, 0, time.UTC) }
	seed := func(userID string, days ...int) {
		for _, d := range days {
			ev := createTestEventFull("login", userID, "game1", "prod", day(d), nil)
			require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &ev))
		}
	}
	// u1/u2 form a mature D1 cohort on 03-18 (u1 returned on 03-19); u3 is
	// only recent (03-25), inside the rolling 7-day WAU window.
	seed("u1", 18, 19)
	seed("u2", 18)
	seed("u3", 25)

	resp, err := overview(ctx, svcCtx, &OverviewRequest{
		GameId: "game1", Env: "prod", StartDate: "2026-03-10", EndDate: "2026-03-26",
	})
	require.NoError(t, err)

	// Rolling 7 days ending 2026-03-27 00:00 (start 2026-03-20): u3 only.
	require.Equal(t, 1, resp.Metrics.WAU)

	// D1 pools mature cohorts 03-18 {u1,u2} / 03-19 {u1} / 03-25 {u3}:
	// retained is u1 (03-18 -> 03-19) only -> 1/4 = 25%.
	d1 := resp.Metrics.RetentionD1
	require.NotNil(t, d1)
	require.Equal(t, "ok", d1.Status)
	require.NotNil(t, d1.Value)
	require.InDelta(t, 25.0, *d1.Value, 1e-9)
	require.Equal(t, 3, d1.Cohorts)

	// D7 mature cohorts (03-18/03-19) have no day+7 returns: a real zero.
	d7 := resp.Metrics.RetentionD7
	require.NotNil(t, d7)
	require.Equal(t, "ok", d7.Status)
	require.NotNil(t, d7.Value)
	require.InDelta(t, 0.0, *d7.Value, 1e-9)

	// D30 has no mature cohort day (oldest activity is 03-18): pending.
	d30 := resp.Metrics.RetentionD30
	require.NotNil(t, d30)
	require.Equal(t, "pending", d30.Status)
	require.Nil(t, d30.Value)

	// A game with no events at all: no_source, distinct from pending.
	respEmpty, err := overview(ctx, svcCtx, &OverviewRequest{
		GameId: "game-empty", Env: "prod", StartDate: "2026-03-10", EndDate: "2026-03-26",
	})
	require.NoError(t, err)
	require.Equal(t, 0, respEmpty.Metrics.WAU)
	for _, metric := range []*RetentionMetric{
		respEmpty.Metrics.RetentionD1, respEmpty.Metrics.RetentionD7, respEmpty.Metrics.RetentionD30,
	} {
		require.NotNil(t, metric)
		require.Equal(t, "no_source", metric.Status)
		require.Nil(t, metric.Value)
	}
}

func TestOverviewRetentionDisclosesRowCapSamplingV27(t *testing.T) {
	db := setupServiceTestDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	ctx := context.Background()

	origCap := maxRetentionPairRows
	maxRetentionPairRows = 2
	t.Cleanup(func() { maxRetentionPairRows = origCap })

	day := func(d int) time.Time { return time.Date(2026, 3, d, 10, 0, 0, 0, time.UTC) }
	for i, d := range []int{18, 19, 20} {
		ev := createTestEventFull("login", fmt.Sprintf("u%d", i), "game1", "prod", day(d), nil)
		require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &ev))
	}

	resp, err := overview(ctx, svcCtx, &OverviewRequest{
		GameId: "game1", Env: "prod", StartDate: "2026-03-10", EndDate: "2026-03-26",
	})
	require.NoError(t, err)
	for _, metric := range []*RetentionMetric{
		resp.Metrics.RetentionD1, resp.Metrics.RetentionD7, resp.Metrics.RetentionD30,
	} {
		require.NotNil(t, metric)
		require.True(t, metric.Sampled, "row-cap truncation must be disclosed")
	}
}
