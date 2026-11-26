// Coverage targets: the zero-value methods of kafkaQueue in the mq package, the
// success and WRONGTYPE error paths of PendingEvents/PendingPayments (miniredis),
// and the newRedisFromEnv environment-variable parsing branches.
package mq

import (
	"context"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKafkaQueue_ZeroValueMethods(t *testing.T) {
	q := &kafkaQueue{}

	assert.NoError(t, q.PublishEvent(map[string]any{"k": "v"}))
	assert.NoError(t, q.PublishPayment(map[string]any{"k": "v"}))
	assert.NoError(t, q.Close())

	// DATA-047: kafkaQueue reports no fake pending/backlog readings.
	_, hasBacklog := any(q).(BacklogReporter)
	assert.False(t, hasBacklog)
	_, hasPending := any(q).(interface {
		PendingEvents() (int64, error)
	})
	assert.False(t, hasPending)
}

// withMQEnv sets an environment variable and restores it after the test.
func withMQEnv(t *testing.T, key, value string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	if value == "" {
		_ = os.Unsetenv(key)
	} else {
		_ = os.Setenv(key, value)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func TestNewRedisFromEnv_ParsesOverrides(t *testing.T) {
	withMQEnv(t, "REDIS_URL", "redis://127.0.0.1:6380/2")
	withMQEnv(t, "ANALYTICS_REDIS_STREAM_EVENTS", "ev")
	withMQEnv(t, "ANALYTICS_REDIS_STREAM_PAYMENTS", "pay")
	withMQEnv(t, "ANALYTICS_REDIS_MAXLEN", "5000")
	withMQEnv(t, "ANALYTICS_REDIS_MAXLEN_APPROX", "0")

	q, err := newRedisFromEnv()
	require.NoError(t, err)
	rq, ok := q.(*redisQueue)
	require.True(t, ok, "expected *redisQueue, got %T", q)
	assert.Equal(t, "ev", rq.streamEvents)
	assert.Equal(t, "pay", rq.streamPayments)
	assert.Equal(t, int64(5000), rq.maxLen)
	assert.False(t, rq.maxLenApprox)
	require.NoError(t, rq.Close())
}

func TestNewRedisFromEnv_InvalidMaxLenAndApproxTrue(t *testing.T) {
	withMQEnv(t, "REDIS_URL", "")
	withMQEnv(t, "ANALYTICS_REDIS_STREAM_EVENTS", "")
	withMQEnv(t, "ANALYTICS_REDIS_STREAM_PAYMENTS", "")
	// An invalid maxlen keeps the default; approx=1 -> true.
	withMQEnv(t, "ANALYTICS_REDIS_MAXLEN", "not-a-number")
	withMQEnv(t, "ANALYTICS_REDIS_MAXLEN_APPROX", "1")

	q, err := newRedisFromEnv()
	require.NoError(t, err)
	rq, ok := q.(*redisQueue)
	require.True(t, ok)
	assert.Equal(t, int64(1000000), rq.maxLen)
	assert.True(t, rq.maxLenApprox)
	assert.Equal(t, "analytics:events", rq.streamEvents)
	assert.Equal(t, "analytics:payments", rq.streamPayments)
	require.NoError(t, rq.Close())
}

func TestRedisQueue_BacklogAgainstMiniredis(t *testing.T) {
	mr := miniredis.RunT(t)
	q, _ := NewRedis("redis://"+mr.Addr()+"/0", "events", "payments", 1000, true)
	rq, ok := q.(*redisQueue)
	require.True(t, ok)
	// miniredis 的 lag 不忠实于真实 Redis：无组语义（DATA-047 无组分支）
	// 经真实 XLEN + 空组表验证，已消费场景见 mq_backlog_test.go 的 seam 注入。

	t.Run("empty stream keeps retained and unknown progress", func(t *testing.T) {
		// 空流尚不存在，XINFO GROUPS 无从读起（真实 Redis 报错）：
		// retained 有效为 0，组进度按 unknown 报告。
		bl, err := rq.Backlog(context.Background())
		require.NoError(t, err)
		assert.Zero(t, bl.Events.Retained)
		assert.Nil(t, bl.Events.Undelivered)
		assert.Nil(t, bl.Events.PendingAck)
	})

	t.Run("published events are undelivered without a group", func(t *testing.T) {
		require.NoError(t, rq.PublishEvent(map[string]any{"id": "e1"}))
		require.NoError(t, rq.PublishEvent(map[string]any{"id": "e2"}))
		require.NoError(t, rq.PublishPayment(map[string]any{"order": "p1"}))

		bl, err := rq.Backlog(context.Background())
		require.NoError(t, err)
		assert.EqualValues(t, 2, bl.Events.Retained)
		require.NotNil(t, bl.Events.Undelivered)
		assert.EqualValues(t, 2, *bl.Events.Undelivered)
		require.NotNil(t, bl.Events.PendingAck)
		assert.Zero(t, *bl.Events.PendingAck)
	})

	t.Run("server down returns error", func(t *testing.T) {
		down := miniredis.RunT(t)
		dq, _ := NewRedis("redis://"+down.Addr()+"/0", "events", "payments", 0, false)
		drq, ok := dq.(*redisQueue)
		require.True(t, ok)
		down.Close()

		_, err := drq.Backlog(context.Background())
		require.Error(t, err)
		require.NoError(t, drq.Close())
	})

	require.NoError(t, rq.Close())
}
