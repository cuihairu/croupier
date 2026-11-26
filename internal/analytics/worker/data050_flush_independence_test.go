package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 验收（DATA-050）：事件 Send 持续失败且支付可正常写入时，flushBatches
// 仍推进支付流——支付落仓、ref ACK、营收入账、checkpoint 前进；事件批次
// 被丢弃、消息保留 pending 待回收重投；两条流的失败在返回错误中分别可见。
func TestFlushBatches_IndependentStreamProgress(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	require.NoError(t, w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "$").Err())
	require.NoError(t, w.rdb.XGroupCreateMkStream(ctx, w.streamPayments, w.group, "$").Err())

	// 各领一条消息进 PEL（deferred-ack 语义可观测）。
	evID := w.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: w.streamEvents, Values: map[string]any{"data": `{"event":"x"}`},
	}).Val()
	payID := w.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: w.streamPayments, Values: map[string]any{"data": `{}`},
	}).Val()
	evRes, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: w.group, Consumer: w.consumer, Streams: []string{w.streamEvents, ">"},
	}).Result()
	require.NoError(t, err)
	require.Len(t, evRes[0].Messages, 1)
	payRes, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: w.group, Consumer: w.consumer, Streams: []string{w.streamPayments, ">"},
	}).Result()
	require.NoError(t, err)
	require.Len(t, payRes[0].Messages, 1)

	require.NoError(t, w.appendEventRow(ctx, evID, "ts", "g1", "prod"))
	// 支付行带营收增量（args[0..2,5,7]：ts/game/env/amount/status）。
	require.NoError(t, w.appendPaymentRow(ctx, payID,
		"2025-06-15T10:30:00Z", "g1", "prod", "u1", "o1", uint64(100), "USD", "success",
		"chan", "ios", "CN", "region", "city", "p1", "reason"))

	// 事件 Send 持续失败：换入失败批（eventAcks 引用仍在）。
	w.eventBatch = &mockBatch{sendErr: errors.New("events send boom")}

	flushErr := w.flushBatches(ctx)
	require.Error(t, flushErr)
	assert.Contains(t, flushErr.Error(), "events send boom", "event failure must be reported")

	// 支付流不受牵连：批次落仓、ref ACK、营收入账、checkpoint 前进。
	payPending, err := w.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: w.streamPayments, Group: w.group, Start: "-", End: "+", Count: 100,
	}).Result()
	require.NoError(t, err)
	assert.Empty(t, payPending, "payments must still be ACKed despite the event Send failure")
	n, err := w.rdb.Exists(ctx, w.checkpointKey(w.streamPayments)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "payment checkpoint must advance")
	assert.Len(t, w.revAgg, 1, "payment revenue delta must be committed")

	// 事件流保留现场：消息仍 pending（批次已丢弃，回收路径重投）。
	evPending, err := w.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: w.streamEvents, Group: w.group, Start: "-", End: "+", Count: 100,
	}).Result()
	require.NoError(t, err)
	assert.Len(t, evPending, 1, "event messages must stay pending for reclaim")
}

// 双流同时失败：两条失败都在联合错误中分别报告。
func TestFlushBatches_ReportsBothFailures(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	w.eventBatch = &mockBatch{sendErr: errors.New("events send boom")}
	w.eventBatchRows = 1
	w.paymentBatch = &mockBatch{sendErr: errors.New("payments send boom")}
	w.paymentBatchRows = 1

	err := w.flushBatches(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "events send boom")
	assert.Contains(t, err.Error(), "payments send boom", "both stream failures must be reported separately")
}
