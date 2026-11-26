package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// DATA-025: online users, active sessions and event throughput must use
// separate authoritative signals instead of one event count wearing three
// different labels. Online is derived from heartbeat/session liveness events
// (distinct users), sessions from session_start events, throughput from all
// behavior events.
// ---------------------------------------------------------------------------

func realtimeSeed(t *testing.T, svcCtx *svc.ServiceContext, events []model.BehaviorEvent) {
	t.Helper()
	for i := range events {
		ev := events[i]
		require.NoError(t, svcCtx.BehaviorModel.RecordEvent(context.Background(), &ev))
	}
}

func TestRealtimeSeparatesOnlineSessionsAndThroughputV25(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	// One user emits three heartbeats; another declares a session via the
	// dotted OTel alias. Online = 2 distinct users, not 4 events.
	// Two session_start events (one canonical, one dotted) = 2 sessions.
	// Six purchase events add throughput but neither online nor sessions.
	events := []model.BehaviorEvent{
		createTestEvent("heartbeat", "u1", "game1", "prod", now.Add(-10*time.Minute), nil),
		createTestEvent("heartbeat", "u1", "game1", "prod", now.Add(-5*time.Minute), nil),
		createTestEvent("heartbeat", "u1", "game1", "prod", now.Add(-1*time.Minute), nil),
		createTestEvent("session.start", "u2", "game1", "prod", now.Add(-4*time.Minute), nil),
		createTestEvent("session_start", "u3", "game1", "prod", now.Add(-3*time.Minute), nil),
	}
	for i := 0; i < 6; i++ {
		events = append(events, createTestEvent("purchase", "buyer", "game1", "prod", now.Add(-2*time.Minute), nil))
	}
	realtimeSeed(t, svcCtx, events)

	resp, err := realtime(context.Background(), svcCtx, &RealtimeRequest{GameId: "game1", Env: "prod"})
	require.NoError(t, err)

	m := resp.RealtimeMetrics
	assert.Equal(t, 3, m.OnlineUsers, "heartbeats from one user count once; session users count too")
	assert.Equal(t, 2, m.ActiveSessions, "one start = one session; purchases are not sessions")
	assert.InDelta(t, 11.0/15.0, m.EventsPerMinute, 0.0001, "all 11 events over the 15-minute window")
	assert.InDelta(t, 11.0/900.0, m.QPS, 0.0001)

	// The response must say where each figure came from and window length.
	assert.Contains(t, resp.Source.OnlineUsers, "heartbeat")
	assert.Contains(t, resp.Source.OnlineUsers, "not concurrent sockets")
	assert.Contains(t, resp.Source.ActiveSessions, "session_start")
	assert.Contains(t, resp.Source.EventsPerMinute, "all behavior events")
	assert.Equal(t, 900, resp.Source.WindowSeconds)
}

// Ordinary activity alone no longer creates an online user or a session.
func TestRealtimeIgnoresBusinessEventsForOnlineAndSessionsV25(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	events := []model.BehaviorEvent{
		createTestEvent("purchase", "u1", "game1", "prod", now.Add(-1*time.Minute), nil),
		createTestEvent("purchase", "u1", "game1", "prod", now.Add(-2*time.Minute), nil),
		createTestEvent("level_complete", "u1", "game1", "prod", now.Add(-3*time.Minute), nil),
		createTestEvent("login", "u1", "game1", "prod", now.Add(-4*time.Minute), nil),
	}
	realtimeSeed(t, svcCtx, events)

	resp, err := realtime(context.Background(), svcCtx, &RealtimeRequest{GameId: "game1", Env: "prod"})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.RealtimeMetrics.OnlineUsers, "login alone is not a liveness signal")
	assert.Equal(t, 0, resp.RealtimeMetrics.ActiveSessions, "no session_start events")
	assert.InDelta(t, 4.0/15.0, resp.RealtimeMetrics.EventsPerMinute, 0.0001)
}

// Long inactivity: events outside the realtime window cannot keep a user online.
func TestRealtimeStaleHeartbeatFallsOutOfWindowV25(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	events := []model.BehaviorEvent{
		createTestEvent("heartbeat", "stale-user", "game1", "prod", now.Add(-2*time.Hour), nil),
		createTestEvent("heartbeat", "live-user", "game1", "prod", now.Add(-5*time.Minute), nil),
	}
	realtimeSeed(t, svcCtx, events)

	resp, err := realtime(context.Background(), svcCtx, &RealtimeRequest{GameId: "game1", Env: "prod"})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.RealtimeMetrics.OnlineUsers, "stale heartbeat must not count as online")
	assert.Equal(t, 0, resp.RealtimeMetrics.ActiveSessions)
}

// Multiple devices: distinct users, not distinct device rows, are counted.
func TestRealtimeMultipleDevicesOneUserV25(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	events := []model.BehaviorEvent{
		createTestEvent("heartbeat", "same-user", "game1", "prod", now.Add(-5*time.Minute), map[string]interface{}{"device": "phone"}),
		createTestEvent("heartbeat", "same-user", "game1", "prod", now.Add(-4*time.Minute), map[string]interface{}{"device": "desktop"}),
		createTestEvent("session_start", "same-user", "game1", "prod", now.Add(-8*time.Minute), map[string]interface{}{"device": "console"}),
	}
	realtimeSeed(t, svcCtx, events)

	resp, err := realtime(context.Background(), svcCtx, &RealtimeRequest{GameId: "game1", Env: "prod"})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.RealtimeMetrics.OnlineUsers, "one user across three devices counts once")
	assert.Equal(t, 1, resp.RealtimeMetrics.ActiveSessions)
}

// The online series uses the same liveness source and is not inflated by
// business-event traffic.
func TestRealtimeSeriesOnlineUsesLivenessOnlyV25(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	now := time.Now().UTC()

	events := []model.BehaviorEvent{
		createTestEvent("heartbeat", "u1", "game1", "prod", now.Add(-30*time.Second), nil),
		createTestEvent("purchase", "u2", "game1", "prod", now.Add(-30*time.Second), nil),
	}
	realtimeSeed(t, svcCtx, events)

	resp, err := realtimeSeries(context.Background(), svcCtx, &RealtimeSeriesRequest{
		GameId: "game1", Env: "prod", Interval: "1m", Duration: 5,
	})
	require.NoError(t, err)

	series, ok := resp.Series["users"].([]map[string]interface{})
	require.True(t, ok)
	onlineTotal := 0
	for _, point := range series {
		if v, ok := point["value"].(int64); ok {
			onlineTotal += int(v)
		}
	}
	assert.Equal(t, 1, onlineTotal, "only the heartbeat user appears in the online series")

	eventsSeries, ok := resp.Series["events"].([]map[string]interface{})
	require.True(t, ok)
	eventsTotal := 0
	for _, point := range eventsSeries {
		if v, ok := point["value"].(int64); ok {
			eventsTotal += int(v)
		}
	}
	assert.Equal(t, 2, eventsTotal, "throughput still counts all events")
}
