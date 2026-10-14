package report

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/cuihairu/croupier/core/healthprobe"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// nowForTest 固定时间基准（窗口时间戳断言不依赖真实时钟）。
var nowForTest = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// sink 收集 ProbeUplink 单发的 AlertEvent 原文（并发安全：上行在探测 goroutine）。
type sink struct {
	mu  sync.Mutex
	buf [][]byte
	err error
}

func (s *sink) send(_ context.Context, body []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.buf = append(s.buf, append([]byte(nil), body...))
	return nil
}

func (s *sink) events(t *testing.T) []*opsv1.AlertEvent {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*opsv1.AlertEvent, 0, len(s.buf))
	for _, b := range s.buf {
		ev := &opsv1.AlertEvent{}
		require.NoError(t, proto.Unmarshal(b, ev))
		out = append(out, ev)
	}
	return out
}

func testUplink(s *sink, maxDetails int) *ProbeUplink {
	return NewProbeUplink(ProbeUplinkConfig{
		AgentID:             "agent-1",
		GameID:              "demo",
		Env:                 "prod",
		Target:              "redis:6379",
		Semantics:           healthprobe.SemanticsReadiness,
		MaxDetailsPerWindow: maxDetails,
	}, s.send)
}

func okResult(latency int64) healthprobe.Result {
	return healthprobe.Result{OK: true, LatencyMS: latency}
}

func failResult(err string, latency, streak int64, windowOpen bool) healthprobe.Result {
	return healthprobe.Result{
		OK:                  false,
		Err:                 err,
		LatencyMS:           latency,
		ConsecutiveFailures: int(streak),
		WindowOpen:          windowOpen,
		At:                  nowForTest,
		Details:             map[string]string{"probeType": "tcp"},
	}
}

func TestProbeUplinkSuccessOnlyCounts(t *testing.T) {
	s := &sink{}
	u := testUplink(s, 0)

	u.OnResult(okResult(10))
	u.OnResult(okResult(30))

	// 成功样本不逐条上行（不对称：正常态只计数）。
	assert.Empty(t, s.events(t))

	st := u.Drain()
	assert.EqualValues(t, 2, st.Checked)
	assert.EqualValues(t, 0, st.Failed)
	assert.Equal(t, 1.0, st.SuccessRate)
	assert.EqualValues(t, 2, st.LatencyBuckets[0])
	assert.Equal(t, "redis:6379", st.Target)
	assert.Equal(t, string(healthprobe.SemanticsReadiness), st.Semantics)
	assert.Equal(t, "demo", st.GameId)
	assert.Equal(t, "prod", st.Env)
}

func TestProbeUplinkDrainResets(t *testing.T) {
	u := testUplink(&sink{}, 0)
	u.OnResult(okResult(5))
	first := u.Drain()
	require.EqualValues(t, 1, first.Checked)

	second := u.Drain()
	assert.EqualValues(t, 0, second.Checked, "Drain must reset counters")
}

func TestProbeUplinkFailureDetailUplink(t *testing.T) {
	s := &sink{}
	u := testUplink(s, 0)

	u.OnResult(failResult("connection refused", 12, 1, true))

	evs := s.events(t)
	require.Len(t, evs, 1)
	assert.Equal(t, ProbeKindDetail, evs[0].Kind)
	assert.Equal(t, "agent-1", evs[0].AgentId)
	assert.Equal(t, "demo", evs[0].GameId)
	require.NotNil(t, evs[0].Detail)
	assert.Equal(t, "redis:6379", evs[0].Detail.Target)
	assert.Equal(t, "connection refused", evs[0].Detail.Err)
	assert.EqualValues(t, 12, evs[0].Detail.LatencyMs)
	assert.True(t, evs[0].Detail.WindowOpen)
	assert.Equal(t, "tcp", evs[0].Detail.Details["probeType"])

	st := u.Drain()
	assert.EqualValues(t, 1, st.Checked)
	assert.EqualValues(t, 1, st.Failed)
	assert.Equal(t, 0.0, st.SuccessRate)
	assert.Equal(t, "connection refused", st.LastReason)
}

func TestProbeUplinkDetailThrottleKeepsFacts(t *testing.T) {
	s := &sink{}
	u := testUplink(s, 2)

	u.OnResult(failResult("r1", 1, 1, true))
	u.OnResult(failResult("r2", 1, 2, true))
	u.OnResult(failResult("r3", 1, 3, true))
	u.OnResult(failResult("r4", 1, 4, true))

	// 明细被上限裁掉，但失败总数照实。
	assert.Len(t, s.events(t), 2)
	st := u.Drain()
	assert.EqualValues(t, 4, st.Failed, "failure facts must never be trimmed")
	assert.EqualValues(t, 2, st.DroppedDetails)
}

