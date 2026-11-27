package worker

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// DATA-054: raw detail rows and aggregation buckets must agree on one event
// time. An old epoch-ts event used to produce a correct historical raw row
// (normalizeTimestamp accepted epochs) while touchAgg's RFC3339-only parse
// threw its DAU/online buckets at the processing-time day/minute — a bucket
// that drifts on every replay. The probe feeds one 72h-old event through
// both paths and requires the event's own day/minute on both sides.
func TestProbe_DATA054_SameEpochEventRawRowAndAgreeingBuckets(t *testing.T) {
	epoch := time.Now().UTC().Add(-72 * time.Hour).Truncate(time.Second)
	tsStr := strconv.FormatInt(epoch.Unix(), 10) // epoch seconds, the quick-start wire shape
	day := epoch.Format("2006-01-02")
	today := time.Now().UTC().Format("2006-01-02")
	minute := epoch.Truncate(time.Minute).Format("200601021504")

	// Raw insert half: eventRowArgs is the single derivation point for both
	// fresh and DATA-053 held rows.
	rawTs, ok := eventRowArgs(map[string]any{
		"ts": tsStr, "game_id": "g", "env": "prod", "user_id": "u1",
		"event": "login",
	})[0].(string)
	require.True(t, ok)
	require.Equal(t, epoch.Format("2006-01-02 15:04:05"), rawTs)

	// Aggregation half: the same payload must touch the event's own
	// day/minute buckets, never the processing-time ones.
	w, _ := testWorker(t)
	ctx := context.Background()
	w.touchAgg(ctx, map[string]any{
		"ts": tsStr, "game_id": "g", "env": "prod", "user_id": "u1",
		"event": "heartbeat",
	})
	require.Contains(t, w.touchedMinutes, "hll:online:g:prod:"+minute,
		"heartbeat minute bucket must carry the event's minute")
	require.NotContains(t, w.touchedMinutes, "hll:online:g:prod:"+time.Now().UTC().Truncate(time.Minute).Format("200601021504"),
		"processing-time minute bucket must not be touched")

	w.touchAgg(ctx, map[string]any{
		"ts": tsStr, "game_id": "g", "env": "prod", "user_id": "u1",
		"event": "login",
	})
	require.Contains(t, w.touchedDays, day+"|g|prod",
		"DAU bucket must carry the event's day")
	require.NotContains(t, w.touchedDays, today+"|g|prod",
		"processing-time day bucket must not be touched (72h-old event)")

	// Millisecond epoch: same agreement on the raw side.
	msStr := strconv.FormatInt(epoch.UnixMilli(), 10)
	rawTsMs, _ := eventRowArgs(map[string]any{"ts": msStr, "game_id": "g"})[0].(string)
	require.Equal(t, epoch.Format("2006-01-02 15:04:05"), rawTsMs)

	// Business-timezone producer (offset-aware RFC3339) normalizes to the
	// same UTC instant on both sides.
	offset := epoch.In(time.FixedZone("biz", 8*3600)).Format(time.RFC3339)
	rawTz, _ := eventRowArgs(map[string]any{"ts": offset, "game_id": "g"})[0].(string)
	require.Equal(t, epoch.Format("2006-01-02 15:04:05"), rawTz)
	w.touchAgg(ctx, map[string]any{
		"ts": offset, "game_id": "g", "env": "prod", "user_id": "u1",
		"event": "session_start",
	})
	require.Contains(t, w.touchedDays, day+"|g|prod",
		"offset-aware RFC3339 must land in the same UTC day bucket")
}

// DATA-054 explicit invalid disposition: empty/unparsable ts reaches neither
// path — the raw insert passes the value through (ClickHouse Append errors
// into the dead letter) and the aggregation skips the event, so no phantom
// bucket member without an underlying raw row can exist.
func TestProbe_DATA054_InvalidTsReachesNeitherPath(t *testing.T) {
	for _, bad := range []string{"", "bad-time", "0", "-100"} {
		got := normalizeTimestamp(bad)
		require.NotEqual(t, got, time.Now().UTC().Format("2006-01-02 15:04:05"),
			"invalid ts %q must not be stamped with processing time", bad)

		w, _ := testWorker(t)
		w.touchAgg(context.Background(), map[string]any{
			"ts": bad, "game_id": "g", "env": "prod", "user_id": "u1",
			"event": "login",
		})
		require.Empty(t, w.touchedDays, "invalid ts %q must not touch day buckets", bad)
		require.Empty(t, w.touchedMinutes, "invalid ts %q must not touch minute buckets", bad)
	}
}

