package mq

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestBacklogQueue(t *testing.T) (*redisQueue, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	q, err := NewRedis("redis://"+mr.Addr(), "events", "payments", 1000, true)
	require.NoError(t, err)
	return q.(*redisQueue), mr
}

// DATA-047：拆分报告——retained 是物理长度（XLEN），积压 = 组进度的
// 未派发 lag + 已派发未 ACK。
func TestRedisQueueBacklogSplit(t *testing.T) {
	rq, _ := newTestBacklogQueue(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		require.NoError(t, rq.PublishEvent(map[string]any{"i": i}))
	}
	// miniredis 的 lag 实现不忠实于真实 Redis，组读数经 seam 注入。
	rq.groupsFn = func(_ context.Context, stream string) ([]redis.XInfoGroup, error) {
		if stream == "events" {
			return []redis.XInfoGroup{{Name: "analytics-worker", Pending: 2, Lag: 1}}, nil
		}
		return nil, nil // payments：尚无消费组
	}
	bl, err := rq.Backlog(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(5), bl.Events.Retained)
	require.NotNil(t, bl.Events.Undelivered)
	assert.Equal(t, int64(1), *bl.Events.Undelivered)
	require.NotNil(t, bl.Events.PendingAck)
	assert.Equal(t, int64(2), *bl.Events.PendingAck)

	// 无组分支：全部留存按未派发计，无已派发未 ACK
	assert.Equal(t, int64(0), bl.Payments.Retained)
	require.NotNil(t, bl.Payments.Undelivered)
	assert.Equal(t, int64(0), *bl.Payments.Undelivered)
	require.NotNil(t, bl.Payments.PendingAck)
	assert.Equal(t, int64(0), *bl.Payments.PendingAck)
}

// 验收：同一流全部消费并 ACK 后积压回落，即使物理长度不变（裁剪策略
// 继续保留已消费条目，XLEN 不代表积压）。
func TestRedisQueueBacklogFallsAfterFullConsume(t *testing.T) {
	rq, _ := newTestBacklogQueue(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		require.NoError(t, rq.PublishEvent(map[string]any{"i": i}))
	}
	// 真实 Redis 语义：全部投递并 ACK 后 lag/pending 归零，XLEN 保持 5。
	rq.groupsFn = func(context.Context, string) ([]redis.XInfoGroup, error) {
		return []redis.XInfoGroup{{Name: "analytics-worker", Pending: 0, Lag: 0}}, nil
	}
	bl, err := rq.Backlog(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(5), bl.Events.Retained) // 物理长度不变
	require.NotNil(t, bl.Events.Undelivered)
	assert.Equal(t, int64(0), *bl.Events.Undelivered) // 积压回落为 0
	require.NotNil(t, bl.Events.PendingAck)
	assert.Equal(t, int64(0), *bl.Events.PendingAck)
}

func TestRedisQueueBacklogUnknowns(t *testing.T) {
	rq, _ := newTestBacklogQueue(t)
	ctx := context.Background()
	require.NoError(t, rq.PublishEvent(map[string]any{"i": 1}))

	// lag = -1（Redis < 7 无法确定）→ 未派发 unknown，未 ACK 仍有效
	rq.groupsFn = func(context.Context, string) ([]redis.XInfoGroup, error) {
		return []redis.XInfoGroup{{Name: "analytics-worker", Pending: 3, Lag: -1}}, nil
	}
	bl, err := rq.Backlog(ctx)
	require.NoError(t, err)
	assert.Nil(t, bl.Events.Undelivered)
	require.NotNil(t, bl.Events.PendingAck)
	assert.Equal(t, int64(3), *bl.Events.PendingAck)

	// 组信息读取失败 → 拆分读数 unknown，retained 仍有效
	rq.groupsFn = func(context.Context, string) ([]redis.XInfoGroup, error) {
		return nil, context.DeadlineExceeded
	}
	bl, err = rq.Backlog(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), bl.Events.Retained)
	assert.Nil(t, bl.Events.Undelivered)
	assert.Nil(t, bl.Events.PendingAck)
}

func TestRedisQueueBacklogXLenFailure(t *testing.T) {
	rq, mr := newTestBacklogQueue(t)
	rq.groupsFn = func(context.Context, string) ([]redis.XInfoGroup, error) { return nil, nil }
	require.NoError(t, rq.cli.Close())
	mr.Close()
	_, err := rq.Backlog(context.Background())
	assert.Error(t, err)
}

// noop 的零是真实读数（从无入队），不是占位（DATA-047）。
func TestNoopBacklogTrueZeros(t *testing.T) {
	bl, err := NewNoop().Backlog(context.Background())
	require.NoError(t, err)
	for _, sb := range []StreamBacklog{bl.Events, bl.Payments} {
		assert.Equal(t, int64(0), sb.Retained)
		require.NotNil(t, sb.Undelivered)
		assert.Equal(t, int64(0), *sb.Undelivered)
		require.NotNil(t, sb.PendingAck)
		assert.Equal(t, int64(0), *sb.PendingAck)
	}
}
