package worker

import (
	"context"
	"errors"
	"strings"
	"testing"

	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// appendFailBatch fails Append so processMessage takes the real requeue
// branch (DATA-051): the mutated retry payload is XAdd'ed back to the source
// stream exactly as production would, and the test reads those bytes back.
type appendFailBatch struct {
	mockBatch
}

func (b *appendFailBatch) Append(v ...any) error { return errors.New("append boom") }

// 验收（DATA-051）：同一消息在 Send 后 ACK 失败（原始字节重投）、重启回收
// （PEL 重投）及重新入队（retry_count 回写后字节串已不同）三种形态下，
// 无上游 event_id 的逻辑事件仍派生同一事件 ID，ClickHouse 自然键可去重。
func TestProcessMessage_StableEventIDAcrossRedeliveryAndRequeue(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()

	original := `{"game_id":"g","env":"prod","ts":"2026-11-26T03:02:55Z","event":"login",` +
		`"user_id":"u1","session_id":"s1","props":{"b":9007199254740993,"a":"x"}}`
	msg := redis.XMessage{ID: "1-1", Values: map[string]any{"data": original}}

	// 基线：原始字节首次处理（模拟首次写入未 ACK 后的各种重投起点）。
	w.eventBatch = &mockBatch{}
	require.NoError(t, w.processMessage(ctx, w.streamEvents, msg))
	base := rowEventID(t, w, 0)
	require.True(t, isUUID(base))

	// 第一轮真实重投：Append 失败 → processMessage 走 requeue 分支回写源流。
	w.eventBatch = &appendFailBatch{}
	require.NoError(t, w.processMessage(ctx, w.streamEvents, msg), "requeue path must ACK after a successful XAdd")

	res, err := w.rdb.XRange(ctx, w.streamEvents, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, res, 1, "the requeue XAdd is the stream's only entry (synthetic original was never enqueued)")
	requeued1 := redis.XMessage{ID: res[0].ID, Values: res[0].Values}
	data1 := string(fmtAny(requeued1.Values["data"]))
	require.NotEqual(t, original, data1, "requeue must actually mutate the wire bytes (retry_count)")
	require.Contains(t, data1, `"retry_count":1`)

	// 重投字节再处理：同一逻辑事件 → 同一事件 ID。
	w.eventBatch = &mockBatch{}
	require.NoError(t, w.processMessage(ctx, w.streamEvents, requeued1))
	assert.Equal(t, base, rowEventID(t, w, 0), "requeued (retry-mutated) payload must keep the derived event id")

	// 第二轮重投（retry_count=2）仍收敛到同一 ID——贯穿持久重试。
	w.eventBatch = &appendFailBatch{}
	require.NoError(t, w.processMessage(ctx, w.streamEvents, requeued1))
	res, err = w.rdb.XRange(ctx, w.streamEvents, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, res, 2, "two requeued entries")
	requeued2 := redis.XMessage{ID: res[1].ID, Values: res[1].Values}
	data2 := string(fmtAny(requeued2.Values["data"]))
	require.Contains(t, data2, `"retry_count":2`)

	w.eventBatch = &mockBatch{}
	require.NoError(t, w.processMessage(ctx, w.streamEvents, requeued2))
	assert.Equal(t, base, rowEventID(t, w, 0), "id stays stable across repeated requeues")

	// 原始字节重投（Send 后 ACK 失败 / 重启回收的 PEL 重投形态）→ 同一 ID。
	w.eventBatch = &mockBatch{}
	require.NoError(t, w.processMessage(ctx, w.streamEvents, msg))
	assert.Equal(t, base, rowEventID(t, w, 0), "raw redelivery of the original bytes must keep the derived event id")

	// 不同逻辑事件不得共享该 ID（重投收敛不等于无差别合并）。
	other := strings.Replace(original, `"user_id":"u1"`, `"user_id":"u2"`, 1)
	w.eventBatch = &mockBatch{}
	require.NoError(t, w.processMessage(ctx, w.streamEvents, redis.XMessage{ID: "1-2", Values: map[string]any{"data": other}}))
	assert.NotEqual(t, base, rowEventID(t, w, 0), "a different logical event must derive a different id")
}
