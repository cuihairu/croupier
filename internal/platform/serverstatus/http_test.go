package serverstatus

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"

	"testing"
	"time"
)

// 注册表：http 工厂已内置注册，产物 Name 即注册表键。
func TestHTTPProvider_Registered(t *testing.T) {
	factory := Lookup(ProviderHTTP)
	if factory == nil {
		t.Fatal("http factory not registered")
	}
	if got := factory(Options{BaseURL: "http://x"}).Name(); got != ProviderHTTP {
		t.Fatalf("name = %q, want %q", got, ProviderHTTP)
	}
}

// 缺省口径：StatusPath 空=baseUrl 即端点（query 参数定位），Fields 空=
// 契约键名直读，bool inMaintenance 直取——「任意能返回状态 JSON 的简单
// 服务」零映射配置可用。
func TestHTTPProvider_ZeroConfigContractKeys(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" || r.URL.Query().Get("agentId") != "agent-7" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"inMaintenance": true, "healthy": true}`))
	}))
	defer srv.Close()

	p := NewHTTPProvider(Options{BaseURL: srv.URL, Token: "tok-1"})
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "agent-7"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !st.InMaintenance || !st.Healthy || st.Source != ProviderHTTP {
		t.Fatalf("status = %+v", st)
	}
	if gotAuth != "Bearer tok-1" {
		t.Fatalf("auth header = %q", gotAuth)
	}
}

// 单对象响应不含匹配键字段 → 整体接受（URL 即标识：单服务器状态端点无需
// 回显自身 id）；携带但值不符 → 无映射。数组响应无此豁免。
func TestHTTPProvider_SingleObjectMatching(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"frozen": true}`))
	}))
	defer srv.Close()
	p := NewHTTPProvider(Options{
		BaseURL: srv.URL,
		Atlas:   AtlasOptions{Fields: map[string]string{"inMaintenance": "frozen"}},
	})
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "agent-7"})
	if err != nil || !st.InMaintenance {
		t.Fatalf("url-as-identity: st = %+v err = %v", st, err)
	}

	mismatch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"agentId":"other","frozen": false}`))
	}))
	defer mismatch.Close()
	p2 := NewHTTPProvider(Options{BaseURL: mismatch.URL})
	if _, err := p2.GetServerStatus(context.Background(), ServerRef{AgentID: "agent-7"}); !errors.Is(err, ErrNoMapping) {
		t.Fatalf("mismatch err = %v, want ErrNoMapping", err)
	}
}

// 数组响应按匹配字段过滤取首条（同 atlas 口径）。
func TestHTTPProvider_ArrayFilteredByMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"agentId":"other","inMaintenance":true},{"agentId":"agent-7","inMaintenance":false}]`))
	}))
	defer srv.Close()
	p := NewHTTPProvider(Options{BaseURL: srv.URL})
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "agent-7"})
	if err != nil || st.InMaintenance || st.Source != ProviderHTTP {
		t.Fatalf("st = %+v err = %v", st, err)
	}
}

// 字符串布尔缺省 "true"：简易源常见字面布尔串；枚举型服务配置
// inMaintenanceStatus 切口径。
func TestHTTPProvider_StringStatusForms(t *testing.T) {
	mk := func(body string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
	}
	// 缺省 "true" 串判等
	srv := mk(`{"inMaintenance":"true"}`)
	defer srv.Close()
	p := NewHTTPProvider(Options{BaseURL: srv.URL})
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "a"})
	if err != nil || !st.InMaintenance {
		t.Fatalf(`literal "true": st = %+v err = %v`, st, err)
	}
	// 非 "true" 串不在维护
	srv2 := mk(`{"inMaintenance":"draining"}`)
	defer srv2.Close()
	p2 := NewHTTPProvider(Options{BaseURL: srv2.URL})
	if st, err := p2.GetServerStatus(context.Background(), ServerRef{AgentID: "a"}); err != nil || st.InMaintenance {
		t.Fatalf("non-matching enum: st = %+v err = %v", st, err)
	}
	// 枚举口径可配
	srv3 := mk(`{"inMaintenance":"draining"}`)
	defer srv3.Close()
	p3 := NewHTTPProvider(Options{
		BaseURL: srv3.URL,
		Atlas:   AtlasOptions{InMaintenanceStatus: "draining"},
	})
	if st, err := p3.GetServerStatus(context.Background(), ServerRef{AgentID: "a"}); err != nil || !st.InMaintenance {
		t.Fatalf("configured enum: st = %+v err = %v", st, err)
	}
}

// 子路径与字段映射：statusPath 拼在 baseUrl 后，window 三字段映射照单全收。
func TestHTTPProvider_StatusPathAndWindow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"state":"maintenance","ok":false,"from":"2026-10-12T00:00:00Z","to":"2026-10-13T00:00:00Z","why":"发版"}`))
	}))
	defer srv.Close()
	p := NewHTTPProvider(Options{
		BaseURL: srv.URL,
		Atlas: AtlasOptions{
			StatusPath:          "/api/status",
			InMaintenanceStatus: "maintenance",
			Fields: map[string]string{
				"inMaintenance": "state",
				"healthy":       "ok",
				"windowStart":   "from",
				"windowEnd":     "to",
				"windowNote":    "why",
			},
		},
	})
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "a"})
	if err != nil || !st.InMaintenance || st.Healthy {
		t.Fatalf("st = %+v err = %v", st, err)
	}
	if st.Window == nil || st.Window.Note != "发版" || st.Window.Start.IsZero() || st.Window.End.IsZero() {
		t.Fatalf("window = %+v", st.Window)
	}
}

