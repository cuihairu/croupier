package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// DATA-026: the realtime series must use one bounded query for the whole
// window, bucket the rows in Go, fill empty buckets and never count a
// boundary event twice.
// ---------------------------------------------------------------------------

func sumSeriesPoints(t *testing.T, points []map[string]interface{}) int {
	t.Helper()
	total := 0
	for _, point := range points {
		value, ok := point["value"].(int64)
		require.True(t, ok, "series values are int64")
		total += int(value)
	}
	return total
}

func seedSeriesEvents(t *testing.T, db *gorm.DB, events []model.BehaviorEvent) {
	t.Helper()
	for i := range events {
		ev := events[i]
		require.NoError(t, db.Create(&ev).Error)
	}
}

// Every bucket is materialised, including empty ones, and the per-bucket
// counts cover events in the right slice.
func TestRealtimeSeriesBatchesEmptyBucketsAndCountsV26(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	seedSeriesEvents(t, db, []model.BehaviorEvent{
		createTestEvent("purchase", "u1", "game1", "prod", now.Add(-9*time.Minute), nil),
		createTestEvent("purchase", "u1", "game1", "prod", now.Add(-7*time.Minute), nil),
		createTestEvent("purchase", "u1", "game1", "prod", now.Add(-1*time.Minute), nil),
	})

	resp, err := realtimeSeries(context.Background(), svcCtx, &RealtimeSeriesRequest{
		GameId: "game1", Env: "prod", Interval: "1m", Duration: 10,
	})
	require.NoError(t, err)

	eventsSeries, ok := resp.Series["events"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, eventsSeries, 10, "empty buckets must still be returned")
	assert.Equal(t, 3, sumSeriesPoints(t, eventsSeries))

	usersSeries, ok := resp.Series["users"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, usersSeries, 10)
	assert.Equal(t, 0, sumSeriesPoints(t, usersSeries), "no liveness events in the window")
}

// A user emitting several heartbeats in one bucket counts once for the
// online series; ordinary events never mark a user online.
func TestRealtimeSeriesBatchesDedupesOnlinePerBucketV26(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	seedSeriesEvents(t, db, []model.BehaviorEvent{
		createTestEvent("heartbeat", "u1", "game1", "prod", now.Add(-5*time.Minute-50*time.Second), nil),
		createTestEvent("heartbeat", "u1", "game1", "prod", now.Add(-5*time.Minute-30*time.Second), nil),
		createTestEvent("heartbeat", "u1", "game1", "prod", now.Add(-5*time.Minute-10*time.Second), nil),
		createTestEvent("login", "u2", "game1", "prod", now.Add(-5*time.Minute), nil),
	})

	resp, err := realtimeSeries(context.Background(), svcCtx, &RealtimeSeriesRequest{
		GameId: "game1", Env: "prod", Interval: "1m", Duration: 6,
	})
	require.NoError(t, err)

	usersSeries, ok := resp.Series["users"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, usersSeries, 6)
	assert.Equal(t, 1, sumSeriesPoints(t, usersSeries), "one distinct online user in the whole window")
}

// An event landing exactly on a bucket boundary is counted once, not twice by
// the two overlapping range queries the old implementation used.
func TestRealtimeSeriesBatchesCountBoundaryEventsOnceV26(t *testing.T) {
	events := []model.RealtimeBucketRow{
		{OccurredAt: time.Unix(1000, 0), EventType: "purchase", UserID: "u1"},
		{OccurredAt: time.Unix(1060, 0), EventType: "purchase", UserID: "u1"},
		{OccurredAt: time.Unix(1120, 0), EventType: "purchase", UserID: "u1"},
	}
	start := time.Unix(1000, 0)
	end := time.Unix(1120, 0)

	eventsSeries, _ := bucketRealtimeRows(events, start, end, time.Minute)
	require.Len(t, eventsSeries, 2)
	assert.Equal(t, int64(1), eventsSeries[0].Value)
	assert.Equal(t, int64(2), eventsSeries[1].Value, "boundary row at end lands only in the final bucket")
}

// The DB round trip count does not grow with the number of buckets: the
// longest supported window at the finest interval issues one query, not 2880.
func TestRealtimeSeriesBatchesRoundtripsDecoupledFromBucketsV26(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	for i := 0; i < 12; i++ {
		ev := createTestEvent("purchase", "u1", "game1", "prod", now.Add(-time.Duration(i+1)*time.Minute), nil)
		require.NoError(t, svcCtx.BehaviorModel.RecordEvent(context.Background(), &ev))
	}

	queries := 0
	count := func(*gorm.DB) { queries++ }
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test/count_series_queries", count))
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register("test/count_series_rows", count))
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove("test/count_series_queries")
		_ = db.Callback().Row().Remove("test/count_series_rows")
	})

	counts := map[string]int{}
	for _, tc := range []struct {
		duration int
		interval string
	}{
		{duration: 60, interval: "1m"},
		{duration: 60, interval: "5m"},
		{duration: 1440, interval: "1m"},
		{duration: 1440, interval: "15m"},
	} {
		queries = 0
		_, err := realtimeSeries(context.Background(), svcCtx, &RealtimeSeriesRequest{
			GameId: "game1", Env: "prod", Interval: tc.interval, Duration: tc.duration,
		})
		require.NoError(t, err)
		counts[tc.interval+"/"+time.Duration(tc.duration).String()] = queries
	}

	for key, n := range counts {
		assert.Equal(t, 1, n, "%s should issue one window query, got %d", key, n)
	}
}

// Events older than the window or in another game/env are not mixed in.
func TestRealtimeSeriesBatchesScopedV26(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	seedSeriesEvents(t, db, []model.BehaviorEvent{
		createTestEvent("purchase", "u1", "game1", "prod", now.Add(-2*time.Minute), nil),
		createTestEvent("purchase", "u2", "game2", "prod", now.Add(-2*time.Minute), nil),
		createTestEvent("purchase", "u3", "game1", "stage", now.Add(-2*time.Minute), nil),
		createTestEvent("purchase", "u4", "game1", "prod", now.Add(-61*time.Minute), nil),
	})

	resp, err := realtimeSeries(context.Background(), svcCtx, &RealtimeSeriesRequest{
		GameId: "game1", Env: "prod", Interval: "1m", Duration: 60,
	})
	require.NoError(t, err)

	eventsSeries, ok := resp.Series["events"].([]map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 1, sumSeriesPoints(t, eventsSeries), "only the in-scope event is counted")
}

// A window larger than the row bound discloses a partial sample instead of
// presenting the truncated read as complete.
func TestRealtimeSeriesBatchesDisclosesPartialSampleV26(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	for i := 0; i < 30; i++ {
		ev := createTestEvent("purchase", "u1", "game1", "prod", now.Add(-time.Duration(i+1)*time.Minute), nil)
		require.NoError(t, svcCtx.BehaviorModel.RecordEvent(context.Background(), &ev))
	}

	prev := maxRealtimeBucketRows
	maxRealtimeBucketRows = 10
	defer func() { maxRealtimeBucketRows = prev }()

	resp, err := realtimeSeries(context.Background(), svcCtx, &RealtimeSeriesRequest{
		GameId: "game1", Env: "prod", Interval: "1m", Duration: 60,
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Sample)
	assert.Equal(t, true, resp.Sample["sampled"])
	assert.Equal(t, "partial", resp.Sample["coverage"])
	assert.Equal(t, 10, resp.Sample["scannedEvents"])
}
