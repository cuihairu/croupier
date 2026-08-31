package lbstats

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnabled(t *testing.T) {
	var nilSvc *LBStatsService
	if nilSvc.Enabled() {
		t.Fatal("nil service 不应 Enabled")
	}
	if NewLBStatsService("").Enabled() {
		t.Fatal("空 URL 不应 Enabled")
	}
	if !NewLBStatsService("http://prom:9090/").Enabled() {
		t.Fatal("配置了 URL 应 Enabled")
	}
}

func TestQueryNotConfigured(t *testing.T) {
	var nilSvc *LBStatsService
	if _, err := nilSvc.Query(context.Background(), "haproxy_up"); err == nil {
		t.Fatal("nil service 应报错")
	}
	if _, err := NewLBStatsService("").Query(context.Background(), "haproxy_up"); err == nil {
		t.Fatal("未配置 URL 应报错")
	}
}

func TestQueryRejectsDisallowed(t *testing.T) {
	s := NewLBStatsService("http://prom:9090")
	if _, err := s.Query(context.Background(), "go_goroutines"); err == nil {
		t.Fatal("非白名单指标应被拒绝")
	}
}

func TestQueryAgainstMockProm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/query":
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"proxy":"be_main"},"value":[1700000000,"42"]}]}}`))
		case "/bad-status":
			w.WriteHeader(http.StatusInternalServerError)
		case "/bad-json":
			_, _ = w.Write([]byte(`{oops`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	s := NewLBStatsService(srv.URL)
	res, err := s.Query(context.Background(), "haproxy_backend_current_sessions")
	if err != nil {
		t.Fatalf("正常查询失败: %v", err)
	}
	if res.Status != "success" || len(res.Data.Result) != 1 {
		t.Fatalf("结果形状错误: %+v", res)
	}

	// 非 200：通过 URL 注入路径——用子路径服务模拟（覆盖 status 分支）
	s2 := NewLBStatsService(srv.URL)
	s2.promURL = srv.URL // 同实例，用错误查询触发白名单拒绝已覆盖；bad-status 用另一个 server
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"status":"error"}`))
	}))
	defer errSrv.Close()
	s3 := NewLBStatsService(errSrv.URL)
	if _, err := s3.Query(context.Background(), "haproxy_up"); err == nil {
		t.Fatal("非 200 状态应报错")
	}

	jsonSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer jsonSrv.Close()
	s4 := NewLBStatsService(jsonSrv.URL)
	if _, err := s4.Query(context.Background(), "haproxy_up"); err == nil {
		t.Fatal("非法 JSON 应报错")
	}

	// 不可达
	s5 := NewLBStatsService("http://127.0.0.1:1")
	if _, err := s5.Query(context.Background(), "haproxy_up"); err == nil {
		t.Fatal("不可达应报错")
	}
}
