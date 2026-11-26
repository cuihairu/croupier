package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

// DATA-028: the daily peak is the interval maximum of the same online series
// the realtime screen plots (liveness users per bucket), bucketed in the
// declared UTC business day boundary, with granularity/coverage meta and an
// explicit missing-day state (a real zero day is never conflated with a day
// that has no observations).

func TestPeakOnlineCrossDaySampleV28(t *testing.T) {
	t.Parallel()

	at := func(day int, hh, mm, ss time.Duration) time.Time {
		return time.Date(2026, 3, day, 0, 0, 0, 0, time.UTC).Add(hh*time.Hour + mm*time.Minute + ss*time.Second)
	}
	rows := []model.RealtimeBucketRow{
		// 03-10: three users share one minute, u1 returns alone later.
		{OccurredAt: at(10, 10, 0, 30), UserID: "u1", EventType: "heartbeat"},
		{OccurredAt: at(10, 10, 0, 40), UserID: "u2", EventType: "heartbeat"},
		{OccurredAt: at(10, 10, 0, 50), UserID: "u3", EventType: "session_start"},
		{OccurredAt: at(10, 11, 5, 0), UserID: "u1", EventType: "heartbeat"},
		// 03-11: business events only — the day is observed, peak is a real zero.
		{OccurredAt: at(11, 12, 0, 0), UserID: "u1", EventType: "login"},
		// 03-12: no rows at all — missing, not a zero.
		// 03-13: boundary row exactly on the minute edge plus a two-user minute.
		{OccurredAt: at(13, 0, 0, 59), UserID: "u1", EventType: "heartbeat"},
		{OccurredAt: at(13, 0, 1, 30), UserID: "u1", EventType: "heartbeat"},
		{OccurredAt: at(13, 0, 1, 40), UserID: "u2", EventType: "heartbeat"},
	}

	start := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)

	report := peakOnlineFromRows(rows, start, end, false)

	require.Equal(t, []PeakOnlinePoint{
		{Date: "2026-03-10", Value: 3},
		{Date: "2026-03-11", Value: 0}, // real zero: observed, no liveness
		{Date: "2026-03-13", Value: 2},
	}, report.Points, "03-12 has no rows and must stay missing, not zero")

	meta := report.Meta
	require.Equal(t, "UTC", meta.Timezone)
	require.Equal(t, 60, meta.GranularitySeconds)
	require.Equal(t, 4, meta.TotalDays)
	require.Equal(t, 3, meta.CoveredDays)
	require.Equal(t, "2026-03-10", meta.CoveredFrom)
	require.Equal(t, "2026-03-13", meta.CoveredTo)
	require.False(t, meta.Sampled)
}

func TestPeakOnlineBucketBoundaryV28(t *testing.T) {
	t.Parallel()

	// An event exactly on the bucket edge belongs to the later bucket
	// (half-open [cursor, cursor+interval)), matching the realtime series.
	rows := []model.RealtimeBucketRow{
		{OccurredAt: time.Date(2026, 3, 10, 10, 0, 59, 0, time.UTC), UserID: "u1", EventType: "heartbeat"},
		{OccurredAt: time.Date(2026, 3, 10, 10, 1, 0, 0, time.UTC), UserID: "u2", EventType: "heartbeat"},
	}
	report := peakOnlineFromRows(rows,
		time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC),
		false)
	require.Len(t, report.Points, 1)
	require.Equal(t, 1, report.Points[0].Value, "u2 must land in the 10:01 bucket, not merge into 10:00")
}

func TestPeakOnlineSampledFlagPassthroughV28(t *testing.T) {
	rows := []model.RealtimeBucketRow{
		{OccurredAt: time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC), UserID: "u1", EventType: "heartbeat"},
	}
	report := peakOnlineFromRows(rows,
		time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC),
		true)
	require.True(t, report.Meta.Sampled)

	db := setupServiceTestDB(t)
	origCap := maxPeakOnlineRows
	maxPeakOnlineRows = 2
	t.Cleanup(func() { maxPeakOnlineRows = origCap })

	svcCtx := setupFullServiceTestContext(t, db)
	ctx := context.Background()
	day := time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
	for i, user := range []string{"u1", "u2", "u3"} {
		ev := createTestEventFull("heartbeat", user, "game1", "prod", day.Add(time.Duration(i)*time.Minute), nil)
		require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &ev))
	}
	report2, err := dailyPeakOnline(ctx, svcCtx.BehaviorModel, "game1", "prod",
		time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.True(t, report2.Meta.Sampled, "row-cap truncation must be disclosed")
	require.Len(t, report2.Points, 1)
}

func TestOverviewWiresPeakOnlineV28(t *testing.T) {
	db := setupServiceTestDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	ctx := context.Background()

	at := func(day, hour int) time.Time {
		return time.Date(2026, 3, day, hour, 0, 30, 0, time.UTC)
	}
	seed := func(eventType, userID string, at time.Time) {
		ev := createTestEventFull(eventType, userID, "game1", "prod", at, nil)
		require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &ev))
	}
	// 03-10: three online users at 10:00; 03-11: business events only;
	// 03-12: unobserved; 03-13: two online users.
	seed("heartbeat", "u1", at(10, 10))
	seed("heartbeat", "u2", at(10, 10))
	seed("session_start", "u3", at(10, 10))
	seed("login", "u1", at(11, 12))
	seed("heartbeat", "u1", at(13, 9))
	seed("heartbeat", "u2", at(13, 9))

	resp, err := overview(ctx, svcCtx, &OverviewRequest{
		GameId: "game1", Env: "prod", StartDate: "2026-03-10", EndDate: "2026-03-13",
	})
	require.NoError(t, err)

	peak := resp.PeakOnline
	require.NotNil(t, peak, "overview must expose the peak online report")
	require.Equal(t, []PeakOnlinePoint{
		{Date: "2026-03-10", Value: 3},
		{Date: "2026-03-11", Value: 0},
		{Date: "2026-03-13", Value: 2},
	}, peak.Points)
	require.Equal(t, 4, peak.Meta.TotalDays)
	require.Equal(t, 3, peak.Meta.CoveredDays)
	require.Equal(t, "UTC", peak.Meta.Timezone)
}
