//go:build !no_go_plugin

package pluginhost

// engine.go 补测：NewController 的 nil Log 兜底、Call 的 ensure 失败透传臂、
// callFailureCause 的超时/不可用/普通错误分类臂、ensure 的停止/未配置/禁用/
// 熔断开启/退避窗口守卫臂与 active 命中两臂（存活复用 + 崩溃后 Kill+failLocked）、
// startLocked 的 connect/Client 转换/Describe 握手失败臂、failLocked 的退避
// 指数封顶钳位、isDeadline 的 errors.Is 臂、Stop 的在飞排水窗口。
//
// 诚实跳过：startLocked 的 Dispense 失败臂（329）——startLocked 以
// ClientConfig.Plugins 注册的同一 ct.Key 调 Dispense，键恒存在，gRPC 客户端
// 侧的 unknown-plugin-type 守卫经该入口不可达。

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stretchr/testify/require"
)

func TestNewControllerNilLogFallback(t *testing.T) {
	c := NewController[testImpl](nil, echoContract(), Knobs{})
	t.Cleanup(c.Stop)
	require.NotNil(t, c.log)
	require.Equal(t, DefaultMaxConcurrent, c.maxConcurrent)
}

func TestCallEnsureFailurePassthrough(t *testing.T) {
	c := newEchoController(t, Knobs{})
	t.Cleanup(c.Stop)
	cause := c.Call(context.Background(), "ghost", time.Second,
		func(context.Context, testImpl) error { return nil })
	require.Contains(t, cause, CauseNotConfigured)
}

func TestCallFailureCauseClassification(t *testing.T) {
	c := newEchoController(t, Knobs{})
	t.Cleanup(c.Stop)

	// 超时：context.DeadlineExceeded → CauseTimeout
	require.Contains(t,
		c.callFailureCause(&process[testImpl]{}, context.DeadlineExceeded), CauseTimeout)

	// gRPC Unavailable（含 client 为 nil 的短路）→ CauseUnavailable
	require.Contains(t,
		c.callFailureCause(&process[testImpl]{}, status.Error(codes.Unavailable, "conn gone")),
		CauseUnavailable)

	// 普通错误 → CauseError
	require.Contains(t,
		c.callFailureCause(&process[testImpl]{}, errors.New("plain")), CauseError)
}

func TestEnsureGuards(t *testing.T) {
	// 停止后 ensure 的兜底臂（Call 入口先经 isStopped 短路，此臂只能直测）
	stopped := newEchoController(t, Knobs{})
	stopped.Stop()
	p, cause := stopped.ensure("echo")
	require.Nil(t, p)
	require.Contains(t, cause, CauseDisabled)

	c := newEchoController(t, Knobs{})
	t.Cleanup(c.Stop)

	// 未配置
	p, cause = c.ensure("ghost")
	require.Nil(t, p)
	require.Contains(t, cause, CauseNotConfigured)

	// 声明但禁用
	c.cfgs["off"] = Config{Name: "off", Command: echoBin, Enabled: false}
	_, cause = c.ensure("off")
	require.Contains(t, cause, CauseDisabled)

	// 熔断开启
	now := time.Now()
	c.health["echo"] = &pluginHealth{openUntil: now.Add(time.Minute)}
	_, cause = c.ensure("echo")
	require.Contains(t, cause, CauseCircuitOpen)

	// 退避窗口（熔断已闭合、下一次重试未到期）
	c.health["echo"] = &pluginHealth{nextAttempt: now.Add(time.Minute)}
	_, cause = c.ensure("echo")
	require.Contains(t, cause, CauseBackoff)
	delete(c.health, "echo")

	// active 命中且存活：驻留进程建立后重复 Call 直接复用（290）
	cause = c.Call(context.Background(), "echo", time.Second,
		func(context.Context, testImpl) error { return nil })
	require.Empty(t, cause)
	cause = c.Call(context.Background(), "echo", time.Second,
		func(context.Context, testImpl) error { return nil })
	require.Empty(t, cause)
}

