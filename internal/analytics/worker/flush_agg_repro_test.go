package worker

import (
	"testing"
	"time"
)

// Reproduces the CI failure: after touchAgg/touchRevenue, a flush() call
// must attempt the daily_users / daily_revenue inserts (i.e. clear
// touchedDays / revAgg). If the mock ClickHouse conn records the prepared
// batch SQL we can also assert which statements were executed.
func TestFlushDailyAggregatesAttempted(t *testing.T) {
	w, _ := testWorker(t)
	now := time.Now()

	w.touchAgg(t.Context(), map[string]any{
		"game_id": "e2e-game", "env": "dev", "user_id": "u1",
		"event": "login", "ts": now.Format(time.RFC3339),
	})
	w.touchRevenue(t.Context(), map[string]any{
		"game_id": "e2e-game", "env": "dev", "user_id": "u1",
		"order_id": "O1", "amount_cents": 100.0, "status": "success",
		"ts": now.Format(time.RFC3339),
	})

	if len(w.touchedDays) != 1 {
		t.Fatalf("touchedDays = %v", w.touchedDays)
	}
	if len(w.revAgg) != 1 {
		t.Fatalf("revAgg = %v", w.revAgg)
	}

	if err := w.flush(t.Context()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if len(w.touchedDays) != 0 {
		t.Errorf("flush should consume touchedDays, got %v", w.touchedDays)
	}
	if len(w.revAgg) != 0 {
		t.Errorf("flush should consume revAgg, got %v", w.revAgg)
	}
}

func TestNormalizeTimestamp(t *testing.T) {
	now := time.Now().Format("2006-01-02 15:04:05")
	cases := []struct{ in, want string }{
		{"", now}, // 空 → 当前时间（仅比对布局）
		{"2026-08-24T03:02:55Z", "2026-08-24 03:02:55"},
		{"2026-08-24T11:02:55+08:00", "2026-08-24 03:02:55"},
		{"2026-08-24 03:02:55", "2026-08-24 03:02:55"}, // 已是目标布局
		{"1787540575", time.Unix(1787540575, 0).UTC().Format("2006-01-02 15:04:05")},
		{"1787540575000", time.Unix(1787540575, 0).UTC().Format("2006-01-02 15:04:05")}, // 毫秒
		{"garbage", "garbage"}, // 未知格式透传（CH Append 报错走死信）
	}
	for _, tc := range cases {
		got := normalizeTimestamp(tc.in)
		if got != tc.want {
			t.Errorf("normalizeTimestamp(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestInsertEventGeneratesUUIDWhenMissing(t *testing.T) {
	w, _ := testWorker(t)
	// No event_id in the payload: appendEventRow must receive a valid UUID
	// or ClickHouse Append fails (reproduces CI dead-letter loop).
	err := w.insertEvent(t.Context(), map[string]any{
		"game_id": "g", "env": "dev", "user_id": "u", "event": "login",
		"ts": "2026-08-24T03:02:55Z",
	})
	if err != nil {
		t.Fatalf("insertEvent without event_id: %v", err)
	}
	batch := w.eventBatch.(*mockBatch)
	if len(batch.rows) == 0 {
		t.Fatal("no rows appended")
	}
	// Each appended row is []any of the 12 insertEventsSQL columns;
	// event_id is column index 10.
	row, ok := batch.rows[0].([]any)
	if !ok {
		t.Fatalf("appended row type %T, want []any", batch.rows[0])
	}
	var eventID string
	switch v := row[10].(type) {
	case string:
		eventID = v
	default:
		t.Fatalf("event_id column type %T, want string", row[10])
	}
	if eventID == "" {
		t.Fatal("event_id must be generated when payload omits it")
	}
}

func TestNormalizeAggEventAliases(t *testing.T) {
	cases := map[string]string{
		// OTel dotted names (telemetry bridge producers)
		"session.start": "session_start",
		"session.end":   "session_end",
		"user.login":    "login",
		"user.register": "register",
		"USER.LOGIN":    "login", // case-insensitive
		// Native underscore names pass through
		"session_start": "session_start",
		"login":         "login",
		"heartbeat":     "heartbeat",
		"register":      "register",
		"first_active":  "first_active",
		// Unknown events unchanged (no aggregation, detail insert only)
		"purchase":      "purchase",
		"gacha.pull":    "gacha.pull",
		"":              "",
		"  user.login ": "login", // whitespace tolerated
	}
	for in, want := range cases {
		if got := normalizeAggEvent(in); got != want {
			t.Errorf("normalizeAggEvent(%q) = %q, want %q", in, got, want)
		}
	}
}

// Dotted OTel events must drive the same HLL aggregations as underscore
// names: session.start → minute online + DAU, user.register → new users.
func TestTouchAggDottedEventsDriveAggregation(t *testing.T) {
	w, mr := testWorker(t)
	now := time.Now().UTC()

	w.touchAgg(t.Context(), map[string]any{
		"game_id": "g", "env": "dev", "user_id": "u1",
		"event": "session.start", "ts": now.Format(time.RFC3339),
	})
	if len(w.touchedMinutes) == 0 {
		t.Error("session.start should touch minute online HLL")
	}
	if len(w.touchedDays) == 0 {
		t.Error("session.start should touch DAU HLL")
	}

	w.touchAgg(t.Context(), map[string]any{
		"game_id": "g", "env": "dev", "user_id": "u2",
		"event": "user.register", "ts": now.Format(time.RFC3339),
	})
	// register only touches the new-users HLL → still one distinct day|game|env key
	if len(w.touchedDays) != 1 {
		t.Errorf("touchedDays = %v, want 1", w.touchedDays)
	}
	_ = mr
}
