package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// maxRequests=0：Allow 恒拒绝且空窗口的 RetryAfter 精确为 0，命中
// WaitWithTimeout 的 waitTime<=0 → 10ms 兜底，最终因 deadline 超限返回
// ErrRateLimitExceeded。
func TestSlidingWindowWaitWithTimeoutNonPositiveRetryAfter(t *testing.T) {
	sw := NewSlidingWindowLimiter(0, 200*time.Millisecond)
	defer func() { _ = sw.Close() }()

	start := time.Now()
	err := sw.WaitWithTimeout(context.Background(), "zero-limit", 120*time.Millisecond)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRateLimitExceeded), "got %v", err)
	assert.Less(t, time.Since(start), 5*time.Second, "应在 deadline 附近返回而非长眠")
}
