package worker

import (
	"context"
	"testing"
	"time"
)

// 时区无关回归：事件 ts 带非 UTC offset（+09:30）时，minute-online 分桶
// 必须仍归一到正确的 UTC 分钟并被 flush。写入侧若按事件 offset（或本地
// 时区 fallback）渲染 hll key，flush 侧 time.Parse（恒 UTC）会与 key 错开
// 一个时区偏移，t.Before(nowMin) 永假，非 UTC 部署上 minute_online 永不
// 落库（993cb65 审计时间窗同族；本用例在任意机器时区下均应通过）。
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
		t.Fatalf("minute key not flushed; touchedMinutes=%v (写入侧未归一 UTC?)", w.touchedMinutes)
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
		t.Fatalf("minute_online m = %v, want %v (必须 UTC 分钟)", got, want)
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
