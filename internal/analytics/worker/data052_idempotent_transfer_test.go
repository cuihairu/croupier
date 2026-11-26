package worker

import (
	"context"
	"errors"
	"sync"
	"testing"

	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustPending(t *testing.T, w *Worker, stream string) []redis.XPendingExt {
	t.Helper()
	pe, err := w.rdb.XPendingExt(ctxBackground(), &redis.XPendingExtArgs{
		Stream: stream, Group: w.group, Start: "-", End: "+", Count: 100,
	}).Result()
	require.NoError(t, err)
	return pe
}

func ctxBackground() context.Context { return context.Background() }

// 验收（DATA-052）：目标流写入成功而源 ACK 失败（或客户端超时结果未知）时，
// 再次处理同一源消息不会复制死信——原子脚本落 marker 后重放只补 ACK。
func TestSendToDeadLetter_IdempotentAcrossReprocessing(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	msg := redis.XMessage{ID: "7-1", Values: map[string]any{"data": "not-json", "retry_count": "2"}}

	// 首次转移：死信 1 条，marker 建立。
	require.NoError(t, w.sendToDeadLetter(ctx, w.streamEvents, msg, "invalid_json", "boom"))
	require.EqualValues(t, 1, w.rdb.XLen(ctx, w.deadEventsStream).Val())
	require.True(t, mr.Exists("analytics:transfer:dead:"+w.streamEvents+":7-1"))

	// 再次处理同一源消息（模拟 ACK 丢失后的回收重投）：不复制死信。
	require.NoError(t, w.sendToDeadLetter(ctx, w.streamEvents, msg, "invalid_json", "boom"))
	assert.EqualValues(t, 1, w.rdb.XLen(ctx, w.deadEventsStream).Val(), "reprocessing must not duplicate the dead letter")

	// 第二个 worker 实例（共享 Redis）重放同样收敛。
	w2 := testWorkerOn(t, mr)
	require.NoError(t, w2.sendToDeadLetter(ctx, w.streamEvents, msg, "invalid_json", "boom"))
	assert.EqualValues(t, 1, w.rdb.XLen(ctx, w.deadEventsStream).Val())
}

// 验收（DATA-052）：重投副本与源 ACK 同为原子脚本——Send 失败重投后，
// 同一源消息再次重投不复制重试副本，retry 计数不虚增。
func TestProcessMessage_RequeueIdempotentAcrossReprocessing(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	require.NoError(t, w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "$").Err())

	id := w.rdb.XAdd(ctx, &redis.XAddArgs{Stream: w.streamEvents, Values: map[string]any{
		"data": `{"event":"click","game_id":"g1","env":"prod","user_id":"u1","ts":"2025-06-15T10:00:00Z"}`,
	}}).Val()
	claimed, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: w.group, Consumer: w.consumer, Streams: []string{w.streamEvents, ">"},
	}).Result()
	require.NoError(t, err)
	msg := claimed[0].Messages[0]
	require.Equal(t, id, msg.ID)

	// 首次重投：批次 Send 失败 → 走真实 requeue 分支。
	w.clickBatchSize = 1
	w.eventBatch = &mockBatch{sendErr: errors.New("send boom")}
	require.NoError(t, w.processMessage(ctx, w.streamEvents, msg))
	require.EqualValues(t, 2, w.rdb.XLen(ctx, w.streamEvents).Val(), "original + one retry copy")
	require.Empty(t, mustPending(t, w, w.streamEvents), "original must be ACKed by the script")

	// 同一源消息再次重投（ACK 丢失重投形态）：不复制重试副本。
	w.eventBatch = &mockBatch{sendErr: errors.New("send boom")}
	require.NoError(t, w.processMessage(ctx, w.streamEvents, msg))
	assert.EqualValues(t, 2, w.rdb.XLen(ctx, w.streamEvents).Val(), "reprocessing must not duplicate the retry copy")

	// 唯一副本载荷 retry_count=1：跳过路径未虚增计数。
	copyMsg, err := w.rdb.XRange(ctx, w.streamEvents, id, "+").Result()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(copyMsg), 2)
	last := copyMsg[len(copyMsg)-1]
	assert.Contains(t, fmtAny(last.Values["data"]), `"retry_count":1`)
}

// 多 worker 并发转移：Redis 单线程脚本注册表串行化，死信恰好一条。
func TestTransfer_ConcurrentWorkersLandSingleCopy(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	w2 := testWorkerOn(t, mr)

	msg := redis.XMessage{ID: "9-3", Values: map[string]any{"data": "not-json"}}
	require.NoError(t, w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "$").Err())
	var wg sync.WaitGroup
	clients := []*Worker{w, w2, w, w2}
	for _, cw := range clients {
		wg.Add(1)
		go func(cw *Worker) {
			defer wg.Done()
			_ = cw.sendToDeadLetter(ctx, w.streamEvents, msg, "invalid_json", "boom")
		}(cw)
	}
	wg.Wait()
	assert.EqualValues(t, 1, w.rdb.XLen(ctx, w.deadEventsStream).Val(), "concurrent transfers must land a single dead letter")
	assert.Empty(t, mustPending(t, w, w.streamEvents))
}

// 目标写失败（XADD WRONGTYPE 中止脚本）：不落 marker、不 ACK——源消息可继续
// 回收，下次重投重新尝试转移（目标未写入 ⇒ 源不丢）。
func TestSendToDeadLetter_FailedTargetWriteLeavesNoMarkerAndNoAck(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	require.NoError(t, w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "$").Err())
	id := w.rdb.XAdd(ctx, &redis.XAddArgs{Stream: w.streamEvents, Values: map[string]any{
		"data": `{"event":"click"}`,
	}}).Val()
	claimed, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: w.group, Consumer: w.consumer, Streams: []string{w.streamEvents, ">"},
	}).Result()
	require.NoError(t, err)
	msg := claimed[0].Messages[0]
	require.Equal(t, id, msg.ID)

	// 死信流键被占成 string 类型 → 脚本在 XADD 处 WRONGTYPE 中止。
	require.NoError(t, w.rdb.Set(ctx, w.deadEventsStream, "occupied", 0).Err())
	injectErr := w.sendToDeadLetter(ctx, w.streamEvents, msg, "invalid_json", "boom")
	require.Error(t, injectErr)

	assert.False(t, mr.Exists("analytics:transfer:dead:"+w.streamEvents+":"+msg.ID), "aborted script must not leave a marker")
	assert.Len(t, mustPending(t, w, w.streamEvents), 1, "message must stay pending for reclaim retry")
}
