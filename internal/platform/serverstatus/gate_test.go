package serverstatus

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// fakeProvider 可编程替身（记录回源次数，供缓存断言）。
type fakeProvider struct {
	name   string
	status ServerStatus
	err    error
	calls  int
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) GetServerStatus(context.Context, ServerRef) (ServerStatus, error) {
	f.calls++
	return f.status, f.err
}

func (f *fakeProvider) ListServers(context.Context) ([]ServerSummary, error) {
	return []ServerSummary{{AgentID: "a1", Host: "h1"}}, nil
}

func TestGate_SuppressInMaintenance(t *testing.T) {
	fp := &fakeProvider{name: "fake", status: ServerStatus{InMaintenance: true, Source: "fake"}}
	g := NewGate(fp, NewCache(0), nil)
	ctx := context.Background()
	ref := ServerRef{AgentID: "agent-1"}

	v := g.Evaluate(ctx, ref)
	if !v.Suppressed || v.SourceUnknown || v.Source != "fake" {
		t.Fatalf("verdict = %+v", v)
	}
	if s, u, q := g.Counters(); s != 1 || u != 0 || q != 1 {
		t.Fatalf("counters = %d/%d/%d", s, u, q)
	}
}

func TestGate_PassWhenNotMaintaining(t *testing.T) {
	fp := &fakeProvider{name: "fake", status: ServerStatus{Healthy: true, InMaintenance: false, Source: "fake"}}
	g := NewGate(fp, NewCache(0), nil)
	v := g.Evaluate(context.Background(), ServerRef{AgentID: "agent-1"})
	if v.Suppressed || v.SourceUnknown || v.Source != "fake" {
		t.Fatalf("verdict = %+v", v)
	}
	if fp.calls != 1 {
		t.Fatalf("calls = %d", fp.calls)
	}
}

func TestGate_UnknownFailsOpen(t *testing.T) {
	fp := &fakeProvider{name: "fake", err: fmt.Errorf("%w: atlas http 503", ErrProviderUnavailable)}
	g := NewGate(fp, NewCache(0), nil)
	v := g.Evaluate(context.Background(), ServerRef{AgentID: "agent-1"})
	if v.Suppressed || !v.SourceUnknown {
		t.Fatalf("verdict = %+v, want fail-open with source_unknown", v)
	}
	// 错误不缓存：第二次查询仍回源，仍 fail-open
	if v2 := g.Evaluate(context.Background(), ServerRef{AgentID: "agent-1"}); !v2.SourceUnknown {
		t.Fatalf("second verdict = %+v, want fail-open", v2)
	}
	if fp.calls != 2 {
		t.Fatalf("calls = %d, want 2 (errors not cached)", fp.calls)
	}
	if s, u, q := g.Counters(); s != 0 || u != 2 || q != 2 {
		t.Fatalf("counters = %d/%d/%d", s, u, q)
	}
}

func TestGate_NoMappingFailsOpen(t *testing.T) {
	fp := &fakeProvider{name: "fake", err: fmt.Errorf("%w: no agentId=a1 row", ErrNoMapping)}
	g := NewGate(fp, NewCache(0), nil)
	v := g.Evaluate(context.Background(), ServerRef{AgentID: "agent-1"})
	if v.Suppressed || !v.SourceUnknown {
		t.Fatalf("verdict = %+v", v)
	}
}

func TestGate_BypassedWhenNoProviderOrNoRef(t *testing.T) {
	fp := &fakeProvider{name: "fake"}
	for name, tc := range map[string]struct {
		gate *Gate
		ref  ServerRef
	}{
		"nil provider": {NewGate(nil, NewCache(0), nil), ServerRef{AgentID: "a1"}},
		"empty ref":    {NewGate(fp, NewCache(0), nil), ServerRef{}},
	} {
		g, ref := tc.gate, tc.ref
		if g.Evaluate(context.Background(), ref) != (Verdict{}) {
			t.Fatalf("%s: gate must bypass", name)
		}
	}
	if fp.calls != 0 {
		t.Fatalf("calls = %d, want 0", fp.calls)
	}
}

