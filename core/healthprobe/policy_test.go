package healthprobe

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProbePolicyValid(t *testing.T) {
	p, err := ParseProbePolicy(`{
		"interval": "30s",
		"targets": [{"target": "db-primary", "interval": "10s"}],
		"windows": [{"start": "19:00", "end": "23:00", "interval": "8s"}],
		"events": [{"at": "2026-10-20T09:00:00+08:00", "duration": "30m", "interval": "5s"}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, "30s", p.Interval)
	require.Len(t, p.Targets, 1)
	assert.Equal(t, "db-primary", p.Targets[0].Target)
	require.Len(t, p.Windows, 1)
	assert.Equal(t, "19:00", p.Windows[0].Start)
	require.Len(t, p.Events, 1)
	assert.True(t, p.AutoBoostEnabled())
	assert.Equal(t, 30*time.Second, p.BaseInterval())
}

func TestParseProbePolicyRejects(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"空内容", ""},
		{"非对象", `[]`},
		{"坏 JSON", `{"interval":`},
		{"基础时长非法", `{"interval": "abc"}`},
		{"基础时长非正", `{"interval": "0s"}`},
		{"目标时长非法", `{"targets": [{"target": "t", "interval": "x"}]}`},
		{"目标名空", `{"targets": [{"target": " ", "interval": "10s"}]}`},
		{"目标重名", `{"targets": [{"target": "t", "interval": "10s"}, {"target": "t", "interval": "5s"}]}`},
		{"钟点非法", `{"windows": [{"start": "19:00", "end": "xx:00", "interval": "8s"}]}`},
		{"终点不晚于起点", `{"windows": [{"start": "19:00", "end": "19:00", "interval": "8s"}]}`},
		{"跨午夜", `{"windows": [{"start": "23:00", "end": "01:00", "interval": "8s"}]}`},
		{"时点缺偏移", `{"events": [{"at": "2026-10-20T09:00:00", "duration": "30m", "interval": "5s"}]}`},
		{"时点格式非法", `{"events": [{"at": "tomorrow", "duration": "30m", "interval": "5s"}]}`},
		{"时点时长非法", `{"events": [{"at": "2026-10-20T09:00:00+08:00", "duration": "0", "interval": "5s"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseProbePolicy(tc.content)
			assert.Error(t, err, tc.content)
		})
	}
	// 未知键宽容（前向扩展不改面板，capture gates 同口径）。
	_, err := ParseProbePolicy(`{"interval": "30s", "future": {"x": 1}}`)
	assert.NoError(t, err)
}

// 时段匹配按 now 自带时区的钟点（生产传 time.Now()=服务器配置时区；测试
// 钉固定时区保证确定性），event 匹配按绝对时刻（+08:00 显式偏移）。
var testTZ = time.FixedZone("test", 8*3600)

func TestEffectiveIntervalOverlay(t *testing.T) {
	p, err := ParseProbePolicy(`{
		"interval": "30s",
		"targets": [{"target": "db-primary", "interval": "10s"}],
		"windows": [{"start": "19:00", "end": "23:00", "interval": "8s"}],
		"events": [{"at": "2026-10-20T20:00:00+08:00", "duration": "1h", "interval": "5s"}]
	}`)
	require.NoError(t, err)

	at := func(h, m int) time.Time { return time.Date(2026, 10, 20, h, m, 0, 0, testTZ) }

	// 基础。
	assert.Equal(t, 30*time.Second, p.EffectiveInterval("other", at(12, 0)))
	// 按目标覆盖（基础之上的第一层）。
	assert.Equal(t, 10*time.Second, p.EffectiveInterval("db-primary", at(12, 0)))
	// 时段覆盖目标（19:30 落在窗口内）。
	assert.Equal(t, 8*time.Second, p.EffectiveInterval("db-primary", at(19, 30)))
	// 窗口边界：start 含、end 不含。
	assert.Equal(t, 8*time.Second, p.EffectiveInterval("other", at(19, 0)))
	assert.Equal(t, 30*time.Second, p.EffectiveInterval("other", at(23, 0)))
	// event 最优先（20:30 同时落在窗口与 event 区间）。
	assert.Equal(t, 5*time.Second, p.EffectiveInterval("db-primary", at(20, 30)))
	// event 结束后（21:30）回落窗口。
	assert.Equal(t, 8*time.Second, p.EffectiveInterval("db-primary", at(21, 30)))
	// event 起点前一刻（19:59）不命中 event：窗口生效。
	assert.Equal(t, 8*time.Second, p.EffectiveInterval("other", at(19, 59)))
}

func TestEffectiveIntervalForBoost(t *testing.T) {
	p, err := ParseProbePolicy(`{"interval": "30s", "windows": [{"start": "19:00", "end": "23:00", "interval": "8s"}]}`)
	require.NoError(t, err)
	noBoost, err := ParseProbePolicy(`{"interval": "30s", "autoBoost": false}`)
	require.NoError(t, err)
	fast, err := ParseProbePolicy(`{"interval": "4s"}`)
	require.NoError(t, err)

	at := func(h, m int) time.Time { return time.Date(2026, 10, 20, h, m, 0, 0, testTZ) }

	// 正常态：纯叠加。
	assert.Equal(t, 8*time.Second, p.EffectiveIntervalFor("other", at(19, 30), false))
	// 异常态提频：30s→15s；8s→5s（下限）；覆盖时段策略。
	assert.Equal(t, 15*time.Second, p.EffectiveIntervalFor("other", at(12, 0), true))
	assert.Equal(t, 5*time.Second, p.EffectiveIntervalFor("other", at(19, 30), true))
	// 原间隔已低于下限 → 提频绝不放慢。
	assert.Equal(t, 4*time.Second, fast.EffectiveIntervalFor("t", time.Time{}, true))
	// 开关关：不提频。
	assert.Equal(t, 30*time.Second, noBoost.EffectiveIntervalFor("t", time.Time{}, true))
}

func TestProbeRunDynamic(t *testing.T) {
	// 间隔函数首轮回 20ms、后续回 40ms：断言动态取值生效（多轮探测都被
	// 执行）且 ctx 取消后循环退出。
	var calls int32
	results := make(chan Result, 8)
	p := New("t", SemanticsReadiness, CheckerFunc(func(context.Context) error { return nil }), OnResult(func(r Result) {
		results <- r
	}))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		p.RunDynamic(ctx, func(time.Time) time.Duration {
			if atomic.AddInt32(&calls, 1) == 1 {
				return 20 * time.Millisecond
			}
			return 40 * time.Millisecond
		})
		close(done)
	}()
	// 首轮探测立即发生，随后按动态间隔推进；收到 3 个结果即取消。
	deadline := time.After(5 * time.Second)
	for i := 0; i < 3; i++ {
		select {
		case <-results:
		case <-deadline:
			t.Fatal("timed out waiting for probe results")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RunDynamic did not exit after cancel")
	}
	assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(3))
}

func TestManagerStartDynamic(t *testing.T) {
	mgr, err := NewManager(
		[]TargetConfig{{Name: "a", Type: "tcp", Address: "127.0.0.1:1", Interval: time.Second}},
		"g", "e", nil, nil,
	)
	require.NoError(t, err)

	var mu int32
	fn := func(target string, now time.Time) time.Duration {
		atomic.AddInt32(&mu, 1)
		return 10 * time.Millisecond
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		mgr.StartDynamic(ctx, fn)
		close(done)
	}()
	// StartDynamic 每轮取间隔：等 fn 被调用多次即证明循环在跑。
	deadline := time.After(5 * time.Second)
	for atomic.LoadInt32(&mu) < 3 {
		select {
		case <-deadline:
			t.Fatal("intervalFn not invoked repeatedly")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("StartDynamic did not exit after cancel")
	}
}