func TestProbeUplinkWindowOpensResetsDetailQuota(t *testing.T) {
	s := &sink{}
	u := testUplink(s, 1)

	// 阈值前失败也即时回流（等窗口开合会淹没最关键的早期事实）。
	u.OnResult(failResult("pre-window", 1, 1, false))
	// 达阈值开窗：事件上行 + 明细配额重置。开窗时间取真实过去时刻，
	// Drain 的窗口时长按墙钟算。
	openedTS := time.Now().Add(-3 * time.Second).UnixMilli()
	u.OnWindow(healthprobe.WindowEvent{
		Kind: healthprobe.EventUnavailable,
		Window: healthprobe.Window{
			Target:     "redis:6379",
			Semantics:  string(healthprobe.SemanticsReadiness),
			StartTS:    openedTS,
			LastReason: "pre-window",
		},
	})
	u.OnResult(failResult("in-window", 1, 4, true))

	evs := s.events(t)
	require.Len(t, evs, 3)
	assert.Equal(t, ProbeKindDetail, evs[0].Kind)
	assert.Equal(t, ProbeKindWindow, evs[1].Kind)
	require.NotNil(t, evs[1].Window)
	assert.Equal(t, "redis:6379", evs[1].Window.Target)
	assert.Equal(t, healthprobe.EventUnavailable, evs[1].Window.WindowKind)
	assert.Equal(t, openedTS, evs[1].Window.StartTsUnixMs)
	assert.Zero(t, evs[1].Window.EndTsUnixMs, "open window has no end")
	// 新故障窗口 → 新明细配额（否则窗口内所有明细都会被阈值前那条挤掉）。
	assert.Equal(t, ProbeKindDetail, evs[2].Kind)
	assert.Equal(t, "in-window", evs[2].Detail.Err)

	st := u.Drain()
	assert.True(t, st.WindowOpen)
	assert.GreaterOrEqual(t, st.WindowDurationMs, int64(3000))
}

func TestProbeUplinkWindowRecovers(t *testing.T) {
	s := &sink{}
	u := testUplink(s, 0)

	u.OnWindow(healthprobe.WindowEvent{
		Kind: healthprobe.EventUnavailable,
		Window: healthprobe.Window{
			Target:  "redis:6379",
			StartTS: nowForTest.UnixMilli() - 5000,
		},
	})
	u.OnWindow(healthprobe.WindowEvent{
		Kind: healthprobe.EventRecovered,
		Window: healthprobe.Window{
			Target:     "redis:6379",
			StartTS:    nowForTest.UnixMilli() - 5000,
			EndTS:      nowForTest.UnixMilli(),
			DurationMS: 5000,
		},
	})

	evs := s.events(t)
	require.Len(t, evs, 2)
	assert.Equal(t, healthprobe.EventRecovered, evs[1].Window.WindowKind)
	assert.EqualValues(t, 5000, evs[1].Window.DurationMs)

	st := u.Drain()
	assert.False(t, st.WindowOpen)
	assert.Zero(t, st.WindowDurationMs)
}

func TestProbeUplinkNilSendStillCounts(t *testing.T) {
	// server 不消费时 agent 照跑（M2 可回退语义）：组包面不因缺通道丢计数。
	u := NewProbeUplink(ProbeUplinkConfig{Target: "t"}, nil)
	u.OnResult(okResult(5))
	u.OnResult(failResult("boom", 5, 1, true))
	st := u.Drain()
	assert.EqualValues(t, 2, st.Checked)
	assert.EqualValues(t, 1, st.Failed)
}

func TestProbeUplinkSendFailureSwallowed(t *testing.T) {
	s := &sink{err: errors.New("not connected")}
	u := testUplink(s, 0)
	u.OnResult(failResult("boom", 5, 1, true))
	st := u.Drain()
	assert.EqualValues(t, 1, st.Failed, "local counting survives uplink failure")
}

func TestProbeUplinkLatencyBuckets(t *testing.T) {
	u := testUplink(&sink{}, 0)
	for _, ms := range []int64{10, 150, 900, 3000, 9000} {
		u.OnResult(okResult(ms))
	}
	st := u.Drain()
	require.Len(t, st.LatencyBuckets, len(LatencyBucketBoundaries)+1)
	for i, want := range []int64{1, 1, 1, 1, 1} {
		assert.Equalf(t, want, st.LatencyBuckets[i], "bucket %d", i)
	}
}

func TestProbeUplinkTruncatesLongError(t *testing.T) {
	long := make([]byte, maxDetailErrBytes+500)
	for i := range long {
		long[i] = 'e'
	}
	s := &sink{}
	u := testUplink(s, 0)
	u.OnResult(failResult(string(long), 1, 1, true))
	evs := s.events(t)
	require.Len(t, evs, 1)
	assert.LessOrEqual(t, len(evs[0].Detail.Err), maxDetailErrBytes)
}

func TestTruncateUTF8KeepsRuneBoundary(t *testing.T) {
	s := "探测失败" // 4 runes = 12 bytes
	assert.Equal(t, "探测失败", truncateUTF8(s, 1024))
	// 7 bytes would cut 失 in half → back off to the 2-rune boundary.
	assert.Equal(t, "探测", truncateUTF8(s, 7))
	assert.True(t, utf8.ValidString(truncateUTF8(s, 7)), "truncation must not split a multi-byte rune")
	// Exact rune boundaries are kept as-is.
	assert.Equal(t, "探测失", truncateUTF8(s, 9))
}

func TestProbeUplinkConcurrentDrain(t *testing.T) {
	u := testUplink(&sink{}, 0)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				u.OnResult(okResult(7))
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 10; j++ {
			_ = u.Drain()
		}
	}()
	wg.Wait()
}
