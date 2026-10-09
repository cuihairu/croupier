// herald 告警出口布线（plugin-mechanism §5/M2）与 supervisor 事件→信封
// 映射的单元覆盖：缺省关、baseUrl/token 缺失降级不注册、开启即注册、
// 仅 breaker_tripped 消费（其余闭集类型出口零改动忽略）。
package svc

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

func svcWithHerald(t *testing.T, cfg config.HeraldConfig) (*ServiceContext, *reg.MetricsStore) {
	t.Helper()
	store := reg.NewMetricsStoreWithConfig(reg.MetricsStoreConfig{MaxMemoryEntries: 2, MaxTotalEntries: 10})
	ctx := &ServiceContext{Config: config.Config{Herald: cfg}, MetricsStore: store}
	return ctx, store
}

func TestRegisterHeraldOutletDisabledByDefault(t *testing.T) {
	t.Setenv("HERALD_TRIGGER_TOKEN", "tok")
	ctx, store := svcWithHerald(t, config.HeraldConfig{Enabled: false, BaseURL: "http://herald:8080"})
	registerHeraldOutlet(ctx)

	// 关=现状不变：无 hook（喂事件不 panic 即可）。
	store.Add("agent-1", &opsv1.MetricsReport{SupervisorEvents: []*opsv1.SupervisorEvent{
		{Seq: 1, TsUnix: 100, Event: "breaker_tripped"},
	}})
}

func TestRegisterHeraldOutletMissingBaseUrlSkips(t *testing.T) {
	t.Setenv("HERALD_TRIGGER_TOKEN", "tok")
	ctx, store := svcWithHerald(t, config.HeraldConfig{Enabled: true})
	registerHeraldOutlet(ctx)

	// 未注册出口：事件照常入库（无 hook，无 panic）。
	store.Add("agent-1", &opsv1.MetricsReport{SupervisorEvents: []*opsv1.SupervisorEvent{
		{Seq: 1, TsUnix: 100, Event: "breaker_tripped"},
	}})
}

func TestRegisterHeraldOutletMissingTokenSkips(t *testing.T) {
	// HERALD_TRIGGER_TOKEN 不设置：降级告警日志，不注册出口。
	t.Setenv("HERALD_TRIGGER_TOKEN", "")
	ctx, _ := svcWithHerald(t, config.HeraldConfig{Enabled: true, BaseURL: "http://herald:8080"})
	registerHeraldOutlet(ctx)
}

func TestSupervisorEventToOutletIgnoresNonBreaker(t *testing.T) {
	// 非 breaker_tripped 与 nil 事件静默忽略（闭集外零消费）；manager 为
	// nil 兜底不 panic。
	man := outlet.NewManager()
	hook := supervisorEventToOutlet(man)
	hook(context.Background(), "agent-1", &opsv1.SupervisorEvent{Seq: 1, Event: "detect_down"})
	hook(context.Background(), "agent-1", nil)
	nilHook := supervisorEventToOutlet(nil)
	nilHook(context.Background(), "agent-1", &opsv1.SupervisorEvent{Seq: 2, Event: "breaker_tripped"})
}

func TestSupervisorEventToOutletEnvelopeFields(t *testing.T) {
	ch := make(chan outlet.AlertEvent, 4)
	man := outlet.NewManager()
	man.Register(&captureOutlet{out: ch})
	hook := supervisorEventToOutlet(man)
	waitFor := func() outlet.AlertEvent {
		select {
		case ev := <-ch:
			return ev
		case <-time.After(2 * time.Second):
			t.Fatal("no envelope delivered within 2s")
			return outlet.AlertEvent{}
		}
	}

	hook(context.Background(), "agent-1", &opsv1.SupervisorEvent{
		Seq:     42,
		TsUnix:  1760000000,
		Event:   "breaker_tripped",
		Process: "gameserver",
		Message: "too many restart failures",
	})

	ev := waitFor()
	if ev.Kind != outlet.KindSupervisorBreaker || ev.Severity != outlet.SeverityCritical {
		t.Fatalf("kind/severity = %s/%s", ev.Kind, ev.Severity)
	}
	if ev.EventID != "agent-1:42" {
		t.Fatalf("EventID = %s, want agent-1:42（seq 跨 agent 重启归零，必携前缀）", ev.EventID)
	}
	if ev.DedupKey != "supervisor.breaker_tripped:agent-1:42" {
		t.Fatalf("DedupKey = %s", ev.DedupKey)
	}
	if ev.Title == "" || ev.Body != "too many restart failures" {
		t.Fatalf("title/body = %q / %q", ev.Title, ev.Body)
	}
	if ev.OccurredAtUnixMs != 1760000000000 {
		t.Fatalf("OccurredAtUnixMs = %d", ev.OccurredAtUnixMs)
	}
	if ev.Scope.AgentID != "agent-1" {
		t.Fatalf("Scope.AgentID = %s", ev.Scope.AgentID)
	}

	// message 空 → body 兜底文案。
	hook(context.Background(), "agent-1", &opsv1.SupervisorEvent{
		Seq: 43, TsUnix: 1760000001, Event: "breaker_tripped", Process: "gameserver",
	})
	if ev := waitFor(); ev.Body == "" {
		t.Fatal("empty message must fall back to default body")
	}
}

// captureOutlet 把信封送进 channel（测试专用，跨 goroutine 无共享变量）。
type captureOutlet struct {
	out chan<- outlet.AlertEvent
}

func (c *captureOutlet) Name() string { return "capture" }

func (c *captureOutlet) Deliver(ctx context.Context, ev outlet.AlertEvent) error {
	c.out <- ev
	return nil
}
