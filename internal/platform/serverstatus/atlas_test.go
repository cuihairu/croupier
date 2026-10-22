package serverstatus

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// 默认口径=atlas 真实 API（2026-10-11 对齐）：GET /v1/discovery/servers/{id}
// 路径参数定位（discovery 无需鉴权），单对象响应，status 字符串枚举
// （"maintenance"=在维护），id 字段定位行。
func TestAtlas_DiscoveryObjectResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/discovery/servers/agent-7" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok-1" {
			t.Fatalf("auth header = %q", got)
		}
		_, _ = w.Write([]byte(`{"id":"agent-7","name":"s7","status":"maintenance","players":3}`))
	}))
	defer srv.Close()

	p := NewAtlasProvider(Options{BaseURL: srv.URL, Token: "tok-1"})
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "agent-7"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !st.InMaintenance || st.Healthy || st.Source != ProviderAtlas {
		t.Fatalf("status = %+v", st)
	}
	if st.Window != nil {
		t.Fatalf("discovery has no window fields, got %+v", st.Window)
	}
}

// query 参数口径兜底（StatusPath 无 {id} 占位符）：数组响应 + bool
// inMaintenance + window 三字段（供应商侧若给出窗口字段照单全收）。
func TestAtlas_QueryParamArrayResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/servers" || r.URL.Query().Get("agentId") != "agent-7" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`[{"id":"agent-7","healthy":true,"inMaintenance":true,"windowStart":"2026-10-11T00:00:00Z","windowEnd":"2026-10-12T00:00:00Z","windowNote":"发版"}]`))
	}))
	defer srv.Close()

	p := NewAtlasProvider(Options{
		BaseURL: srv.URL,
		Atlas: AtlasOptions{
			StatusPath: "/api/servers",
			Fields:     map[string]string{"inMaintenance": "inMaintenance"}, // 布尔键行显式映射（默认走 status 枚举）
		},
	})
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "agent-7"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !st.InMaintenance || !st.Healthy {
		t.Fatalf("status = %+v", st)
	}
	if st.Window == nil || st.Window.Note != "发版" || st.Window.Start.IsZero() || st.Window.End.IsZero() {
		t.Fatalf("window = %+v", st.Window)
	}
}

// 在维护的另一种口径：映射字段是 bool（非枚举）直取。
func TestAtlas_FieldMappingBool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"agent-7","frozen":true}`))
	}))
	defer srv.Close()

	p := NewAtlasProvider(Options{
		BaseURL: srv.URL,
		Atlas: AtlasOptions{
			Fields: map[string]string{
				"inMaintenance": "frozen",
			},
		},
	})
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "agent-7"})
	if err != nil || !st.InMaintenance {
		t.Fatalf("st = %+v err = %v", st, err)
	}
}

// 配置只映射部分键：默认映射（agentId→id / inMaintenance→status）保留，
// 只覆盖写到的键。
func TestAtlas_PartialFieldMappingMergesDefaults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"agent-7","status":"online"}]`))
	}))
	defer srv.Close()

	p := NewAtlasProvider(Options{
		BaseURL: srv.URL,
		Atlas: AtlasOptions{
			StatusPath: "/api/servers", // query 参数口径
			Fields:     map[string]string{"healthy": "players_ok"},
		},
	})
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "agent-7"})
	if err != nil || st.InMaintenance {
		t.Fatalf("st = %+v err = %v（默认 status→maintenance 枚举须仍生效）", st, err)
	}
}

func TestAtlas_ObjectResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"agent-7","status":"online"}`))
	}))
	defer srv.Close()
	p := NewAtlasProvider(Options{BaseURL: srv.URL})
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "agent-7"})
	if err != nil || st.InMaintenance {
		t.Fatalf("status = %+v err=%v", st, err)
	}
}

// draining/suspect/offline 不是维护（维护=运维显式声明，告警噪音只有
// 维护窗一种）。
func TestAtlas_NonMaintenanceStatusesPass(t *testing.T) {
	for _, status := range []string{"online", "draining", "suspect", "offline", "starting"} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"id":"a","status":"` + status + `"}`))
		}))
		p := NewAtlasProvider(Options{BaseURL: srv.URL})
		st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "a"})
		if err != nil || st.InMaintenance {
			srv.Close()
			t.Fatalf("status %q: st = %+v err = %v, want not in maintenance", status, st, err)
		}
		srv.Close()
	}
}

// InMaintenanceStatus 可配：换枚举口径不改代码。
func TestAtlas_InMaintenanceStatusConfigurable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"a","status":"draining"}`))
	}))
	defer srv.Close()
	p := NewAtlasProvider(Options{
		BaseURL: srv.URL,
		Atlas:   AtlasOptions{InMaintenanceStatus: "draining"},
	})
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "a"})
	if err != nil || !st.InMaintenance {
		t.Fatalf("st = %+v err = %v, want draining counted as maintenance", st, err)
	}
}

func TestAtlas_NoMappingFailsOpen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"other"}]`))
	}))
	defer srv.Close()
	p := NewAtlasProvider(Options{BaseURL: srv.URL})
	_, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "agent-7"})
	if !errors.Is(err, ErrNoMapping) {
		t.Fatalf("err = %v, want ErrNoMapping", err)
	}
	// 无定位键的 ref 同样映射不到
	if _, err := p.GetServerStatus(context.Background(), ServerRef{}); !errors.Is(err, ErrNoMapping) {
		t.Fatalf("empty ref err = %v", err)
	}
}

