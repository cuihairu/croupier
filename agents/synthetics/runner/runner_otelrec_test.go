package runner

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/config"
)

// fakeRecorder records emissions under a mutex: execute() runs on the test
// goroutine, so assertions read after execute returns are race-free by
// construction; the mutex keeps the fake itself safe under -race if a later
// test drives it from Run's goroutine.
type fakeRecorder struct {
	mu     sync.Mutex
	starts int
	stops  int
	counts []recCall
	obss   []recCall
}

type recCall struct {
	name  string
	n     int64
	attrs map[string]string
}

func (f *fakeRecorder) Start(context.Context, map[string]string) {
	f.mu.Lock()
	f.starts++
	f.mu.Unlock()
}

func (f *fakeRecorder) Stop(context.Context, error) {
	f.mu.Lock()
	f.stops++
	f.mu.Unlock()
}

func (f *fakeRecorder) Connected(context.Context)           {}
func (f *fakeRecorder) Disconnected(context.Context, error) {}

func (f *fakeRecorder) Count(_ context.Context, name string, n int64, attrs map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counts = append(f.counts, recCall{name: name, n: n, attrs: attrs})
}

func (f *fakeRecorder) Observe(_ context.Context, name string, v float64, attrs map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.obss = append(f.obss, recCall{name: name, n: int64(v), attrs: attrs})
}

func waitForCond(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal(msg)
}

// Run 装配 recorder 后必须打到 Start/Stop：启停是 agent 生命周期的两条
// 基线指标（OBS-AGENT-001）。dial 恒败不影响断言——register 客户端后台
// 重试，Run 阻塞在 runTargets 直到 ctx 取消。
func TestRunLifecycleStartStopObserved(t *testing.T) {
	r := New(config.Config{
		Agent:      config.Agent{ID: "syn-otel", ServerAddr: "127.0.0.1:1", GameID: "demo", Env: "prod"},
		Synthetics: config.Synthetics{Interval: time.Minute},
	}, nil)
	rec := &fakeRecorder{}
	r.SetRecorder(rec)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()

	waitForCond(t, func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return rec.starts == 1
	}, "expected Start recorded")
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("expected Run to return after cancel")
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.stops != 1 {
		t.Fatalf("expected exactly one Stop, got %d", rec.stops)
	}
}

// execute 每次探测必须打点 probe 计数（target/ok/status）+ probe_latency
// 直方图；会话不可用时再打 errors{kind=session_unavailable}（探测结果
// 未上报的显式口径，不与探测失败混淆）。
func TestExecuteEmitsProbeMetrics(t *testing.T) {
	r, target := newConnTestRunner(t)
	rec := &fakeRecorder{}
	r.SetRecorder(rec)

	r.execute(context.Background(), target) // no upstream: degraded path

	rec.mu.Lock()
	defer rec.mu.Unlock()
	var probe, sessionErr int
	var status string
	for _, c := range rec.counts {
		switch {
		case c.name == "probe":
			probe++
			if c.attrs["target"] != target.Name {
				t.Fatalf("probe attrs target = %q", c.attrs["target"])
			}
			status = c.attrs["status"]
		case c.name == "errors" && c.attrs["kind"] == "session_unavailable":
			sessionErr++
		}
	}
	if probe != 1 || status != "200" {
		t.Fatalf("probe count = %d status = %q", probe, status)
	}
	if sessionErr != 1 {
		t.Fatalf("session_unavailable errors = %d", sessionErr)
	}
	if len(rec.obss) != 1 || rec.obss[0].name != "probe_latency" || rec.obss[0].attrs["target"] != target.Name {
		t.Fatalf("probe_latency observations = %+v", rec.obss)
	}
}

// SetRecorder(nil) 回退 noop（默认关闭=零开销路径契约）。
func TestSetRecorderNilFallsBackToNoop(t *testing.T) {
	r, _ := newConnTestRunner(t)
	r.SetRecorder(nil)
	if r.rec == nil {
		t.Fatal("nil recorder must fall back to a non-nil noop")
	}
}
