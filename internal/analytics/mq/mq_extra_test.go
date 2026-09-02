// 覆盖目标：mq 包 kafkaQueue 零值方法、PendingEvents/PendingPayments 的
// 成功与 WRONGTYPE 错误路径（miniredis）、newRedisFromEnv 环境变量解析分支。
package mq

import (
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

	n, err := q.PendingEvents()
	require.NoError(t, err)
	assert.Zero(t, n)

	n, err = q.PendingPayments()
	require.NoError(t, err)
	assert.Zero(t, n)
}

// withEnv 设置环境变量并在测试结束后恢复。
func withMQEnv(t *testing.T, key, value string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	if value == "" {
		os.Unsetenv(key)
	} else {
		os.Setenv(key, value)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv(key, prev)
		} else {
			os.Unsetenv(key)
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
	// 非法 maxlen 保持默认；approx=1 → true。
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

func TestRedisQueue_PendingAgainstMiniredis(t *testing.T) {
	mr := miniredis.RunT(t)
	q := NewRedis("redis://"+mr.Addr()+"/0", "events", "payments", 1000, true)
	rq, ok := q.(*redisQueue)
	require.True(t, ok)

	t.Run("empty streams report zero", func(t *testing.T) {
		n, err := rq.PendingEvents()
		require.NoError(t, err)
		assert.Zero(t, n)
		n, err = rq.PendingPayments()
		require.NoError(t, err)
		assert.Zero(t, n)
	})

	t.Run("published events counted", func(t *testing.T) {
		require.NoError(t, rq.PublishEvent(map[string]any{"id": "e1"}))
		require.NoError(t, rq.PublishEvent(map[string]any{"id": "e2"}))
		require.NoError(t, rq.PublishPayment(map[string]any{"order": "p1"}))

		n, err := rq.PendingEvents()
		require.NoError(t, err)
		assert.EqualValues(t, 2, n)
		n, err = rq.PendingPayments()
		require.NoError(t, err)
		assert.EqualValues(t, 1, n)
	})

	t.Run("server down returns error", func(t *testing.T) {
		down := miniredis.RunT(t)
		dq := NewRedis("redis://"+down.Addr()+"/0", "events", "payments", 0, false)
		drq, ok := dq.(*redisQueue)
		require.True(t, ok)
		down.Close()

		_, err := drq.PendingEvents()
		require.Error(t, err)
		_, err = drq.PendingPayments()
		require.Error(t, err)
		require.NoError(t, drq.Close())
	})

	require.NoError(t, rq.Close())
}
