package register

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSyncOnce_BoundedByRequestTimeout 验证 ARCH-022：SyncOnce 单次注册也
// 应用 requestTimeout 边界——Register 挂起（无响应）时按预算退出，不永久阻塞。
func TestSyncOnce_BoundedByRequestTimeout(t *testing.T) {
	hang := newFakeConn()
	hang.registerRelease = make(chan struct{}) // 永不关闭 → Register 阻塞到 ctx.Done
	s := &dialerScript{conns: []*fakeConn{hang}}
	cfg := fastCfg(s, nil)
	cfg.RequestTimeout = 60 * time.Millisecond
	c := New(cfg)
	c.setConn(hang)

	start := time.Now()
	err := c.SyncOnce(context.Background())
	elapsed := time.Since(start)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled),
		"expected deadline/cancel error, got: %v", err)
	// 按预算退出：不因挂起 Register 长期阻塞。
	assert.Less(t, elapsed, 2*time.Second, "SyncOnce should exit within budget, took %v", elapsed)
	assert.GreaterOrEqual(t, elapsed, 40*time.Millisecond, "should not exit before requestTimeout")
}

// TestDialAndRegister_RegisterBounded 验证 ARCH-022：DialAndRegister 的注册
// 调用同样应用 requestTimeout 边界——无注册响应时按预算退出并关闭半开连接。
func TestDialAndRegister_RegisterBounded(t *testing.T) {
	hang := newFakeConn()
	hang.registerRelease = make(chan struct{}) // 永不关闭 → Register 阻塞到 ctx.Done
	s := &dialerScript{conns: []*fakeConn{hang}}
	cfg := fastCfg(s, nil)
	cfg.RequestTimeout = 60 * time.Millisecond
	c := New(cfg)

	start := time.Now()
	err := c.DialAndRegister(context.Background())
	elapsed := time.Since(start)
	require.Error(t, err)
	assert.Less(t, elapsed, 2*time.Second, "DialAndRegister should exit within budget, took %v", elapsed)
	// 半开连接必须关闭，连接快照清空。
	assert.True(t, hang.closed.Load(), "half-open conn should be closed on register failure")
	assert.Nil(t, c.Conn())
}

// TestRegisterCtx_CallerDeadlinePriority 验证 ARCH-022：调用方 deadline 早于
// requestTimeout 时以调用方为准（取较早者），不因 requestTimeout 放宽挂起。
func TestRegisterCtx_CallerDeadlinePriority(t *testing.T) {
	cfg := fastCfg(&dialerScript{}, nil)
	cfg.RequestTimeout = 10 * time.Second // 默认档，远晚于调用方
	c := New(cfg)

	callerCtx, callerCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer callerCancel()
	regCtx, cancel := c.registerCtx(callerCtx)
	defer cancel()

	deadline, ok := regCtx.Deadline()
	require.True(t, ok, "registerCtx should carry a deadline")
	// deadline 应≈调用方 deadline（50ms），而非 10s。
	assert.Less(t, time.Until(deadline), 500*time.Millisecond,
		"caller deadline should win, remaining=%v", time.Until(deadline))
	assert.True(t, time.Until(deadline) > 0, "deadline should be in the future")
}

// TestRegisterCtx_NoCallerDeadlineUsesBudget 验证 ARCH-022：调用方无 deadline
// 时按 requestTimeout 边界执行（默认 10s 档）。
func TestRegisterCtx_NoCallerDeadlineUsesBudget(t *testing.T) {
	cfg := fastCfg(&dialerScript{}, nil)
	cfg.RequestTimeout = 250 * time.Millisecond
	c := New(cfg)

	regCtx, cancel := c.registerCtx(context.Background())
	defer cancel()
	deadline, ok := regCtx.Deadline()
	require.True(t, ok)
	remaining := time.Until(deadline)
	assert.Greater(t, remaining, 100*time.Millisecond,
		"no caller deadline → use requestTimeout budget, remaining=%v", remaining)
	assert.Less(t, remaining, 400*time.Millisecond,
		"no caller deadline → use requestTimeout budget, remaining=%v", remaining)
}
