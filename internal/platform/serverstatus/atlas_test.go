package serverstatus

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAtlas_MapsArrayResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("agentId") != "agent-7" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok-1" {
			t.Fatalf("auth header = %q", got)
		}
		_, _ = w.Write([]byte(`[{"agentId":"agent-7","host":"h7","healthy":true,"inMaintenance":true,"windowStart":"2026-10-10T00:00:00Z","windowEnd":"2026-10-11T00:00:00Z","windowNote":"发版"}]`))
	}))
	defer srv.Close()

	p := NewAtlasProvider(Options{BaseURL: srv.URL, Token: "tok-1"})
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "agent-7"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !st.InMaintenance || !st.Healthy || st.Source != ProviderAtlas {
		t.Fatalf("status = %+v", st)
	}
	if st.Window == nil || st.Window.Note != "发版" || st.Window.Start.IsZero() || st.Window.End.IsZero() {
		t.Fatalf("window = %+v", st.Window)
	}
}

func TestAtlas_FieldMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"agent-7","alive":"yes","frozen":1}]`))
	}))
	defer srv.Close()

	p := NewAtlasProvider(Options{
		BaseURL: srv.URL,
		Atlas: AtlasOptions{
			Fields: map[string]string{
				"agentId":       "id",
				"inMaintenance": "frozen",
				"healthy":       "alive",
			},
		},
	})
	// 映射后的 inMaintenance 键是 bool，非 bool 值按零值处理
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "agent-7"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if st.Source != ProviderAtlas {
		t.Fatalf("source = %q", st.Source)
	}
}

func TestAtlas_ObjectResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"agentId":"agent-7","inMaintenance":false}`))
	}))
	defer srv.Close()
	p := NewAtlasProvider(Options{BaseURL: srv.URL})
	st, err := p.GetServerStatus(context.Background(), ServerRef{AgentID: "agent-7"})
	if err != nil || st.InMaintenance {
		t.Fatalf("status = %+v err=%v", st, err)
	}
}

func TestAtlas_NoMappingFailsOpen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"agentId":"other"}]`))
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

func TestAtlas_ListServers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"servers":[{"agentId":"a1","host":"h1","healthy":true,"inMaintenance":false},{"agentId":"a2","inMaintenance":true}]}`))
	}))
	defer srv.Close()
	p := NewAtlasProvider(Options{BaseURL: srv.URL})
	rows, err := p.ListServers(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 || rows[0].AgentID != "a1" || rows[0].Host != "h1" || !rows[0].Healthy || !rows[1].InMaintenance {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestAtlas_MatchKeyVariants(t *testing.T) {
	var gotKey, gotVal string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVal = r.URL.Query().Get("host")
		gotKey = r.URL.Query().Get("agentId")
		_, _ = w.Write([]byte(`[{"host":"h9","inMaintenance":true}]`))
	}))
	defer srv.Close()
	p := NewAtlasProvider(Options{BaseURL: srv.URL, Atlas: AtlasOptions{MatchKey: "host"}})
	st, err := p.GetServerStatus(context.Background(), ServerRef{Host: "h9"})
	if err != nil || !st.InMaintenance {
		t.Fatalf("st=%+v err=%v", st, err)
	}
	if gotVal != "h9" || gotKey != "" {
		t.Fatalf("match params = host:%q agentId:%q", gotVal, gotKey)
	}
}

func TestGate_WithAtlasStub(t *testing.T) {
	// S1 验收路径全链：atlas httptest stub → gate 三分支
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("agentId") == "maintained" {
			_, _ = w.Write([]byte(`[{"agentId":"maintained","inMaintenance":true}]`))
			return
		}
		_, _ = w.Write([]byte(`[{"agentId":"free","inMaintenance":false}]`))
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
