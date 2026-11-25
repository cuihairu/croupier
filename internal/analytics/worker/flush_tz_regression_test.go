package worker

import (
	"context"
	"testing"
	"time"
)

// Timezone-independent regression: when an event ts carries a non-UTC offset (+09:30),
// minute-online bucketing must still normalize to the correct UTC minute and be flushed.
// If the write side rendered the hll key using the event's own offset (or a local-timezone
// fallback), the flush side's time.Parse (always UTC) would be off from the key by one
// timezone offset, t.Before(nowMin) would never hold, and minute_online would never land
// in the database on non-UTC deployments (same family as the 993cb65 audit time-window
// issue; this case must pass under any machine timezone).
func TestTouchAggFlushOffsetIndependentMinute(t *testing.T) {
	w, _ := testWorker(t)
	zone := time.FixedZone("FIXED", 9*60*60+30*60)
	now := time.Now().UTC().Truncate(time.Minute).In(zone)

	w.touchAgg(context.Background(), map[string]any{
		"ts":      now.Add(-2 * time.Minute).Format(time.RFC3339),
		"game_id": "g1",
		"env":     "prod",
		"event":   "heartbeat",
		"user_id": "u1",
	})
	if len(w.touchedMinutes) != 1 {
		t.Fatalf("touchedMinutes = %d, want 1", len(w.touchedMinutes))
	}

	if err := w.flush(context.Background()); err != nil {
		t.Fatalf("flush = %v", err)
	}
	if len(w.touchedMinutes) != 0 {
		t.Fatalf("minute key not flushed; touchedMinutes=%v (write side not normalized to UTC?)", w.touchedMinutes)
	}
	mc := getMockConn(t, w)
	if mc.batch == nil || mc.batch.Rows() != 1 {
		t.Fatalf("minute_online batch rows = %d, want 1", func() int {
			if mc.batch == nil {
				return -1
			}
			return mc.batch.Rows()
		}())
	}
	row, ok := mc.batch.rows[0].([]interface{})
	if !ok {
		t.Fatalf("row not a value slice: %T", mc.batch.rows[0])
	}
	got, ok := row[0].(time.Time)
	if !ok {
		t.Fatalf("row[0] not time.Time: %T", row[0])
	}
	want := now.Add(-2 * time.Minute).UTC()
	if !got.Equal(want) {
		t.Fatalf("minute_online m = %v, want %v (must be the UTC minute)", got, want)
	}
}

// DATA-003: minute_online snapshots are replaced per minute key, not summed.
// A replayed flush must carry a newer engine version column.
func TestFlushMinuteOnlineReplacesSnapshot(t *testing.T) {
	w, _ := testWorker(t)
	key := pastMinuteKeyV9()
	w.touchedMinutes[key] = struct{}{}
	if err := w.flush(context.Background()); err != nil {
		t.Fatalf("first flush: %v", err)
	}
	mc := getMockConn(t, w)
	wantQuery := "INSERT INTO analytics.minute_online (m, game_id, env, online, version)"
	if mc.lastQuery != wantQuery {
		t.Fatalf("minute_online insert = %q, want %q", mc.lastQuery, wantQuery)
	}
	if len(mc.batch.rows) != 1 || len(mc.batch.rows[0].([]any)) != 5 {
		t.Fatalf("minute_online rows/columns = %d/%d, want 1/5", len(mc.batch.rows),
			func() int {
				if len(mc.batch.rows) == 0 {
					return -1
				}
				return len(mc.batch.rows[0].([]any))
			}())
	}
	first := mc.batch.rows[0].([]any)
	if ver := first[4].(uint64); ver == 0 {
		t.Fatal("minute_online version must be a non-zero unix-second snapshot marker")
	}

	w.touchedMinutes[key] = struct{}{}
	if err := w.flush(context.Background()); err != nil {
		t.Fatalf("replayed flush: %v", err)
	}
	second := mc.batch.rows[1].([]any)
	if ver := second[4].(uint64); ver < first[4].(uint64) {
		t.Fatalf("replayed version %d must not be older than %d", ver, first[4].(uint64))
	}
}
