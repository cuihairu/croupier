// 覆盖目标：Query 中 http.NewRequestWithContext 构造失败（非法 URL）与
// 响应体读取失败（服务端中断连接）分支。
package lbstats

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQuery_NewRequestError_V8(t *testing.T) {
	// 含非法字符的 URL 让 http.NewRequestWithContext 的 url.Parse 失败。
	s := NewLBStatsService("http://127.0.0.1:9090/\x7f")
	if !s.Enabled() {
		t.Fatal("expected service enabled")
	}
	_, err := s.Query(context.Background(), "haproxy_server_status")
	if err == nil {
		t.Fatal("expected request build error for invalid prometheus url")
	}
}

func TestQuery_BodyReadError_V8(t *testing.T) {
	// hijack 连接：先发送 200 响应头（声明大 Content-Length），再发送不完整
	// body 后立即断开——客户端 io.ReadAll 报 unexpected EOF。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("server does not support hijack")
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 1024\r\n\r\npartial-body"))
		_ = conn.Close()
	}))
	t.Cleanup(srv.Close)

	s := NewLBStatsService(srv.URL)
	_, err := s.Query(context.Background(), "haproxy_server_status")
	if err == nil {
		t.Fatal("expected body read error from interrupted response")
	}
}
