// 维护状态 gate 装配层覆盖（server-status-provider 设计 §5）：
// config→serverstatus.Options 逐项映射（tokenEnv 解析、matchBy 回落、
// 超时/路径/字段映射透传）、三分支接线（抑制整链静默 / 未知标注照报）、
// 旁路语义（enabled=false 与未注册 provider 不挂 gate）。
package svc

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	"github.com/cuihairu/croupier/internal/platform/serverstatus"
)

// fakeStatusProvider 记录装配得到的 Options，状态与错误可编程。calls 用
// atomic：GetServerStatus 在 Dispatch 的异步 goroutine 里被调（-race 下
// 主 goroutine 直读 int 即 DATA RACE）。
type fakeStatusProvider struct {
	status serverstatus.ServerStatus
	err    error
	opts   serverstatus.Options
	calls  atomic.Int64
}

func (f *fakeStatusProvider) Name() string { return "test-fake" }

func (f *fakeStatusProvider) GetServerStatus(context.Context, serverstatus.ServerRef) (serverstatus.ServerStatus, error) {
	f.calls.Add(1)
	return f.status, f.err
}

func (f *fakeStatusProvider) ListServers(context.Context) ([]serverstatus.ServerSummary, error) {
	return nil, nil
}

func registerFakeProvider(t *testing.T, fake *fakeStatusProvider) {
	t.Helper()
	serverstatus.RegisterFactory("test-fake", func(opts serverstatus.Options) serverstatus.Provider {
		fake.opts = opts
		return fake
	})
	t.Cleanup(func() { serverstatus.RegisterFactory("test-fake", nil) })
}

func serverStatusConfig(matchBy string) config.Config {
	return config.Config{
		ServerStatus: config.ServerStatusConfig{
			Enabled:         true,
			Provider:        "test-fake",
			MatchBy:         matchBy,
			CacheTtlSeconds: 60,
			Providers: map[string]config.ServerStatusSourceConfig{
				"test-fake": {
					BaseURL:      "http://atlas.test:8080",
					TokenEnv:     "CROUPIER_TEST_ATLAS_TOKEN",
					TimeoutMs:    1500,
					StatusPath:   "/status",
					FieldMapping: map[string]string{"inMaintenance": "frozen"},
				},
			},
		},
	}
}

func agentEnvelope() outlet.AlertEvent {
	return outlet.AlertEvent{
		Kind:     outlet.KindSupervisorBreaker,
		Severity: outlet.SeverityCritical,
		EventID:  "agent-1:42",
		Body:     "too many restart failures",
		Scope:    outlet.Scope{AgentID: "agent-1", GameID: "game_demo", Env: "prod"},
	}
}

func TestWireServerStatusGateNilSafe(t *testing.T) {
	wireServerStatusGate(nil)
	wireServerStatusGate(&ServiceContext{}) // OutletManager 未布线
}

func TestInstallServerStatusGateMapsOptions(t *testing.T) {
	fake := &fakeStatusProvider{status: serverstatus.ServerStatus{InMaintenance: true}}
	registerFakeProvider(t, fake)
	t.Setenv("CROUPIER_TEST_ATLAS_TOKEN", "tok-123")

	m := outlet.NewManager()
	installServerStatusGate(serverStatusConfig(""), m)

	got := fake.opts
	if got.BaseURL != "http://atlas.test:8080" || got.Token != "tok-123" ||
		got.Timeout != 1500*time.Millisecond || got.Atlas.MatchKey != "agentId" ||
		got.Atlas.StatusPath != "/status" || got.Atlas.Fields["inMaintenance"] != "frozen" {
		t.Fatalf("opts = %+v", got)
	}

	// InMaintenance → 整链静默：出口零投递，provider 恰好回源一次。
	ch := make(chan outlet.AlertEvent, 4)
	m.Register(&captureOutlet{out: ch})
	m.Dispatch(agentEnvelope())
	select {
	case ev := <-ch:
		t.Fatalf("suppressed event reached outlet: %+v", ev)
	case <-time.After(200 * time.Millisecond):
	}
	if got := fake.calls.Load(); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
}

// matchBy 非 agentId：v1 事件 scope 只带 agentId，回落 agentId（不静默忽略配置）。
func TestInstallServerStatusGateMatchByHostFallsBack(t *testing.T) {
	fake := &fakeStatusProvider{}
	registerFakeProvider(t, fake)
	m := outlet.NewManager()
	installServerStatusGate(serverStatusConfig("host"), m)
	if fake.opts.Atlas.MatchKey != "agentId" {
		t.Fatalf("MatchKey = %q, want agentId fallback", fake.opts.Atlas.MatchKey)
	}
}

// 未知状态（provider 错误）→ 照报 + 信封标注 source_unknown（fail-open）。
func TestInstallServerStatusGateUnknownAnnotatesEnvelope(t *testing.T) {
	fake := &fakeStatusProvider{
		err: fmt.Errorf("%w: atlas http 503", serverstatus.ErrProviderUnavailable),
	}
	registerFakeProvider(t, fake)

	m := outlet.NewManager()
	ch := make(chan outlet.AlertEvent, 4)
	m.Register(&captureOutlet{out: ch})
	installServerStatusGate(serverStatusConfig(""), m)

	m.Dispatch(agentEnvelope())
	select {
	case ev := <-ch:
		if got := ev.Metadata[outlet.MetaSourceUnknown]; got != "true" {
			t.Fatalf("Metadata[%s] = %q, want true", outlet.MetaSourceUnknown, got)
		}
		if ev.Metadata[outlet.MetaMaintenanceSuppressed] != "" {
			t.Fatalf("Metadata must not carry suppression flag on a delivered event: %v", ev.Metadata)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fail-open event must still reach the outlet")
	}
}

// 旁路：enabled=false 与未注册 provider 都不挂 gate，事件直投零标注。
func TestInstallServerStatusGateBypassed(t *testing.T) {
	for name, cfg := range map[string]config.Config{
		"disabled": {ServerStatus: config.ServerStatusConfig{Enabled: false, Provider: "test-fake"}},
		"unknown":  {ServerStatus: config.ServerStatusConfig{Enabled: true, Provider: "bogus"}},
	} {
		m := outlet.NewManager()
		ch := make(chan outlet.AlertEvent, 4)
		m.Register(&captureOutlet{out: ch})
		installServerStatusGate(cfg, m)

		m.Dispatch(agentEnvelope())
		select {
		case ev := <-ch:
			if len(ev.Metadata) != 0 {
				t.Fatalf("%s: bypassed event must not be annotated: %v", name, ev.Metadata)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s: bypassed event must reach the outlet", name)
		}
	}
}