func TestAtlas_UnavailableFailsOpen(t *testing.T) {
	// baseURL 未配置
	p := NewAtlasProvider(Options{})
	if _, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "a"}); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("unconfigured err = %v", err)
	}
	// HTTP 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	p2 := NewAtlasProvider(Options{BaseURL: srv.URL})
	if _, err := p2.GetServerStatus(context.Background(), ServerRef{AgentID: "a"}); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("http500 err = %v", err)
	}
	// 超时
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer slow.Close()
	p3 := NewAtlasProvider(Options{BaseURL: slow.URL, Timeout: 10 * time.Millisecond})
	if _, err := p3.GetServerStatus(context.Background(), ServerRef{AgentID: "a"}); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("timeout err = %v", err)
	}
	// 坏 JSON
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer bad.Close()
	p4 := NewAtlasProvider(Options{BaseURL: bad.URL})
	if _, err := p4.GetServerStatus(context.Background(), ServerRef{AgentID: "a"}); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("bad json err = %v", err)
	}
}

// ListServers 吃 atlas 真实包装 {"servers":[...]}（status 枚举判定同款）。
func TestAtlas_ListServers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/discovery/servers" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"servers":[{"id":"a1","host":"h1","status":"online"},{"id":"a2","status":"maintenance"}]}`))
	}))
	defer srv.Close()
	p := NewAtlasProvider(Options{BaseURL: srv.URL})
	rows, err := p.ListServers(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 || rows[0].AgentID != "a1" || rows[0].Host != "h1" || rows[0].InMaintenance || !rows[1].InMaintenance {
		t.Fatalf("rows = %+v", rows)
	}
}

// MatchKey=host + query 口径：定位参数名随 MatchKey 走。
func TestAtlas_MatchKeyVariants(t *testing.T) {
	var gotKey, gotVal string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVal = r.URL.Query().Get("host")
		gotKey = r.URL.Query().Get("agentId")
		_, _ = w.Write([]byte(`[{"host":"h9","inMaintenance":true}]`))
	}))
	defer srv.Close()
	p := NewAtlasProvider(Options{
		BaseURL: srv.URL,
		Atlas: AtlasOptions{
			StatusPath: "/api/servers",
			MatchKey:   "host",
			Fields:     map[string]string{"inMaintenance": "inMaintenance"},
		},
	})
	st, err := p.GetServerStatus(context.Background(), ServerRef{Host: "h9"})
	if err != nil || !st.InMaintenance {
		t.Fatalf("st=%+v err=%v", st, err)
	}
	if gotVal != "h9" || gotKey != "" {
		t.Fatalf("match params = host:%q agentId:%q", gotVal, gotKey)
	}
}

// {id} 占位符替换对 host 定位同样生效（值经 QueryEscape）。
func TestAtlas_PathPlaceholderWithHostMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/discovery/servers/h 9" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"host":"h 9","status":"maintenance"}`))
	}))
	defer srv.Close()
	p := NewAtlasProvider(Options{
		BaseURL: srv.URL,
		Atlas:   AtlasOptions{MatchKey: "host"},
	})
	st, err := p.GetServerStatus(context.Background(), ServerRef{Host: "h 9"})
	if err != nil || !st.InMaintenance {
		t.Fatalf("st=%+v err=%v", st, err)
	}
}

func TestGate_WithAtlasStub(t *testing.T) {
	// S1 验收路径全链：atlas httptest stub → gate 三分支（真实口径：
	// 路径参数定位 + status 枚举）。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/v1/discovery/servers/")
		switch id {
		case "maintained":
			_, _ = w.Write([]byte(`{"id":"maintained","status":"maintenance"}`))
		case "free":
			_, _ = w.Write([]byte(`{"id":"free","status":"online"}`))
		default:
			_, _ = w.Write([]byte(`{"servers":[]}`))
		}
	}))
	defer srv.Close()
	g := NewGate(NewAtlasProvider(Options{BaseURL: srv.URL}), NewCache(0), nil)
	ctx := context.Background()

	if v := g.Evaluate(ctx, ServerRef{AgentID: "maintained"}); !v.Suppressed {
		t.Fatalf("maintained verdict = %+v", v)
	}
	if v := g.Evaluate(ctx, ServerRef{AgentID: "free"}); v.Suppressed || v.SourceUnknown {
		t.Fatalf("free verdict = %+v", v)
	}
	if v := g.Evaluate(ctx, ServerRef{AgentID: "ghost"}); v.Suppressed || !v.SourceUnknown {
		t.Fatalf("ghost verdict = %+v", v)
	}
	// 无定位不过 gate
	if v := g.Evaluate(ctx, ServerRef{}); v != (Verdict{}) {
		t.Fatalf("no-ref verdict = %+v", v)
	}
}
