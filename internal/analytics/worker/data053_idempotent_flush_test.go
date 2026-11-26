package worker

import (
	"context"
	"errors"
	"testing"

	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const data053PaymentPayload = `{"ts":"2025-06-15T10:30:00Z","game_id":"g1","env":"e1","status":"success","amount_cents":100,"order_id":"o1"}`

// data053Pending adds a payment to the source stream and delivers it to the
// worker's consumer so it sits in the PEL (un-ACKed), the state a row is in
// while its batch has not durably Send()ed yet.
func data053Pending(t *testing.T, w *Worker, payload string) string {
	t.Helper()
	ctx := ctxBackground()
	require.NoError(t, w.rdb.XGroupCreateMkStream(ctx, w.streamPayments, w.group, "$").Err())
	id := w.rdb.XAdd(ctx, &redis.XAddArgs{Stream: w.streamPayments, Values: map[string]any{"data": payload}}).Val()
	_, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: w.group, Consumer: w.consumer, Streams: []string{w.streamPayments, ">"},
	}).Result()
	require.NoError(t, err)
	require.Len(t, mustPending(t, w, w.streamPayments), 1)
	return id
}

// 验收（DATA-053）：Send 失败后行与其延迟 ref 被保留（held），源消息保持
// pending；仓库恢复后下一轮刷新重派生该行、Send 成功、ref ACK、PEL 清空、
// 营收恰好入账一次。
func TestFlushHold_RecoveryAfterWarehouseRecovers(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	id := data053Pending(t, w, data053PaymentPayload)
	w.holdEventData(w.streamPayments, id, data053PaymentPayload)

	good := &mockBatch{}
	getMockConn(t, w).batch = good
	w.clickBatchSize = 0 // no auto-flush; drive rounds explicitly

	w.paymentBatch = &mockBatch{sendErr: errors.New("clickhouse down")}
	w.paymentBatchRows = 1
	w.paymentAcks = []ackRef{{stream: w.streamPayments, id: id}}
	err := w.flushPaymentBatch(ctx)
	require.Error(t, err)
	require.Len(t, w.paymentHeld, 1, "failed rows must be held, not dropped")
	assert.Nil(t, w.paymentBatch, "a burned batch must be dropped")
	assert.Empty(t, w.paymentAcks)
	require.Len(t, mustPending(t, w, w.streamPayments), 1, "un-sent row keeps its message pending")

	// 仓库恢复：held 行重派生进新批次并成功落库。
	getMockConn(t, w).batch = good
	require.NoError(t, w.flushPaymentBatch(ctx))
	assert.Empty(t, w.paymentHeld)
	assert.Empty(t, w.paymentAcks)
	assert.Empty(t, mustPending(t, w, w.streamPayments), "delivered row ACKs its message")
	require.Len(t, good.rows, 1, "the held row lands exactly once")
	rv := w.revAgg["2025-06-15|g1|e1"]
	require.NotNil(t, rv)
	assert.EqualValues(t, 100, rv.revenue, "revenue commits exactly once after recovery")
	assert.False(t, mr.Exists("analytics:transfer:dead:"+w.streamPayments+":"+id))
}

// 验收（DATA-053）：连续 maxPendingRetries 轮 Send 失败的记录经幂等转移
// 进入死信（reason=max_flush_retries_exceeded），源消息被 ACK（不再循环
// 回收），held 清空。
func TestFlushHold_DeadLettersAfterSustainedFailures(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	id := data053Pending(t, w, data053PaymentPayload)
	w.holdEventData(w.streamPayments, id, data053PaymentPayload)

	for i := 0; i < maxPendingRetries; i++ {
		w.paymentBatch = &mockBatch{sendErr: errors.New("clickhouse down")}
		w.paymentBatchRows = 1
		w.paymentAcks = []ackRef{{stream: w.streamPayments, id: id}}
		require.Error(t, w.flushPaymentBatch(ctx))
	}
	assert.Empty(t, w.paymentHeld, "threshold record leaves the held list")
	assert.Empty(t, mustPending(t, w, w.streamPayments), "the dead transfer ACKs the source")
	require.EqualValues(t, 1, w.rdb.XLen(ctx, w.deadPaymentsStream).Val())
	res, err := w.rdb.XRange(ctx, w.deadPaymentsStream, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "max_flush_retries_exceeded", res[0].Values["reason"])
	assert.Equal(t, data053PaymentPayload, res[0].Values["original_data"])
	assert.True(t, mr.Exists("analytics:transfer:dead:"+w.streamPayments+":"+id))
}

