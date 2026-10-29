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
// ARCH-011 — 分析 worker 持久化后确认（deferred XACK）
//
// 核心不变量：消息在 ClickHouse 批次 Send() 成功之前不得 XACK。消息行只被
// append 进内存批次时仍保持 PENDING；Send 失败时批次与其 ack refs 一并
// 丢弃，消息留在 PENDING 由 reclaim 重投（天然幂等，raw 表无服务端去重，
// 消费端以 event_id/order_id 为去重键）。
// ===========================================================================

// appendEventRow 记录 ackRef，且不立即 ACK（消息保持 PENDING）。
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

// appendPaymentRow 记录 payment ackRef，且不立即 ACK。
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

// 批次 Send 成功后 ACK 其 refs 并推进 checkpoint；此前消息保持 PENDING。
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

// advanceCheckpoint 必须单调：后 flush 的消息先 ACK 后，再 ACK 较早消息不得
// 回退高水位（否则重启后会重读已解析消息）。
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

// requeue XAdd 失败时必须返回错误且不 ACK——否则原消息被确认后其行从未落库。
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

// dead-letter XAdd 失败时必须返回错误且不 ACK——否则原消息被确认后其行从未
// 进入死信流。
func TestProcessMessage_DeadLetterFailureNoAck(t *testing.T) {
	w, _ := testWorker(t)
	w.rdb = deadRedisClientV9()

	msg := redis.XMessage{ID: "4-0", Values: map[string]any{"data": "not-json"}}
	if err := w.processMessage(context.Background(), w.streamEvents, msg); err == nil {
		t.Fatal("processMessage: expected error on dead-letter failure (message must not be acked)")
	}
}

// requeue 成功时 ACK 原消息（其行重投到 retry 副本），retry 副本留在流中。
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