// active 命中但进程已退出：Kill 标志置位后 ensure 走崩溃臂（Kill + 剔除 +
// failLocked 记退避），下一次 ensure 在退避窗口内被拒。
func TestEnsureActiveCrashedKillsAndBacksOff(t *testing.T) {
	c := newEchoController(t, Knobs{})
	t.Cleanup(c.Stop)
	cause := c.Call(context.Background(), "echo", time.Second,
		func(context.Context, testImpl) error { return nil })
	require.Empty(t, cause)

	c.mu.Lock()
	p := c.active["echo"]
	c.mu.Unlock()
	require.NotNil(t, p)
	p.client.Kill()

	// Kill 返回后退出标志由 watcher 落位；轮询确保 Exited() 读到 true。
	deadline := time.Now().Add(time.Second)
	for !p.client.Exited() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	require.True(t, p.client.Exited())

	_, cause = c.ensure("echo")
	require.Contains(t, cause, CauseUnavailable)

	_, cause = c.ensure("echo")
	require.Contains(t, cause, CauseBackoff)
}

func TestStartLockedConnectFailure(t *testing.T) {
	c := newEchoController(t, Knobs{})
	t.Cleanup(c.Stop)
	c.cfgs["bad"] = Config{
		Name: "bad", Command: "/nonexistent-croupier-plugin-bin",
		Timeout: 200 * time.Millisecond, Enabled: true,
	}
	_, cause := c.ensure("bad")
	require.Contains(t, cause, CauseUnavailable)
	require.Contains(t, cause, "connect")
}

// Client 转换失败与 Describe 握手失败：复用 echo 子进程（握手与 dispense
// 均通过），在 Contract 的两个适配钩子上注入错误。
func TestStartLockedClientCastAndDescribeFailures(t *testing.T) {
	build := func(mutate func(*Contract[testImpl])) *Controller[testImpl] {
		ct := echoContract()
		mutate(&ct)
		c := NewController([]Config{
			{Name: "echo", Command: echoBin, Timeout: 5 * time.Second, Enabled: true},
		}, ct, Knobs{Log: testLogger(t)})
		t.Cleanup(c.Stop)
		return c
	}

	castBad := build(func(ct *Contract[testImpl]) {
		ct.Client = func(any) (testImpl, error) { return testImpl{}, errors.New("cast refused") }
	})
	_, cause := castBad.ensure("echo")
	require.Contains(t, cause, "client")

	descBad := build(func(ct *Contract[testImpl]) {
		ct.Describe = func(context.Context, testImpl) (string, string, error) {
			return "", "", errors.New("describe refused")
		}
	})
	_, cause = descBad.ensure("echo")
	require.Contains(t, cause, "describe handshake")
}

// failLocked 的指数封顶钳位：backoff=3s、cap=4s、consecFails=5 →
// d=3s×2=6s 超出 cap → 钳位到 4s（断路器未触发，返回原始 cause）。
func TestFailLockedBackoffCapClamp(t *testing.T) {
	c := newEchoController(t, Knobs{Backoff: 3 * time.Second, BackoffCap: 4 * time.Second, BreakAfter: 100})
	t.Cleanup(c.Stop)
	c.health["echo"] = &pluginHealth{consecFails: 5}
	require.Equal(t, "boom", c.failLocked("echo", "boom"))
	st := c.health["echo"]
	require.WithinDuration(t, time.Now().Add(4*time.Second), st.nextAttempt, time.Second)
}

func TestIsDeadlineErrorsIs(t *testing.T) {
	require.True(t, isDeadline(context.DeadlineExceeded))
	require.True(t, isDeadline(status.Error(codes.DeadlineExceeded, "late")))
	require.False(t, isDeadline(errors.New("other")))
}

// Stop 的在飞排水窗口：预占一个并发槽，Stop 须在槽释放后立即收口
// （而非等满整个 stopDrainWindow）。
func TestStopDrainsInFlightWindow(t *testing.T) {
	c := newEchoController(t, Knobs{})
	c.gate <- struct{}{}
	go func() {
		time.Sleep(30 * time.Millisecond)
		<-c.gate
	}()
	c.Stop()
	require.Equal(t, 0, c.InFlight())
}