func TestParseEventTime(t *testing.T) {
	want := time.Date(2026, 8, 24, 3, 2, 55, 0, time.UTC)
	cases := []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"2026-08-24T03:02:55Z", want, true},
		{"2026-08-24T11:02:55+08:00", want, true}, // offset normalized to UTC
		{"2026-08-24 03:02:55", want, true},       // ClickHouse datetime layout
		{"1787540575", time.Unix(1787540575, 0).UTC(), true},
		{"1787540575000", time.Unix(1787540575, 0).UTC(), true}, // milliseconds
		{"", time.Time{}, false},
		{"bad-time", time.Time{}, false},
		{"0", time.Time{}, false},    // secs > 0 required
		{"-100", time.Time{}, false}, // pre-epoch rejected
	}
	for _, tc := range cases {
		got, ok := parseEventTime(tc.in)
		if ok != tc.ok {
			t.Errorf("parseEventTime(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			continue
		}
		if ok && !got.Equal(tc.want) {
			t.Errorf("parseEventTime(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// Drill against a disposable ClickHouse: the same 72h-old epoch event lands
// a historical raw row (event_time == the epoch instant) AND the event-day
// bucket, i.e. the wire-level agreement the unit probe asserts is preserved
// through a real PrepareBatch/Append/Send.
func TestDrill_DATA054_EpochEventRawRowAndBucket(t *testing.T) {
	dsn := os.Getenv("DATA054_CLICKHOUSE_DSN")
	if dsn == "" {
		t.Skip("DATA-054: DATA054_CLICKHOUSE_DSN not set (needs a disposable ClickHouse)")
	}
	// The production worker runs in a TZ=UTC container; clickhouse-go parses
	// appended DateTime strings in the client location (probed: a +08 client
	// stores a -8h shifted wall clock), so the drill pins the process to UTC
	// to mirror the production posture on a developer box. Package tests run
	// sequentially (no t.Parallel), the restore makes the pin leak-free.
	prevLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = prevLocal })
	opts, err := clickhouse.ParseDSN(dsn)
	require.NoError(t, err)
	conn, err := clickhouse.Open(opts)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, conn.Ping(ctx))

	// insertEventsSQL hardcodes analytics.events; the DSN must point at a
	// disposable server (local throwaway container), never a shared stack.
	require.NoError(t, conn.Exec(ctx, "CREATE DATABASE IF NOT EXISTS analytics"))
	require.NoError(t, conn.Exec(ctx, "DROP TABLE IF EXISTS analytics.events"))
	require.NoError(t, conn.Exec(ctx, `CREATE TABLE analytics.events (
		event_time   DateTime DEFAULT now(),
		game_id      LowCardinality(String),
		env          LowCardinality(String),
		user_id      String,
		session_id   String,
		event        LowCardinality(String),
		channel      LowCardinality(String),
		platform     LowCardinality(String),
		country      FixedString(2),
		app_version  String,
		event_id     UUID,
		props_json   String
	) ENGINE = MergeTree
	PARTITION BY toYYYYMM(event_time)
	ORDER BY (game_id, env, event, user_id, event_time)`))
	t.Cleanup(func() { _ = conn.Exec(context.Background(), "DROP DATABASE analytics") })

	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	w := &Worker{
		rdb:                rdb,
		ch:                 conn,
		streamEvents:       "analytics:events",
		streamPayments:     "analytics:payments",
		group:              "analytics-worker",
		consumer:           "drill-consumer",
		touchedMinutes:     map[string]struct{}{},
		touchedDays:        map[string]struct{}{},
		revAgg:             map[string]*revRow{},
		clickBatchSize:     1, // Send immediately so the probe row lands
		checkpointPrefix:   "analytics:checkpoint",
		lastIDs:            map[string]string{},
		deadEventsStream:   "analytics:events:dead",
		deadPaymentsStream: "analytics:payments:dead",
		aggDirtySet:        "analytics:agg:dirty",
		aggRebuildSet:      "analytics:agg:rebuild",
		aggRdb:             rdb,
		revProgressSet:     "analytics:rev:progress",
		revProgressSeenTTL: revProgressSeenDefaultTTL,
	}

	epoch := time.Now().UTC().Add(-72 * time.Hour).Truncate(time.Second)
	payload := map[string]any{
		"ts":      strconv.FormatInt(epoch.Unix(), 10),
		"game_id": "g", "env": "prod", "user_id": "u1", "session_id": "s1",
		"event": "login", "country": "CN",
	}
	w.touchAgg(ctx, payload)
	require.NoError(t, w.insertEvent(ctx, payload, "1-1"))

	var got string
	require.NoError(t, conn.QueryRow(ctx,
		"SELECT toString(event_time) FROM analytics.events LIMIT 1").Scan(&got))
	require.Equal(t, epoch.Format("2006-01-02 15:04:05"), got,
		"raw row must carry the epoch event's historical event_time (UTC wall clock)")

	day := epoch.Format("2006-01-02")
	require.Contains(t, w.touchedDays, day+"|g|prod",
		"DAU bucket must carry the event's day")
	require.NotContains(t, w.touchedDays, time.Now().UTC().Format("2006-01-02")+"|g|prod",
		"processing-time day bucket must not be touched")
}
