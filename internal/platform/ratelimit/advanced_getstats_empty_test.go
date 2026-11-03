package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// C-026：GetStats 对空 timestamps 窗口必须返回零值而非 index -1 panic。
// 空窗口真实可达：AllowN 超限（n>maxRequests）建窗口但不写入 timestamps
// （拒绝路径无 append），或窗口时间戳全部 prune 后超限；cleanup 删空窗口前
// 的窗口内 GetStats 即触发。
func TestSlidingWindowGetStats_EmptyWindow(t *testing.T) {
	sw := NewSlidingWindowLimiter(1, time.Minute)
	defer func() { _ = sw.Close() }()

	// maxRequests=1、n=2 超限：窗口已建（map 存在）但 timestamps 保持空。
	res, err := sw.AllowN(context.Background(), "empty-key", 2)
	require.NoError(t, err)
	assert.False(t, res.Allowed)

	// 修复前：len(timestamps)-1 = -1 → index out of range panic。
	require.NotPanics(t, func() {
		stats := sw.GetStats("empty-key")
		require.NotNil(t, stats)
		assert.Equal(t, "empty-key", stats.Key)
		assert.Equal(t, 1, stats.CurrentTokens) // maxRequests(1) - 0
		assert.True(t, stats.LastRequest.IsZero(), "空窗口 LastRequest 应为零值")
	})
}
