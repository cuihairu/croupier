package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/redis/go-redis/v9"
)

// ===========================================================================
// ARCH-011 — analytics worker acknowledgement after persistence (deferred XACK)
//
// Core invariant: a message must not be XACKed before the ClickHouse batch
// Send() succeeds. While a message row is merely appended into the in-memory
// batch it stays PENDING; when Send fails the batch and its ack refs are
// dropped together, leaving the message PENDING for reclaim to redeliver
// (naturally idempotent — the raw tables have no server-side dedup, and the
// consumer dedups by event_id/order_id).
// ===========================================================================

// appendEventRow records an ackRef without ACKing immediately (message stays PENDING).
func TestAppendEventRow_RecordsAckRef(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()

	if err := w.appendEventRow(ctx, "7-1", "ts", "g1", "prod"); err != nil {
		t.Fatalf("appendEventRow: %v", err)
	}
	if len(w.eventAcks) != 1 {
		t.Fatalf("eventAcks = %d, want 1", len(w.eventAcks))
	}
	if w.eventAcks[0].stream != w.streamEvents || w.eventAcks[0].id != "7-1" {
		t.Errorf("ackRef = %+v, want {stream:%s id:7-1}", w.eventAcks[0], w.streamEvents)
	}
	if w.eventBatchRows != 1 {
		t.Errorf("eventBatchRows = %d, want 1", w.eventBatchRows)
	}
}

// appendPaymentRow records a payment ackRef without ACKing immediately.
func TestAppendPaymentRow_RecordsAckRef(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()

	if err := w.appendPaymentRow(ctx, "9-2", "ts", "g1", "prod"); err != nil {
		t.Fatalf("appendPaymentRow: %v", err)
	}
	if len(w.paymentAcks) != 1 {
		t.Fatalf("paymentAcks = %d, want 1", len(w.paymentAcks))
	}
	if w.paymentAcks[0].stream != w.streamPayments || w.paymentAcks[0].id != "9-2" {
		t.Errorf("ackRef = %+v, want {stream:%s id:9-2}", w.paymentAcks[0], w.streamPayments)
	}
	if w.paymentBatchRows != 1 {
		t.Errorf("paymentBatchRows = %d, want 1", w.paymentBatchRows)
	}
}

// After a successful batch Send, its refs are ACKed and the checkpoint advances;
// before that the message stays PENDING.
func TestFlushEventBatch_AcksRefsAfterSend(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "$")

	// Claim the message into the group's pending set so the deferred-ack
	// semantics are observable (processMessage itself does not XReadGroup).
	id := w.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: w.streamEvents, Values: map[string]any{"data": `{"event":"x"}`},
	}).Val()
	res, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: w.group, Consumer: "test-consumer",
		Streams: []string{w.streamEvents, ">"}, Count: 10,
	}).Result()
	if err != nil || len(res) == 0 || len(res[0].Messages) == 0 {
		t.Fatalf("claim: err=%v res=%v", err, res)
	}
	if res[0].Messages[0].ID != id {
		t.Fatalf("claimed id %q, want %q", res[0].Messages[0].ID, id)
	}

	if err := w.appendEventRow(ctx, id, "ts", "g1", "prod"); err != nil {
		t.Fatalf("appendEventRow: %v", err)
	}

	// Not yet flushed: message still pending, no checkpoint.
	pending, _ := w.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: w.streamEvents, Group: w.group, Start: "-", End: "+", Count: 100,
	}).Result()
	if len(pending) != 1 {
		t.Fatalf("before flush: pending = %d, want 1 (ack deferred)", len(pending))
	}
	if n, _ := w.rdb.Exists(ctx, w.checkpointPrefix+":"+w.streamEvents).Result(); n != 0 {
		t.Fatalf("before flush: checkpoint must not exist")
	}

	// Flush: Send succeeds → refs ACKed, checkpoint advanced.
	if err := w.flushEventBatch(ctx); err != nil {
		t.Fatalf("flushEventBatch: %v", err)
	}
	pending, _ = w.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: w.streamEvents, Group: w.group, Start: "-", End: "+", Count: 100,
	}).Result()
	if len(pending) != 0 {
		t.Errorf("after flush: pending = %d, want 0 (acked)", len(pending))
	}
	got, _ := w.rdb.Get(ctx, w.checkpointPrefix+":"+w.streamEvents).Result()
	if got != id {
		t.Errorf("checkpoint = %q, want %q", got, id)
	}
	if w.lastIDs[w.streamEvents] != id {
		t.Errorf("lastIDs = %q, want %q", w.lastIDs[w.streamEvents], id)
	}
}

