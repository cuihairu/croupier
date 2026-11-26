package mq

import (
	"context"
	"errors"
	"testing"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTrimQueue builds a miniredis-backed queue with a small cap. miniredis's
// XPENDING/XTRIM MINID/XINFO GROUPS are faithful (unlike its lag), so the
// DATA-048 retention assertions run real flows without seams.
func newTrimQueue(t *testing.T, maxLen int64, approx bool) (*redisQueue, context.Context) {
	t.Helper()
	mr := miniredis.RunT(t)
	q, err := NewRedis("redis://"+mr.Addr(), "events", "payments", maxLen, approx)
	require.NoError(t, err)
	_ = mr
	return q.(*redisQueue), context.Background()
}

func publishN(t *testing.T, q *redisQueue, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		require.NoError(t, q.PublishEvent(map[string]any{"i": i}))
	}
}

// 验收：worker 停机（组不存在），积压超过旧长度上限时所有事件留存；
// 恢复后（建组消费）全部可重放——旧的 XADD MaxLen 盲裁剪会把它们淘汰。
func TestXAddKeepsUndeliveredAboveCap(t *testing.T) {
	q, ctx := newTrimQueue(t, 10, true)
	publishN(t, q, 25)
	retained, err := q.cli.XLen(ctx, "events").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(25), retained, "undelivered entries must survive the cap")

	// 恢复：worker 首次启动建组（DATA-023，从流头开始），全部可重放。
	require.NoError(t, q.cli.XGroupCreate(ctx, "events", "analytics-worker", "0").Err())
	msgs, err := q.cli.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: "analytics-worker", Consumer: "c", Streams: []string{"events", ">"},
	}).Result()
	require.NoError(t, err)
	assert.Len(t, msgs[0].Messages, 25, "every event published during the outage must be replayable")
}

// 验收：已派发未 ACK 的事件在裁剪后仍可重放——裁剪边界只落在
// 「已派发且已 ACK」的前缀上，PEL 条目与其后的未派发条目全部留存。
func TestXAddTrimsOnlyFullyConsumedPrefix(t *testing.T) {
	q, ctx := newTrimQueue(t, 10, true)
	publishN(t, q, 25)
	require.NoError(t, q.cli.XGroupCreate(ctx, "events", "analytics-worker", "0").Err())
	msgs, err := q.cli.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: "analytics-worker", Consumer: "c", Streams: []string{"events", ">"},
	}).Result()
	require.NoError(t, err)
	require.Len(t, msgs[0].Messages, 25)
	var acked []string
	for i := 0; i < 15; i++ {
		acked = append(acked, msgs[0].Messages[i].ID)
	}
	require.NoError(t, q.cli.XAck(ctx, "events", "analytics-worker", acked...).Err())

	// 再次发布使长度越限，触发清理；此刻 oldest pending 与其后的未派发条目必须存活。
	publishN(t, q, 5)
	retained, err := q.cli.XLen(ctx, "events").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(15), retained, "only the delivered-and-ACKed prefix (15) is removed")

	first, err := q.cli.XRange(ctx, "events", "-", "+").Result()
	require.NoError(t, err)
	require.NotEmpty(t, first)
	assert.Equal(t, msgs[0].Messages[15].ID, first[0].ID, "the oldest pending entry must survive at the stream head")

	pe, err := q.cli.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: "events", Group: "analytics-worker", Start: "-", End: "+", Count: 100,
	}).Result()
	require.NoError(t, err)
	assert.Len(t, pe, 10, "un-ACKed entries must all be replayable after trimming")
}

// 全部消费并 ACK 后清理可推进到 last-delivered 之后；低于上限时不触发读取。
func TestXAddTrimsFullyConsumedAndSkipsBelowCap(t *testing.T) {
	q, ctx := newTrimQueue(t, 10, true)
	publishN(t, q, 25)
	require.NoError(t, q.cli.XGroupCreate(ctx, "events", "analytics-worker", "0").Err())
	// 单次读全量：miniredis 对已排空流的 XREADGROUP ">" 会阻塞等待新条目
	// （真实 Redis 无 BLOCK 时立即返回 nil），读到空为止的循环在测试里挂死。
	msgs, err := q.cli.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: "analytics-worker", Consumer: "c", Streams: []string{"events", ">"},
	}).Result()
	require.NoError(t, err)
	require.Len(t, msgs[0].Messages, 25)
	var ids []string
	for _, m := range msgs[0].Messages {
		ids = append(ids, m.ID)
	}
	require.NoError(t, q.cli.XAck(ctx, "events", "analytics-worker", ids...).Err())
	publishN(t, q, 3)
	retained, err := q.cli.XLen(ctx, "events").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(3), retained, "fully consumed prefix trims up to last-delivered+1")

	// 低于上限：不触发进度读取，留存不动。
	publishN(t, q, 2)
	retained, err = q.cli.XLen(ctx, "events").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(5), retained)
}

// 进度读数失败 = 未知 = 不清理（fail-safe）；精确策略分支同样只清已消费前缀。
func TestXAddNoTrimWhenProgressUnknownAndExactStrategy(t *testing.T) {
	q, ctx := newTrimQueue(t, 10, false)
	publishN(t, q, 25)
	q.groupsFn = func(context.Context, string) ([]redis.XInfoGroup, error) {
		return nil, errors.New("groups unavailable")
	}
	publishN(t, q, 5)
	retained, err := q.cli.XLen(ctx, "events").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(30), retained, "unknown progress must retain everything (fail-safe)")

	// 恢复读数且用精确策略：同样只清已消费前缀。
	require.NoError(t, q.cli.XGroupCreate(ctx, "events", "analytics-worker", "0").Err())
	msgs, err := q.cli.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: "analytics-worker", Consumer: "c", Streams: []string{"events", ">"},
	}).Result()
	require.NoError(t, err)
	var ids []string
	for _, m := range msgs[0].Messages {
		ids = append(ids, m.ID)
	}
	require.NoError(t, q.cli.XAck(ctx, "events", "analytics-worker", ids...).Err())
	q.groupsFn = defaultGroupsFn(q.cli)
	publishN(t, q, 1)
	retained, err = q.cli.XLen(ctx, "events").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), retained, "exact strategy also trims only the consumed prefix")
}

// ID 排序比较器：跨毫秒与同毫秒不同序号。
func TestStreamIDBefore(t *testing.T) {
	assert.True(t, streamIDBefore("1-0", "2-0"))
	assert.True(t, streamIDBefore("2-1", "2-2"))
	assert.False(t, streamIDBefore("2-0", "1-0"))
	assert.False(t, streamIDBefore("2-2", "2-2"))
	assert.False(t, streamIDBefore("bad", "3-0"), "unparseable compares as not-before (fail-safe)")
}