func TestGate_CacheAvoidsSecondQuery(t *testing.T) {
	fp := &fakeProvider{name: "fake", status: ServerStatus{Healthy: true, Source: "fake"}}
	g := NewGate(fp, NewCache(time.Second), nil)
	ref := ServerRef{AgentID: "agent-1"}
	for i := 0; i < 3; i++ {
		v := g.Evaluate(context.Background(), ref)
		if v.Suppressed || v.SourceUnknown || v.Source != "fake" {
			t.Fatalf("verdict[%d] = %+v", i, v)
		}
	}
	if fp.calls != 1 {
		t.Fatalf("calls = %d, want 1 (cached)", fp.calls)
	}
	if s, _, q := g.Counters(); s != 0 || q != 1 {
		t.Fatalf("counters = %d/%d", s, q)
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	var now int64 = time.Now().Unix()
	c := NewCache(60 * time.Second)
	c.now = func() time.Time { return time.Unix(now, 0) }
	ref := ServerRef{AgentID: "agent-1"}
	st := ServerStatus{Healthy: true, Source: "fake"}

	if _, ok := c.Get(ref); ok {
		t.Fatal("cache should be empty")
	}
	c.Set(ref, st)
	if got, ok := c.Get(ref); !ok || got.Source != "fake" {
		t.Fatalf("hit = %v %+v", ok, got)
	}
	// TTL 内命中
	now += 59
	if _, ok := c.Get(ref); !ok {
		t.Fatal("should still hit within TTL")
	}
	// 过期失效
	now += 2
	if _, ok := c.Get(ref); ok {
		t.Fatal("should expire after TTL")
	}
	// 不同 ref 不串扰
	c.Set(ServerRef{Host: "h1"}, ServerStatus{Healthy: false})
	if got, ok := c.Get(ref); ok || got.Source == "fake" {
		t.Fatalf("cross ref hit = %v %+v", ok, got)
	}
}

func TestCache_DefaultTTL(t *testing.T) {
	if c := NewCache(0); c.ttl != 60*time.Second {
		t.Fatalf("ttl = %v", c.ttl)
	}
	if c := NewCache(-5); c.ttl != 60*time.Second {
		t.Fatalf("negative ttl = %v", c.ttl)
	}
}

func TestRegistry(t *testing.T) {
	for _, name := range []string{ProviderNoop, ProviderAtlas} {
		if Lookup(name) == nil {
			t.Fatalf("factory %q not registered", name)
		}
	}
	if Lookup("bogus") != nil {
		t.Fatal("unknown key must be nil")
	}
	names := Names()
	if len(names) < 2 {
		t.Fatalf("names = %v", names)
	}
	// 覆盖注册（测试注入替身）
	RegisterFactory("test-fake", func(Options) Provider { return &fakeProvider{name: "test-fake"} })
	defer RegisterFactory("test-fake", nil)
	if p := Lookup("test-fake")(Options{}); p == nil || p.Name() != "test-fake" {
		t.Fatalf("fake provider = %v", p)
	}
	if p := Lookup(ProviderNoop)(Options{}); p == nil || p.Name() != ProviderNoop {
		t.Fatalf("noop = %v", p)
	}
}

func TestNoopProvider(t *testing.T) {
	p := NewNoopProvider()
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "a1"})
	if err != nil || st.InMaintenance || st.Source != ProviderNoop {
		t.Fatalf("noop = %+v err=%v", st, err)
	}
	if !errors.Is(err, nil) {
		t.Fatal("noop must not error")
	}
	servers, err := p.ListServers(context.Background())
	if err != nil || len(servers) != 0 {
		t.Fatalf("list = %v err=%v", servers, err)
	}
	// no-op 语义：gate 恒通过
	g := NewGate(p, NewCache(0), nil)
	if v := g.Evaluate(context.Background(), ServerRef{AgentID: "a1"}); v.Suppressed || v.SourceUnknown {
		t.Fatalf("noop verdict = %+v, want plain pass", v)
	}
}