// statusPath 含 {id} 占位符=路径参数口径（与 atlas 同引擎）。
func TestHTTPProvider_PathPlaceholder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/servers/agent-7" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"inMaintenance": true}`))
	}))
	defer srv.Close()
	p := NewHTTPProvider(Options{
		BaseURL: srv.URL,
		Atlas:   AtlasOptions{StatusPath: "/servers/{id}"},
	})
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "agent-7"})
	if err != nil || !st.InMaintenance {
		t.Fatalf("st = %+v err = %v", st, err)
	}
}

// baseUrl 已带固定 query 时用 & 续接定位参数（简易源常见形态）。
func TestHTTPProvider_BaseURLWithQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("slot") != "1" || r.URL.Query().Get("agentId") != "agent-7" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"inMaintenance": false}`))
	}))
	defer srv.Close()
	p := NewHTTPProvider(Options{BaseURL: srv.URL + "/status?slot=1"})
	if _, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "agent-7"}); err != nil {
		t.Fatalf("query continuation: %v", err)
	}
}

// 降级口径（与 atlas 同）：baseURL 未配置 / HTTP 500 / 超时 / 坏 JSON →
// ErrProviderUnavailable；ref 无定位键 → ErrNoMapping。
func TestHTTPProvider_FailsOpen(t *testing.T) {
	p := NewHTTPProvider(Options{})
	if _, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "a"}); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("unconfigured err = %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	p2 := NewHTTPProvider(Options{BaseURL: srv.URL})
	if _, err := p2.GetServerStatus(context.Background(), ServerRef{AgentID: "a"}); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("http500 err = %v", err)
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer slow.Close()
	p3 := NewHTTPProvider(Options{BaseURL: slow.URL, Timeout: 10 * time.Millisecond})
	if _, err := p3.GetServerStatus(context.Background(), ServerRef{AgentID: "a"}); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("timeout err = %v", err)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer bad.Close()
	p4 := NewHTTPProvider(Options{BaseURL: bad.URL})
	if _, err := p4.GetServerStatus(context.Background(), ServerRef{AgentID: "a"}); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("bad json err = %v", err)
	}
	p5 := NewHTTPProvider(Options{BaseURL: srv.URL})
	if _, err := p5.GetServerStatus(context.Background(), ServerRef{}); !errors.Is(err, ErrNoMapping) {
		t.Fatalf("empty ref err = %v", err)
	}
}

// ListServers：数组与 {servers|items} 包裹均可；单服务器状态对象解出空
// 清单不报错（接口允许空清单）。
func TestHTTPProvider_ListServers(t *testing.T) {
	array := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"agentId":"a1","host":"h1","healthy":true,"inMaintenance":false}]`))
	}))
	defer array.Close()
	p := NewHTTPProvider(Options{BaseURL: array.URL})
	rows, err := p.ListServers(context.Background())
	if err != nil || len(rows) != 1 || rows[0].AgentID != "a1" || rows[0].Host != "h1" || !rows[0].Healthy {
		t.Fatalf("array rows = %+v err = %v", rows, err)
	}

	wrapped := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"agentId":"a2","inMaintenance":true}]}`))
	}))
	defer wrapped.Close()
	p2 := NewHTTPProvider(Options{BaseURL: wrapped.URL})
	rows2, err := p2.ListServers(context.Background())
	if err != nil || len(rows2) != 1 || !rows2[0].InMaintenance {
		t.Fatalf("wrapped rows = %+v err = %v", rows2, err)
	}

	single := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"inMaintenance": false}`))
	}))
	defer single.Close()
	p3 := NewHTTPProvider(Options{BaseURL: single.URL})
	rows3, err := p3.ListServers(context.Background())
	if err != nil || len(rows3) != 0 {
		t.Fatalf("single-object rows = %+v err = %v", rows3, err)
	}
}

// gate 全链（http 源 stub）：三分支判定与 atlas 同口径。分支各用独立
// agentId，避开同 ref 的 TTL 缓存（缓存内命中不回源）。
func TestGate_WithHTTPStub(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("agentId") {
		case "free":
			_, _ = w.Write([]byte(`{"inMaintenance": false}`))
		case "maintained":
			_, _ = w.Write([]byte(`{"inMaintenance": true}`))
		default:
			_, _ = w.Write([]byte(`[{"agentId":"someone-else","inMaintenance":true}]`))
		}
	}))
	defer srv.Close()
	g := NewGate(NewHTTPProvider(Options{BaseURL: srv.URL}), NewCache(0), nil)
	ctx := context.Background()

	if v := g.Evaluate(ctx, ServerRef{AgentID: "free"}); v.Suppressed || v.SourceUnknown || v.Source != ProviderHTTP {
		t.Fatalf("running verdict = %+v", v)
	}
	if v := g.Evaluate(ctx, ServerRef{AgentID: "maintained"}); !v.Suppressed || v.Source != ProviderHTTP {
		t.Fatalf("maintenance verdict = %+v", v)
	}
	if v := g.Evaluate(ctx, ServerRef{AgentID: "ghost"}); v.Suppressed || !v.SourceUnknown {
		t.Fatalf("no-row verdict = %+v", v)
	}
}
