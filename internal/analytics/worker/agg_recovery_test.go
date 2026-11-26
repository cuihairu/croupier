package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/redis/go-redis/v9"
)

// hllFailRedis simulates a Redis HLL write failure while leaving every other
// command (dirty markers, PFCount, set reads) on the real client. This is the
// DATA-020 scenario: the raw event is already ACKed in ClickHouse when the
// HLL write fails, so the metric cannot be lost.
type hllFailRedis struct {
	aggRedis
	fail bool
}

func (h *hllFailRedis) PFAdd(ctx context.Context, key string, els ...interface{}) *redis.IntCmd {
	if h.fail {
		return redis.NewIntResult(0, errors.New("PFADD unavailable"))
	}
	return h.aggRedis.PFAdd(ctx, key, els...)
}

// rawEventConn serves the rebuild query from an in-memory map keyed by
// game|env|event, and records aggregate rows that were flushed.
type rawEventConn struct {
	mockConn
	users    map[string][]string
	flushed  []string
	failRead bool
	queryN   int
}

func (c *rawEventConn) Select(ctx context.Context, dest any, query string, args ...any) error {
	c.queryN++
	if c.failRead {
		return errors.New("clickhouse unavailable")
	}
	if len(args) < 4 {
		return fmt.Errorf("unexpected args: %v", args)
	}
	game, _ := args[0].(string)
	env, _ := args[1].(string)
	out, ok := dest.(*[]string)
	if !ok {
		return fmt.Errorf("unexpected dest %T", dest)
	}
	// Match any event variant for the bucket kind by unioning all users for
	// the game/env whose event name appears in the IN list.
	seen := map[string]bool{}
	for key, uids := range c.users {
		if prefix := game + "|" + env + "|"; len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			event := key[len(prefix):]
			for _, a := range args[4:] {
				if e, ok := a.(string); ok && e == event {
					for _, u := range uids {
						if !seen[u] {
							seen[u] = true
							*out = append(*out, u)
						}
					}
				}
			}
		}
	}
	return nil
}

func (c *rawEventConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	return &recordingAggBatch{conn: c, query: query}, nil
}

type recordingAggBatch struct {
	conn  *rawEventConn
	query string
	rows  [][]any
}

func (b *recordingAggBatch) Append(v ...any) error { b.rows = append(b.rows, v); return nil }
func (b *recordingAggBatch) Send() error {
	for _, row := range b.rows {
		b.conn.flushed = append(b.conn.flushed, fmt.Sprintf("%s|%v", b.query, row))
	}
	return nil
}
func (b *recordingAggBatch) Abort() error                  { return nil }
func (b *recordingAggBatch) AppendStruct(v any) error      { return nil }
func (b *recordingAggBatch) Column(int) driver.BatchColumn { return nil }
func (b *recordingAggBatch) Flush() error                  { return nil }
func (b *recordingAggBatch) IsSent() bool                  { return false }
func (b *recordingAggBatch) Rows() int                     { return len(b.rows) }
func (b *recordingAggBatch) Columns() []column.Interface   { return nil }
func (b *recordingAggBatch) Close() error                  { return nil }

func aggTestPayload(evt string, ts time.Time) map[string]any {
	return map[string]any{
		"ts": ts.UTC().Format(time.RFC3339), "game_id": "g1", "env": "prod",
		"user_id": "u1", "event": evt,
	}
}

// Core DATA-020 assertion: an event whose raw row is durably stored but
// whose HLL write failed is fully rebuilt after restart — from the persisted
// raw events, not from the in-memory touched map (which a restart loses).
func TestAggRecovery_RebuildsFromRawEventsAfterCrash(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	failedHLL := &hllFailRedis{aggRedis: w.aggRdb, fail: true}
	w.aggRdb = failedHLL
	ch := &rawEventConn{users: map[string][]string{
		"g1|prod|login": {"u1", "u2", "u3"},
	}}
	w.ch = ch

	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	// Event 1: raw row is ACKed; the HLL write fails; the dirty marker is
	// still durably written before the failed PFAdd.
	w.touchAgg(ctx, aggTestPayload("login", ts))
	if n := w.rdb.PFCount(ctx, fmt.Sprintf("hll:dau:g1:prod:%s", ts.Format("2006-01-02"))).Val(); n != 0 {
		t.Fatalf("expected failed HLL write to leave count 0, got %d", n)
	}
	if got := w.rdb.SCard(ctx, w.aggDirtySet).Val(); got != 1 {
		t.Fatalf("dirty markers = %d, want 1", got)
	}

	// Crash: a brand-new Worker on the same Redis has empty in-memory state.
	w2 := testWorkerOn(t, mr)
	w2.ch = ch
	w2.ensureGroups(ctx)
	if len(w2.touchedDays) != 1 {
		t.Fatalf("restart did not restore dirty day: %v", w2.touchedDays)
	}

	// Flush must rebuild the DAU HLL from the persisted raw events and write
	// a truthful 3 — a bare PFCount on the missing HLL would have written 0.
	if err := w2.flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
	n := w2.rdb.PFCount(ctx, fmt.Sprintf("hll:dau:g1:prod:%s", ts.Format("2006-01-02"))).Val()
	if n != 3 {
		t.Fatalf("rebuilt DAU = %d, want 3", n)
	}
	if len(ch.flushed) == 0 {
		t.Fatal("no daily_users row flushed")
	}
	if got := w2.rdb.SCard(ctx, w2.aggDirtySet).Val(); got != 0 {
		t.Fatalf("dirty markers after flush = %d, want 0", got)
	}
}