// 验收（DATA-053）：held 载荷再也解不出行的记录立即死信隔离
// （reason=retry_decode_failed）——永久坏记录不占用重试轮次。
func TestFlushHold_DeadLettersUndecodablePayload(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	require.NoError(t, w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "$").Err())
	id := w.rdb.XAdd(ctx, &redis.XAddArgs{Stream: w.streamEvents, Values: map[string]any{"data": `{"event":"click"}`}}).Val()
	_, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: w.group, Consumer: w.consumer, Streams: []string{w.streamEvents, ">"},
	}).Result()
	require.NoError(t, err)

	w.eventHeld = []heldRow{{ref: ackRef{stream: w.streamEvents, id: id}, data: "{truncated"}}
	require.NoError(t, w.flushEventBatch(ctx))
	assert.Empty(t, w.eventHeld)
	assert.Empty(t, mustPending(t, w, w.streamEvents))
	require.EqualValues(t, 1, w.rdb.XLen(ctx, w.deadEventsStream).Val())
	res, err := w.rdb.XRange(ctx, w.deadEventsStream, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "retry_decode_failed", res[0].Values["reason"])
}

// 验收（DATA-053）：同一源消息的两份 ref（回收重投竞态下同一 Send 中出现）
// 营收只入账一次。
func TestCommitRevenue_DuplicateRefsCommitOnce(t *testing.T) {
	w, _ := testWorker(t)
	ref := ackRef{stream: w.streamPayments, id: "1-1", revKey: "2025-06-15|g1|e1", rev: &revRow{revenue: 100}}
	w.commitRevenue([]ackRef{ref, {stream: w.streamPayments, id: "1-1", revKey: "2025-06-15|g1|e1", rev: &revRow{revenue: 100}}})
	rv := w.revAgg["2025-06-15|g1|e1"]
	require.NotNil(t, rv)
	assert.EqualValues(t, 100, rv.revenue)
}

// 验收（DATA-053）：消息走 requeue 离开管道时，其内存 held 行必须被丢弃
// ——流上的重投副本取代它，否则记录会写两次（held 一行 + 重投副本一行）。
func TestRequeueDropsHeldRow(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	id := data053Pending(t, w, data053PaymentPayload)
	w.holdEventData(w.streamPayments, id, data053PaymentPayload)

	good := &mockBatch{}
	getMockConn(t, w).batch = good
	w.clickBatchSize = 0
	w.paymentBatch = &mockBatch{sendErr: errors.New("clickhouse down")}
	w.paymentBatchRows = 1
	w.paymentAcks = []ackRef{{stream: w.streamPayments, id: id}}
	require.Error(t, w.flushPaymentBatch(ctx))
	require.Len(t, w.paymentHeld, 1)

	// 同一消息的再处理走上不可解析金额的错误分支 → requeue（原子脚本复制
	// 重投副本并 ACK 源消息）→ 书记账本连同 held 行一起遗忘。
	bad := redis.XMessage{ID: id, Values: map[string]any{"data": []byte(`{"ts":"2025-06-15T10:30:00Z","game_id":"g1","env":"e1","status":"success","amount_cents":"NaN","order_id":"o1"}`)}}
	require.NoError(t, w.processMessage(ctx, w.streamPayments, bad))
	assert.Empty(t, w.paymentHeld, "the stream retry copy supersedes the held row")
	assert.Empty(t, mustPending(t, w, w.streamPayments))

	// 后续刷新不重放 held 行：仓库只应收到重投副本处理后的那一行。
	require.NoError(t, w.flushPaymentBatch(ctx))
	require.NoError(t, w.flushPaymentBatch(ctx))
	assert.Empty(t, good.rows, "no second copy of the requeued record")
}