// advanceCheckpoint must be monotonic: after the later-flushed message is ACKed,
// ACKing an earlier message must not regress the high-water mark (otherwise a
// restart would re-read already-processed messages).
func TestAdvanceCheckpoint_Monotonic(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()

	w.advanceCheckpoint(ctx, w.streamEvents, "2000-5")
	w.advanceCheckpoint(ctx, w.streamEvents, "1000-0") // earlier — must not regress

	got, _ := w.rdb.Get(ctx, w.checkpointPrefix+":"+w.streamEvents).Result()
	if got != "2000-5" {
		t.Errorf("checkpoint = %q, want 2000-5 (no regression)", got)
	}
	if w.lastIDs[w.streamEvents] != "2000-5" {
		t.Errorf("lastIDs = %q, want 2000-5", w.lastIDs[w.streamEvents])
	}
}

// A failed requeue XAdd must return an error and not ACK — otherwise the original
// message gets confirmed although its row never landed in the database.
func TestProcessMessage_RequeueFailureNoAck(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb = deadRedisClientV9()
	w.clickBatchSize = 1 // flush each row so the mock Send error surfaces
	getMockConn(t, w).batch = &mockBatch{sendErr: errors.New("ch down")}

	msg := redis.XMessage{ID: "3-0", Values: map[string]any{
		"data": `{"event":"click","game_id":"g1","env":"prod","user_id":"u1","ts":"2025-06-15T10:00:00Z"}`,
	}}
	// Re-queue XAdd hits the dead Redis → processMessage must return the error
	// (contract: caller leaves the message pending for reclaim).
	if err := w.processMessage(context.Background(), w.streamEvents, msg); err == nil {
		t.Fatal("processMessage: expected error on requeue failure (message must not be acked)")
	}
}

// A failed dead-letter XAdd must return an error and not ACK — otherwise the
// original message gets confirmed although its row never entered the dead-letter stream.
func TestProcessMessage_DeadLetterFailureNoAck(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb = deadRedisClientV9()

	msg := redis.XMessage{ID: "4-0", Values: map[string]any{"data": "not-json"}}
	if err := w.processMessage(context.Background(), w.streamEvents, msg); err == nil {
		t.Fatal("processMessage: expected error on dead-letter failure (message must not be acked)")
	}
}

// A successful requeue ACKs the original message (its row is redelivered as a
// retry copy), leaving the retry copy in the stream.
func TestProcessMessage_RequeueSuccessAcksOriginal(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	w.clickBatchSize = 1 // flush each row so the mock Send error surfaces
	getMockConn(t, w).batch = &mockBatch{sendErr: fmt.Errorf("ch error")}
	w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "$")

	eventData := map[string]any{
		"ts": "2025-06-15T10:30:00Z", "game_id": "g1", "env": "prod",
		"user_id": "u1", "event": "login",
	}
	data, _ := json.Marshal(eventData)

	id := w.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: w.streamEvents, Values: map[string]any{"data": string(data)},
	}).Val()
	claimed, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: w.group, Consumer: "test-consumer",
		Streams: []string{w.streamEvents, ">"}, Count: 10,
	}).Result()
	if err != nil || len(claimed) == 0 || len(claimed[0].Messages) == 0 {
		t.Fatalf("claim: err=%v", err)
	}
	msg := claimed[0].Messages[0]
	if msg.ID != id {
		t.Fatalf("claimed id %q, want %q", msg.ID, id)
	}

	if err := w.processMessage(ctx, w.streamEvents, msg); err != nil {
		t.Fatalf("processMessage: %v", err)
	}

	// Re-queue succeeded → original ACKed (pending drained), retry copy present.
	pending, _ := w.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: w.streamEvents, Group: w.group, Start: "-", End: "+", Count: 100,
	}).Result()
	if len(pending) != 0 {
		t.Errorf("pending = %d, want 0 (original acked after successful requeue)", len(pending))
	}
	if streamLen := w.rdb.XLen(ctx, w.streamEvents).Val(); streamLen < 2 {
		t.Errorf("stream len = %d, want >= 2 (original + retry copy)", streamLen)
	}
}
