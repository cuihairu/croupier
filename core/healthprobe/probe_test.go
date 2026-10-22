package healthprobe

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbeThresholdsOpenAndCloseWindow(t *testing.T) {
	fail := true
	var events []WindowEvent
	p := New("game-1", SemanticsReadiness, CheckerFunc(func(context.Context) error {
		if fail {
			return errors.New("conn refused")
		}
		return nil
	}),
		WithThresholds(2, 1),
		WithScope("demo", "prod"),
		OnWindow(func(ev WindowEvent) { events = append(events, ev) }),
	)
	ctx := context.Background()

	// 初始 available；单次失败不达阈值不变态、不开窗。
	r := p.ProbeOnce(ctx)
	assert.True(t, r.Available)
	assert.False(t, r.WindowOpen)
	assert.Equal(t, 1, r.ConsecutiveFailures)

	r = p.ProbeOnce(ctx)
	assert.False(t, r.Available)
	assert.True(t, r.WindowOpen)
	require.Len(t, events, 1)
	assert.Equal(t, EventUnavailable, events[0].Kind)
	assert.Equal(t, int64(0), events[0].Window.EndTS)
	assert.Equal(t, "conn refused", events[0].Window.LastReason)
	assert.Equal(t, "demo", events[0].Window.GameID)
	assert.Equal(t, "prod", events[0].Window.Env)
	assert.Equal(t, string(SemanticsReadiness), events[0].Window.Semantics)

	// 未恢复前继续失败：仍是一个窗口，last_reason 持续刷新。
	p.ProbeOnce(ctx)
	_, open := p.CurrentWindow()
	require.True(t, open)
	cur, _ := p.CurrentWindow()
	assert.Equal(t, "conn refused", cur.LastReason)

	// 恢复达阈值：闭窗 + duration 落值 + recovered 事件。
	fail = false
	time.Sleep(2 * time.Millisecond) // 保证 duration > 0
	r = p.ProbeOnce(ctx)
	assert.True(t, r.Available)
	assert.False(t, r.WindowOpen)
	require.Len(t, events, 2)
	assert.Equal(t, EventRecovered, events[1].Kind)
	assert.Greater(t, events[1].Window.EndTS, events[1].Window.StartTS)
	assert.GreaterOrEqual(t, events[1].Window.DurationMS, int64(0))
	assert.Len(t, p.History(), 1)
}

func TestProbeRecoverThreshold(t *testing.T) {
	fail := true
	p := New("t", SemanticsLiveness, CheckerFunc(func(context.Context) error {
		if fail {
			return errors.New("down")
		}
		return nil
	}), WithThresholds(1, 2))
	ctx := context.Background()

	assert.True(t, p.ProbeOnce(ctx).WindowOpen) // fail=1 立即开窗
	fail = false
	assert.True(t, p.ProbeOnce(ctx).WindowOpen)  // 恢复 1 次不达阈值
	assert.False(t, p.ProbeOnce(ctx).WindowOpen) // 连续 2 次闭窗
}

func TestProbeHistoryLimit(t *testing.T) {
	n := 0
	p := New("t", SemanticsReadiness, CheckerFunc(func(context.Context) error {
		n++
		if n%2 == 1 {
			return errors.New("flap")
		}
		return nil
	}), WithThresholds(1, 1), WithHistoryLimit(3))
	ctx := context.Background()
	for range 10 {
		p.ProbeOnce(ctx)
	}
	assert.Len(t, p.History(), 3)
}

func TestProbeListenerNotDeadlockedByCallback(t *testing.T) {
	// listener 内回调查询方法：验证事件在解锁后触发。
	var p *Probe
	p = New("t", SemanticsLiveness, CheckerFunc(func(context.Context) error {
		return errors.New("down")
	}), WithThresholds(1, 1), OnWindow(func(WindowEvent) {
		assert.False(t, p.Available())
	}))
	p.ProbeOnce(context.Background())
}

func TestProbeRunStopsOnCancel(t *testing.T) {
	p := New("t", SemanticsLiveness, CheckerFunc(func(context.Context) error { return nil }), WithThresholds(1, 1))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.Run(ctx, time.Millisecond); close(done) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop on ctx cancel")
	}
}

type detailingChecker struct {
	err    error
	detail map[string]string
}

func (c *detailingChecker) Check(context.Context) error { return c.err }
func (c *detailingChecker) Details() map[string]string  { return c.detail }

func TestProbeOnceResultFields(t *testing.T) {
	// 明细型 Checker：Details 随结果带出；成功/失败两态都带。
	chk := &detailingChecker{err: errors.New("phase login failed"),
		detail: map[string]string{"probeType": "protocol", "phase": "login"}}
	var results []Result
	p := New("bot", SemanticsReadiness, chk,
		WithThresholds(1, 1),
		OnResult(func(r Result) { results = append(results, r) }))
	ctx := context.Background()

	r := p.ProbeOnce(ctx)
	assert.False(t, r.OK)
	assert.Equal(t, "phase login failed", r.Err)
	assert.Equal(t, map[string]string{"probeType": "protocol", "phase": "login"}, r.Details)
	assert.GreaterOrEqual(t, r.LatencyMS, int64(0))
	assert.False(t, r.Available)
	require.Len(t, results, 1) // 失败明细逐次回调，不等窗口开合

	chk.err = nil
	r = p.ProbeOnce(ctx)
	assert.Empty(t, r.Err)
	assert.True(t, r.OK)
	assert.True(t, r.Available)
	assert.Equal(t, 2, len(results))
}

func TestProbeOnceLatencyMeasured(t *testing.T) {
	p := New("t", SemanticsLiveness, CheckerFunc(func(context.Context) error {
		time.Sleep(15 * time.Millisecond)
		return nil
	}))
	r := p.ProbeOnce(context.Background())
	assert.GreaterOrEqual(t, r.LatencyMS, int64(10))
}

func TestProbeOnResultWithoutWindow(t *testing.T) {
	// OnResult 每次探测必回调（含未开窗的失败），窗口事件仍只在达阈值时产生。
	fail := true
	var windows []WindowEvent
	var results []Result
	p := New("t", SemanticsLiveness, CheckerFunc(func(context.Context) error {
		if fail {
			return errors.New("down")
		}
		return nil
	}), WithThresholds(3, 1),
		OnWindow(func(ev WindowEvent) { windows = append(windows, ev) }),
		OnResult(func(r Result) { results = append(results, r) }))
	ctx := context.Background()

	for range 2 { // 两次失败：2 条 result、0 条窗口
		p.ProbeOnce(ctx)
	}
	assert.Empty(t, windows)
	assert.Len(t, results, 2)

	fail = false
	p.ProbeOnce(ctx)
	assert.Empty(t, windows)
	assert.Len(t, results, 3)
}