// A failed rebuild (ClickHouse unavailable) must NOT write a zero aggregate
// row or clear the marker: the next flush still owes the rebuild.
func TestAggRecovery_RebuildFailureKeepsDirtyAndDoesNotPersistZero(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	// The HLL write failed (so a rebuild marker is set) and ClickHouse is
	// unavailable for the rebuild query: flush must not persist a false zero.
	w.aggRdb = &hllFailRedis{aggRedis: w.aggRdb, fail: true}
	ch := &rawEventConn{failRead: true}
	w.ch = ch

	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	w.touchAgg(ctx, aggTestPayload("login", ts))
	if err := w.flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(ch.flushed) != 0 {
		t.Fatalf("rebuild failure must not persist a row: %v", ch.flushed)
	}
	if got := w.rdb.SCard(ctx, w.aggDirtySet).Val(); got != 1 {
		t.Fatalf("dirty marker lost after failed rebuild: %d", got)
	}
}

// A healthy HLL write must NOT trigger a raw-event rebuild query: only a
// bucket with a durable rebuild marker pays the ClickHouse scan cost.
func TestAggRecovery_SuccessfulHLLWriteSkipsRebuild(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	ch := &rawEventConn{users: map[string][]string{
		"g1|prod|login": {"u1", "u2", "u3"},
	}}
	w.ch = ch
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	w.touchAgg(ctx, aggTestPayload("login", ts))
	if got := w.rdb.SCard(ctx, w.aggRebuildSet).Val(); got != 0 {
		t.Fatalf("rebuild markers = %d, want 0 on a healthy write", got)
	}
	if err := w.flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if ch.queryN != 0 {
		t.Fatalf("healthy flush issued %d raw-event rebuild queries, want 0", ch.queryN)
	}
	if got := w.rdb.SCard(ctx, w.aggDirtySet).Val(); got != 0 {
		t.Fatalf("flush markers after successful row = %d, want 0", got)
	}
}

// Duplicate reconstruction must be idempotent: after a successful rebuild
// and marker clear, a second flush (no new events) does not double-count.
func TestAggRecovery_RepeatFlushDoesNotDoubleCount(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	key := fmt.Sprintf("hll:dau:g1:prod:%s", ts.Format("2006-01-02"))
	// The HLL is already populated from a previous successful write; no
	// rebuild occurs, and repeated flushes read the same count.
	w.touchAgg(ctx, aggTestPayload("login", ts))
	w.rdb.PFAdd(ctx, key, "u1", "u2")
	ch := &rawEventConn{users: map[string][]string{"g1|prod|login": {"u1", "u2"}}}
	w.ch = ch
	for i := 0; i < 2; i++ {
		if err := w.flush(ctx); err != nil {
			t.Fatalf("flush %d: %v", i, err)
		}
	}
	if n := w.rdb.PFCount(ctx, key).Val(); n != 2 {
		t.Fatalf("HLL count after two flushes = %d, want 2", n)
	}
	// Restart with a cleared marker: no dirty state to restore, count intact.
	w2 := testWorkerOn(t, mr)
	w2.ch = ch
	w2.ensureGroups(ctx)
	if len(w2.touchedDays) != 0 {
		t.Fatalf("unexpected restored dirty days: %v", w2.touchedDays)
	}
	if n := w2.rdb.PFCount(ctx, key).Val(); n != 2 {
		t.Fatalf("HLL count after restart = %d, want 2", n)
	}
}

// Scenario from the acceptance line: raw event is ACKed and the process dies
// before the aggregate row is persisted. The dirty marker survives in Redis
// and the next worker rebuilds the row from the raw events.
func TestAggRecovery_PendingDirtySurvivesUntilAggFlush(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	w.aggRdb = &hllFailRedis{aggRedis: w.aggRdb, fail: true}
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	w.touchAgg(ctx, aggTestPayload("heartbeat", ts))
	// No flush before the simulated crash: raw event already ACKed, marker
	// durable, in-memory state lost with the process.
	w2 := testWorkerOn(t, mr)
	w2.ch = &rawEventConn{users: map[string][]string{
		"g1|prod|heartbeat": {"u1"},
	}}
	w2.ensureGroups(ctx)
	if len(w2.touchedMinutes) != 1 {
		t.Fatalf("minute dirty state not restored: %v", w2.touchedMinutes)
	}
	// Heartbeats are in the past (2025) so flush writes the minute row.
	if err := w2.flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if got := w2.rdb.SCard(ctx, w2.aggDirtySet).Val(); got != 0 {
		t.Fatalf("dirty markers after recovery flush = %d, want 0", got)
	}
}
