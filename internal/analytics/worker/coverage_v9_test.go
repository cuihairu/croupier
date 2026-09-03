package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// V9 helpers
// ---------------------------------------------------------------------------

// stubConnV9 wraps *mockConn and lets a test override PrepareBatch results.
type stubConnV9 struct {
	*mockConn
	prepareErr error
	retBatch   driver.Batch
}

func (c *stubConnV9) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	c.mockConn.PrepareBatch(ctx, query, opts...)
	if c.prepareErr != nil {
		return nil, c.prepareErr
	}
	if c.retBatch != nil {
		return c.retBatch, nil
	}
	return c.mockConn.batch, nil
}

// appendErrBatchV9 wraps *mockBatch with a failing Append.
type appendErrBatchV9 struct {
	*mockBatch
	appendErr error
}

func (b *appendErrBatchV9) Append(v ...any) error { return b.appendErr }

// deadRedisClientV9 returns a client pointing at a closed port (fast failures).
func deadRedisClientV9() *redis.Client {
	return redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: 0})
}

// pastMinuteKeyV9 builds an hll:online key for a minute safely in the past.
func pastMinuteKeyV9() string {
	return fmt.Sprintf("hll:online:g1:prod:%s",
		time.Now().Add(-2*time.Minute).Truncate(time.Minute).Format("200601021504"))
}

// ---------------------------------------------------------------------------
// NewWorker: valid batch size env
// ---------------------------------------------------------------------------

func TestNewWorkerValidBatchSizeV9(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://127.0.0.1:6379/0")
	t.Setenv("CLICKHOUSE_DSN", "clickhouse://127.0.0.1:9000/analytics")
	t.Setenv("ANALYTICS_CLICKHOUSE_BATCH", "640")

	w, err := NewWorker()
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	if w.clickBatchSize != 640 {
		t.Errorf("clickBatchSize = %d, want 640", w.clickBatchSize)
	}
	if w.consumer == "" || w.group == "" {
		t.Errorf("consumer/group must default, got %q/%q", w.consumer, w.group)
	}
}

// ---------------------------------------------------------------------------
// Run: pre-canceled context, real message processing, XReadGroup error loop
// ---------------------------------------------------------------------------

func TestRunPreCanceledContextV9(t *testing.T) {
	w, _ := testWorker(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Run = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}
}

