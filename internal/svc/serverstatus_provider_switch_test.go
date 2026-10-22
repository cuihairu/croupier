// 批 S4 切换实测：同一 gate 装配面（installServerStatusGate）下
// atlas ↔ http ↔ noop 只改配置切换。每种源真打一个 httptest 测试源，覆盖
// 状态判定三态与降级：在维护=整链静默 / 不在维护=照投零标注 /
// 源挂掉=fail-open 照投标 source_unknown。
package svc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	"github.com/cuihairu/croupier/internal/platform/serverstatus"
)

// switchConfig 拼切换配置：除 provider 键与对应 providers.<key> 段外全同
// ——「同一 gate 面，只改配置切换」的实证。
func switchConfig(t *testing.T, provider, baseURL string, tune func(*config.ServerStatusSourceConfig)) config.Config {
	t.Helper()
	key := provider
	if key == "" {
		key = serverstatus.ProviderAtlas // provider 空=缺省 atlas（批 S4）
	}
	src := config.ServerStatusSourceConfig{BaseURL: baseURL}
	if tune != nil {
		tune(&src)
	}
	return config.Config{ServerStatus: config.ServerStatusConfig{
		Enabled:         true,
		Provider:        provider,
		CacheTtlSeconds: 60,
		Providers: map[string]config.ServerStatusSourceConfig{
			key: src,
		},
	}}
}

// dispatchExpect 投一条 agent 定位事件，断言到达形态（wantSuppressed=整链
// 静默零投递；否则必须到达且 metadata 标注符合预期）。
func dispatchExpect(t *testing.T, cfg config.Config, wantSuppressed, wantSourceUnknown bool) {
	t.Helper()
	m := outlet.NewManager()
	ch := make(chan outlet.AlertEvent, 4)
	m.Register(&captureOutlet{out: ch})
	installServerStatusGate(cfg, m)

	m.Dispatch(agentEnvelope())
	if wantSuppressed {
		select {
		case ev := <-ch:
			t.Fatalf("suppressed event reached outlet: %+v", ev)
		case <-time.After(200 * time.Millisecond):
		}
		return
	}
	select {
	case ev := <-ch:
		if wantSourceUnknown && ev.Metadata[outlet.MetaSourceUnknown] != "true" {
			t.Fatalf("Metadata[source_unknown] = %q, want true: %v", ev.Metadata[outlet.MetaSourceUnknown], ev.Metadata)
		}
		if !wantSourceUnknown && len(ev.Metadata) != 0 {
			t.Fatalf("clean delivery must not be annotated: %v", ev.Metadata)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("event must reach the outlet")
	}
}

// 缺省源=atlas（provider 字段留空），atlas 缺省口径一字段不配（只有
// baseUrl）：/v1/discovery/servers/{id} 路径参数 + status 枚举判定——
// 在维护 → 整链静默。
func TestProviderSwitchDefaultAtlasSuppresses(t *testing.T) {
	var hits atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		id := strings.TrimPrefix(r.URL.Path, "/v1/discovery/servers/")
		_, _ = w.Write([]byte(`{"id":"` + id + `","status":"maintenance"}`))
	}))
	defer ts.Close()

	dispatchExpect(t, switchConfig(t, "", ts.URL, nil), true, false)
	if got := hits.Load(); got != 1 {
		t.Fatalf("atlas source hits = %d, want 1", got)
	}
}

// 同一缺省源，状态=online（不在维护）→ 照投零标注。
func TestProviderSwitchDefaultAtlasRunningDelivers(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/v1/discovery/servers/")
		_, _ = w.Write([]byte(`{"id":"` + id + `","status":"online"}`))
	}))
	defer ts.Close()

	dispatchExpect(t, switchConfig(t, "", ts.URL, nil), false, false)
}

// 切 http 源（只改 provider 键 + providers.http 段）：简易状态 JSON
// （自定义字段名 + fieldMapping + bool 维护位）→ 在维护 → 整链静默。
func TestProviderSwitchHTTPSuppresses(t *testing.T) {
	var gotMatch string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotMatch = r.URL.Query().Get("agentId")
		_, _ = w.Write([]byte(`{"frozen": true}`))
	}))
	defer ts.Close()

	cfg := switchConfig(t, "http", ts.URL, func(s *config.ServerStatusSourceConfig) {
		s.StatusPath = "/status.json"
		s.FieldMapping = map[string]string{"inMaintenance": "frozen"}
	})
	dispatchExpect(t, cfg, true, false)
	if gotMatch != "agent-1" {
		t.Fatalf("http source match param = %q, want agent-1", gotMatch)
	}
}

// 降级实测（http 源挂掉：连接拒绝）→ fail-open 照投 + source_unknown。
func TestProviderSwitchHTTPDownDeliversUnknown(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	base := ts.URL
	ts.Close() // 源挂掉：端口无人监听

	cfg := switchConfig(t, "http", base, func(s *config.ServerStatusSourceConfig) {
		s.TimeoutMs = 1500
	})
	dispatchExpect(t, cfg, false, true)
}

// 切 noop（只改 provider 键，无需任何状态源）：恒无维护信息 → 照投零标注。
func TestProviderSwitchNoopDelivers(t *testing.T) {
	dispatchExpect(t, switchConfig(t, "noop", "", nil), false, false)
}

// 缺省 atlas 但 baseUrl 从未配置：装配旁路不挂 gate（fail-open 只针对
// 「源挂了」，不针对「从未配置」）——照投零标注，与 enabled=false 同形。
func TestProviderSwitchDefaultAtlasNoBaseURLBypasses(t *testing.T) {
	cfg := config.Config{ServerStatus: config.ServerStatusConfig{Enabled: true}}
	dispatchExpect(t, cfg, false, false)
}
