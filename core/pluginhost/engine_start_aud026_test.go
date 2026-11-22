//go:build !no_go_plugin

package pluginhost

// AUD-026 回归：插件首启动（subprocess spawn + Describe 握手）不得持控制器全局
// 状态锁阻塞其他插件的调用；启动预算须遵守调用方上下文。
//
// 缺陷形态：ensure 在 c.mu 内调 startLocked——慢启动插件的 Describe 持锁期间，
// 任何插件的 Call（含 isStopped 入口）都阻塞，调用方 deadline 不生效。
// 修复形态：start 移出状态锁（按插件 startLock 序列化首启动），Describe 预算
// = min(调用方上下文, pc.Timeout)。

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestSlowStartDoesNotBlockOtherPlugins 覆盖「慢启动不得阻塞其他调用」：插件
// slow 的 Describe 挂起（模拟慢握手），另一插件 quick 的调用必须不受影响——
// quick 的 Describe 须在 slow 仍被挂起期间被调用。缺陷下 quick 阻塞于全局锁，
// 直到 slow Describe 被释放才进入，本用例失败。
func TestSlowStartDoesNotBlockOtherPlugins(t *testing.T) {
	slowDescribe := make(chan struct{}, 1)
	releaseSlow := make(chan struct{})
	quickDescribe := make(chan struct{}, 1)
	var describeCalls atomic.Int32

	ct := echoContract()
	ct.Describe = func(ctx context.Context, impl testImpl) (string, string, error) {
		// 首次 Describe（slow 插件的握手）挂起直到测试释放；后续（quick 插件）
		// 立即返回。测试在发出 quick 调用前已等到 slowDescribe，保证顺序。
		if describeCalls.Add(1) == 1 {
			slowDescribe <- struct{}{}
			select {
			case <-releaseSlow:
			case <-ctx.Done():
				return "", "", ctx.Err()
			}
		}
		select {
		case quickDescribe <- struct{}{}:
		default:
		}
		return "echo", "1.0.0", nil
	}
	controller := NewController([]Config{
		{Name: "slow", Command: echoBin, Timeout: 30 * time.Second, Enabled: true},
		{Name: "quick", Command: echoBin, Timeout: 30 * time.Second, Enabled: true},
	}, ct, Knobs{Log: testLogger(t)})
	t.Cleanup(controller.Stop)

	slowDone := make(chan string, 1)
	go func() {
		slowDone <- controller.Call(context.Background(), "slow", time.Minute,
			func(context.Context, testImpl) error { return nil })
	}()
	<-slowDescribe // slow 握手进行中（缺陷下此时正持全局状态锁）

	quickDone := make(chan string, 1)
	go func() {
		quickDone <- controller.Call(context.Background(), "quick", time.Minute,
			func(context.Context, testImpl) error { return nil })
	}()
	select {
	case <-quickDescribe:
	case <-time.After(2 * time.Second):
		t.Fatal("call for another plugin was blocked by the slow plugin's start")
	}
	quickCause := <-quickDone
	requireEmptyCause(t, "quick call", quickCause)

	close(releaseSlow)
	requireEmptyCause(t, "slow call", <-slowDone)
}

// TestCallStartBudgetFollowsCallerContext 覆盖「启动预算遵守调用方上下文」：调用方
// 带短 deadline（50ms）发起冷启动，Describe 挂起，调用必须在调用方 deadline 量级
// 内返回（不再等待配置的长启动超时）。缺陷下 Describe 用 Background+pc.Timeout，
// 调用方 deadline 不生效，调用挂满 pc.Timeout 后才返回。
func TestCallStartBudgetFollowsCallerContext(t *testing.T) {
	releaseDescribe := make(chan struct{})
	entered := make(chan struct{}, 1)
	ct := echoContract()
	ct.Describe = func(ctx context.Context, impl testImpl) (string, string, error) {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-releaseDescribe:
		case <-ctx.Done():
			return "", "", ctx.Err()
		}
		return "echo", "1.0.0", nil
	}
	// 配置的启动超时远大于调用方 deadline：修复下由调用方 deadline 截断；
	// 缺陷下挂满该值。
	const configuredTimeout = 10 * time.Second
	controller := NewController([]Config{
		{Name: "echo", Command: echoBin, Timeout: configuredTimeout, Enabled: true},
	}, ct, Knobs{Log: testLogger(t)})
	t.Cleanup(controller.Stop)

	start := time.Now()
	done := make(chan string, 1)
	go func() {
		done <- controller.Call(context.Background(), "echo", 50*time.Millisecond,
			func(context.Context, testImpl) error { return nil })
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("start never began")
	}
	// 修复：调用在调用方 deadline（50ms）量级内返回（含进程 spawn 耗时）；
	// 缺陷：挂满 configuredTimeout。窗口取两者中段，修复留足余量。
	select {
	case cause := <-done:
		if elapsed := time.Since(start); elapsed >= configuredTimeout/2 {
			t.Fatalf("start budget did not follow the caller context: returned after %s (configured timeout %s)", elapsed, configuredTimeout)
		}
		if cause == "" {
			t.Fatal("call with a 50ms deadline against a hung handshake must not report success")
		}
	case <-time.After(configuredTimeout + time.Second):
		t.Fatal("call did not return at all")
	}
}

// requireEmptyCause 断言调用未被降级（cause 为空），失败时给出上下文。
func requireEmptyCause(t *testing.T, what, cause string) {
	t.Helper()
	if cause != "" {
		t.Fatalf("%s degraded: %s", what, cause)
	}
}
