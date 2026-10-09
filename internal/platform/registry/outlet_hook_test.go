package registry

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// TestMetricsStoreSupervisorEventHook 覆盖新监管事件回调：仅去重后的
// 新事件触发、nil 事件跳过、重传不重触发、回调在锁外异步执行。
// 回调是每事件一 goroutine（锁外异步），收集用锁保护；跨事件到达顺序
// 无保证（出口侧由 herald dedup 按事件 identity 折叠），断言按集合比。
func TestMetricsStoreSupervisorEventHook(t *testing.T) {
	s := NewMetricsStoreWithConfig(MetricsStoreConfig{MaxMemoryEntries: 10, MaxTotalEntries: 100})

	var mu sync.Mutex
	var got []string
	s.SetOnSupervisorEvent(func(ctx context.Context, agentID string, ev *opsv1.SupervisorEvent) {
		if ctx == nil {
			t.Error("ctx must not be nil")
		}
		mu.Lock()
		defer mu.Unlock()
		got = append(got, agentID+":"+ev.GetEvent()+":"+strconv.FormatInt(ev.GetSeq(), 10))
	})

	mk := func(seq, ts int64, ev string) *opsv1.MetricsReport {
		return &opsv1.MetricsReport{SupervisorEvents: []*opsv1.SupervisorEvent{
			{Seq: seq, TsUnix: ts, Event: ev, Process: "gs", Message: "m"},
		}}
	}

	s.Add("agent-1", mk(1, 100, "detect_down"))
	s.Add("agent-1", mk(2, 200, "breaker_tripped"))
	// 重传：seq/ts 都不新，丢弃不触发。
	s.Add("agent-1", mk(2, 200, "breaker_tripped"))
	// nil 事件跳过。
	s.Add("agent-1", nil)
	// agent 重启游标归零：seq 回退但 ts 更新 → 新事件触发。
	s.Add("agent-1", mk(3, 150, "manual_start"))

	want := []string{
		"agent-1:breaker_tripped:2",
		"agent-1:detect_down:1",
		"agent-1:manual_start:3",
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n >= 3 || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 3 {
		t.Fatalf("hook calls = %d (%v), want 3", len(got), got)
	}
	sorted := append([]string(nil), got...)
	sort.Strings(sorted)
	for i := range want {
		if sorted[i] != want[i] {
			t.Fatalf("hook events = %v, want %v", got, want)
		}
	}
}

func TestMetricsStoreSupervisorEventHookNilSafe(t *testing.T) {
	s := NewMetricsStoreWithConfig(MetricsStoreConfig{MaxMemoryEntries: 10, MaxTotalEntries: 100})
	report := &opsv1.MetricsReport{SupervisorEvents: []*opsv1.SupervisorEvent{
		{Seq: 1, TsUnix: 100, Event: "detect_down"},
	}}
	s.Add("agent-1", report) // 未设 hook：不 panic
}