func TestRunProcessesMessagesV9(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	// Groups created at "0" so messages added now are delivered by ">".
	w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "0")
	w.rdb.XGroupCreateMkStream(ctx, w.streamPayments, w.group, "0")

	w.rdb.XAdd(ctx, &redis.XAddArgs{Stream: w.streamEvents, Values: map[string]any{
		"data": `{"event":"click","game_id":"g1","env":"prod","user_id":"u1","ts":"2025-06-15T10:00:00Z"}`,
	}})
	w.rdb.XAdd(ctx, &redis.XAddArgs{Stream: w.streamPayments, Values: map[string]any{
		"data": `{"order_id":"o1","amount_cents":100,"status":"success","game_id":"g1","env":"prod","user_id":"u1","ts":"2025-06-15T10:00:00Z"}`,
	}})

	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(runCtx) }()

	// Wait until both checkpoints are persisted (= both messages processed).
	deadline := time.Now().Add(5 * time.Second)
	for {
		n, _ := w.rdb.Exists(context.Background(),
			w.checkpointPrefix+":"+w.streamEvents,
			w.checkpointPrefix+":"+w.streamPayments).Result()
		if n == 2 {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("messages were not processed within timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Run = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestRunXReadGroupErrorLoopV9(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "$")
	w.rdb.XGroupCreateMkStream(ctx, w.streamPayments, w.group, "$")

	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(runCtx) }()

	time.Sleep(100 * time.Millisecond)
	// Deleting the streams also destroys the groups → XReadGroup returns
	// NOGROUP errors (non-Nil) → warn + continue path.
	w.rdb.Del(context.Background(), w.streamEvents, w.streamPayments)
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Run = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

// ---------------------------------------------------------------------------
// Batch ensure/append error paths
// ---------------------------------------------------------------------------

func TestEnsureBatchPrepareErrorsV9(t *testing.T) {
	w, _ := testWorker(t)
	w.ch = &stubConnV9{mockConn: getMockConn(t, w), prepareErr: errors.New("prepare boom")}
	ctx := context.Background()

	if _, err := w.ensureEventBatch(ctx); err == nil {
		t.Error("ensureEventBatch: expected error")
	}
	if _, err := w.ensurePaymentBatch(ctx); err == nil {
		t.Error("ensurePaymentBatch: expected error")
	}
	if err := w.appendEventRow(ctx, "x"); err == nil {
		t.Error("appendEventRow: expected error")
	}
	if err := w.appendPaymentRow(ctx, "x"); err == nil {
		t.Error("appendPaymentRow: expected error")
	}
}

func TestAppendRowAppendErrorsV9(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()

	w.eventBatch = &appendErrBatchV9{mockBatch: &mockBatch{}, appendErr: errors.New("append boom")}
	if err := w.appendEventRow(ctx, "x"); err == nil {
		t.Error("appendEventRow: expected Append error")
	}

	w.paymentBatch = &appendErrBatchV9{mockBatch: &mockBatch{}, appendErr: errors.New("append boom")}
	if err := w.appendPaymentRow(ctx, "x"); err == nil {
		t.Error("appendPaymentRow: expected Append error")
	}
}

func TestFlushBatchesErrorPropagationV9(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()

	// Event batch flush error propagates.
	w.eventBatch = &mockBatch{sendErr: errors.New("send boom")}
	w.eventBatchRows = 2
	if err := w.flushBatches(ctx); err == nil {
		t.Error("flushBatches: expected event flush error")
	}

	// Payment batch flush error propagates (event batch now clean).
	w2, _ := testWorker(t)
	w2.paymentBatch = &mockBatch{sendErr: errors.New("send boom")}
	w2.paymentBatchRows = 1
	if err := w2.flushBatches(ctx); err == nil {
		t.Error("flushBatches: expected payment flush error")
	}

	// flush() itself never returns the batch error, only logs it.
	w3, _ := testWorker(t)
	w3.eventBatch = &mockBatch{sendErr: errors.New("send boom")}
	w3.eventBatchRows = 1
	if err := w3.flush(ctx); err != nil {
		t.Errorf("flush should swallow batch errors, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// flush(): minute_online / daily_users / daily_revenue error branches
// ---------------------------------------------------------------------------

func TestFlushMinuteOnlineErrorPathsV9(t *testing.T) {
	ctx := context.Background()

	// PFCount failure (dead Redis).
	w, _ := testWorker(t)
	w.rdb = deadRedisClientV9()
	k := pastMinuteKeyV9()
	w.touchedMinutes[k] = struct{}{}
	if err := w.flush(ctx); err != nil {
		t.Fatalf("flush = %v", err)
	}
	if _, ok := w.touchedMinutes[k]; !ok {
		t.Error("key should remain after PFCount failure")
	}

	// PrepareBatch failure.
	w2, _ := testWorker(t)
	w2.ch = &stubConnV9{mockConn: getMockConn(t, w2), prepareErr: errors.New("pb")}
	w2.touchedMinutes[pastMinuteKeyV9()] = struct{}{}
	if err := w2.flush(ctx); err != nil {
		t.Fatalf("flush = %v", err)
	}
	if len(w2.touchedMinutes) != 1 {
		t.Error("key should remain after PrepareBatch failure")
	}

	// Append failure.
	w3, _ := testWorker(t)
	w3.ch = &stubConnV9{mockConn: getMockConn(t, w3),
		retBatch: &appendErrBatchV9{mockBatch: &mockBatch{}, appendErr: errors.New("ab")}}
	w3.touchedMinutes[pastMinuteKeyV9()] = struct{}{}
	if err := w3.flush(ctx); err != nil {
		t.Fatalf("flush = %v", err)
	}

	// Send failure.
	w4, _ := testWorker(t)
	w4.ch = &stubConnV9{mockConn: getMockConn(t, w4),
		retBatch: &mockBatch{sendErr: errors.New("sb")}}
	w4.touchedMinutes[pastMinuteKeyV9()] = struct{}{}
	if err := w4.flush(ctx); err != nil {
		t.Fatalf("flush = %v", err)
	}
	if len(w4.touchedMinutes) != 1 {
		t.Error("key should remain after Send failure")
	}
}

func TestFlushDailyUsersErrorPathsV9(t *testing.T) {
	ctx := context.Background()

	// PrepareBatch failure.
	w, _ := testWorker(t)
	w.ch = &stubConnV9{mockConn: getMockConn(t, w), prepareErr: errors.New("pb")}
	w.touchedDays["2025-06-15|g1|e1"] = struct{}{}
	if err := w.flush(ctx); err != nil {
		t.Fatalf("flush = %v", err)
	}
	if _, ok := w.touchedDays["2025-06-15|g1|e1"]; !ok {
		t.Error("key should remain after PrepareBatch failure")
	}

	// Append failure.
	w2, _ := testWorker(t)
	w2.ch = &stubConnV9{mockConn: getMockConn(t, w2),
		retBatch: &appendErrBatchV9{mockBatch: &mockBatch{}, appendErr: errors.New("ab")}}
	w2.touchedDays["2025-06-15|g1|e1"] = struct{}{}
	if err := w2.flush(ctx); err != nil {
		t.Fatalf("flush = %v", err)
	}
	if len(w2.touchedDays) != 1 {
		t.Error("key should remain after Append failure")
	}

	// Send failure.
	w3, _ := testWorker(t)
	w3.ch = &stubConnV9{mockConn: getMockConn(t, w3),
		retBatch: &mockBatch{sendErr: errors.New("sb")}}
	w3.touchedDays["2025-06-15|g1|e1"] = struct{}{}
	if err := w3.flush(ctx); err != nil {
		t.Fatalf("flush = %v", err)
	}
	if len(w3.touchedDays) != 1 {
		t.Error("key should remain after Send failure")
	}
}

func TestFlushDailyRevenueErrorPathsV9(t *testing.T) {
	ctx := context.Background()

	// PrepareBatch failure.
	w, _ := testWorker(t)
	w.ch = &stubConnV9{mockConn: getMockConn(t, w), prepareErr: errors.New("pb")}
	w.revAgg["2025-06-15|g1|e1"] = &revRow{revenue: 10}
	if err := w.flush(ctx); err != nil {
		t.Fatalf("flush = %v", err)
	}
	if _, ok := w.revAgg["2025-06-15|g1|e1"]; !ok {
		t.Error("key should remain after PrepareBatch failure")
	}

	// Append failure.
	w2, _ := testWorker(t)
	w2.ch = &stubConnV9{mockConn: getMockConn(t, w2),
		retBatch: &appendErrBatchV9{mockBatch: &mockBatch{}, appendErr: errors.New("ab")}}
	w2.revAgg["2025-06-15|g1|e1"] = &revRow{revenue: 10}
	if err := w2.flush(ctx); err != nil {
		t.Fatalf("flush = %v", err)
	}
	if len(w2.revAgg) != 1 {
		t.Error("key should remain after Append failure")
	}
}

// ---------------------------------------------------------------------------
// Checkpoints
// ---------------------------------------------------------------------------

func TestRestoreCheckpointsV9(t *testing.T) {
	ctx := context.Background()

	// Get error (dead Redis, non-Nil).
	w, _ := testWorker(t)
	w.rdb = deadRedisClientV9()
	w.restoreCheckpoints(ctx, w.streamEvents) // must not panic

	// XGroupSetID failure: miniredis does not implement XGROUP SETID, so the
	// setid error branch (warn + return) is exercised with a group present.
	w2, _ := testWorker(t)
	w2.rdb.XGroupCreateMkStream(ctx, w2.streamEvents, w2.group, "$")
	w2.rdb.Set(ctx, w2.checkpointPrefix+":"+w2.streamEvents, "5-1", 0)
	w2.restoreCheckpoints(ctx, w2.streamEvents)
	if got := w2.lastIDs[w2.streamEvents]; got != "" {
		t.Errorf("lastIDs should stay empty on setid failure, got %q", got)
	}

	// Same on a stream without any group.
	w3, _ := testWorker(t)
	w3.rdb.Set(ctx, w3.checkpointPrefix+":"+w3.streamEvents, "5-1", 0)
	w3.restoreCheckpoints(ctx, w3.streamEvents)
	if got := w3.lastIDs[w3.streamEvents]; got != "" {
		t.Errorf("lastIDs should stay empty on setid failure, got %q", got)
	}
}

func TestPersistCheckpointSetErrorV9(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb = deadRedisClientV9()
	w.persistCheckpoint(context.Background(), w.streamEvents, "1-1") // must not panic
}

// ---------------------------------------------------------------------------
// processMessage / dead letter error paths
// ---------------------------------------------------------------------------

func TestProcessMessageAckErrorV9(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	// The stream key holds a non-stream value → XACK fails with WRONGTYPE.
	w.rdb.Set(ctx, w.streamEvents, "not-a-stream", 0)
	msg := redis.XMessage{ID: "1-1", Values: map[string]any{
		"data": `{"event":"click","game_id":"g1","env":"prod","user_id":"u1"}`,
	}}
	err := w.processMessage(ctx, w.streamEvents, msg)
	if err == nil {
		t.Fatal("processMessage: expected ack error")
	}
}

func TestSendToDeadLetterXAddErrorV9(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb = deadRedisClientV9()
	w.sendToDeadLetter(context.Background(), w.streamEvents,
		redis.XMessage{ID: "1-1", Values: map[string]any{"data": "junk"}},
		"invalid_json", "boom") // must not panic
}

// ---------------------------------------------------------------------------
// reclaim: idle pending messages via miniredis SetTime
// ---------------------------------------------------------------------------

func TestReclaimPendingIdleClaimV9(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "$")
	w.rdb.XAdd(ctx, &redis.XAddArgs{Stream: w.streamEvents, Values: map[string]any{
		"data": `{"event":"click","game_id":"g1","env":"prod","user_id":"u9","ts":"2025-06-15T10:00:00Z"}`,
	}})
	w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: w.group, Consumer: "stale-consumer",
		Streams: []string{w.streamEvents, ">"}, Count: 10,
	})

	// Advance miniredis clock: the pending entry is now idle ≥ 5 minutes.
	mr.SetTime(time.Now().Add(10 * time.Minute))

	w.reclaimPendingFromStream(ctx, w.streamEvents)

	// The reclaimed message was processed: checkpoint persisted and PEL drained.
	n, err := w.rdb.Exists(ctx, w.checkpointPrefix+":"+w.streamEvents).Result()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("checkpoint missing after reclaim (exists=%d)", n)
	}
	pending, _ := w.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: w.streamEvents, Group: w.group,
		Start: "-", End: "+", Count: 100,
	}).Result()
	if len(pending) != 0 {
		t.Errorf("pending should be drained after reclaim, got %d", len(pending))
	}
}
