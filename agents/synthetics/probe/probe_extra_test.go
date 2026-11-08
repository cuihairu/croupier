package probe

// probe.go 补测：Run 的配置错误、网络错误分类、状态码不符、body 断言
// 不符/命中、HEAD 跳过 body 断言、body 读取失败（hijack 半截响应）、
// classify 三分类直测（timeout/dns/network）。
//
// 诚实跳过：无（Run 全分支可经 httptest/hijack 直达）。

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/config"
	"github.com/stretchr/testify/require"
)

func TestRunConfigurationError(t *testing.T) {
	// 非法 method（含控制字符）→ NewRequest 直接失败
	r := Run(context.Background(), config.Target{
		Name: "bad", URL: "http://127.0.0.1:1", Method: "GE\nT", Timeout: time.Second,
	})
	require.False(t, r.OK)
	require.Contains(t, r.Error, "configuration: ")
}

func TestRunNetworkErrorClassified(t *testing.T) {
	// 关闭端口 → 连接拒绝 → network 分类
	s := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := s.URL
	s.Close()
	r := Run(context.Background(), config.Target{Name: "down", URL: url, Method: "GET", Timeout: time.Second})
	require.False(t, r.OK)
	require.Contains(t, r.Error, "network: ")
}

func TestRunStatusMismatch(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer s.Close()
	r := Run(context.Background(), config.Target{
		Name: "s500", URL: s.URL, Method: "GET", Timeout: time.Second, ExpectedStatus: 200,
	})
	require.False(t, r.OK)
	require.Contains(t, r.Error, "http_status: got 503 want 200")
}

func TestRunBodyContainsMismatch(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("maintenance page"))
	}))
	defer s.Close()
	r := Run(context.Background(), config.Target{
		Name: "body", URL: s.URL, Method: "GET", Timeout: time.Second,
		ExpectedStatus: 200, BodyContains: "healthy",
	})
	require.False(t, r.OK)
	require.Contains(t, r.Error, "content_assertion: expected configured text not found")
}

func TestRunHeadSkipsBodyAssertion(t *testing.T) {
	// HEAD + BodyContains → 跳过 body 断言，状态命中即成功
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()
	r := Run(context.Background(), config.Target{
		Name: "head", URL: s.URL, Method: "HEAD", Timeout: time.Second,
		ExpectedStatus: 200, BodyContains: "unreachable-text",
	})
	require.True(t, r.OK)
	require.Equal(t, 200, r.Status)
}

func TestRunBodyReadError(t *testing.T) {
	// 半截响应：Content-Length 声称 100，实际写 5 字节后断连 → 读取失败
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := http.NewResponseController(w).Hijack()
		require.NoError(t, err)
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\nshort"))
		_ = conn.Close()
	}))
	defer s.Close()
	r := Run(context.Background(), config.Target{
		Name: "trunc", URL: s.URL, Method: "GET", Timeout: 3 * time.Second,
		ExpectedStatus: 200, BodyContains: "healthy",
	})
	require.False(t, r.OK)
	require.True(t, strings.HasPrefix(r.Error, "response_read: "), "got %q", r.Error)
}

func TestClassifyErrorFamilies(t *testing.T) {
	// timeout：错误实现 Timeout() bool
	var timeoutErr timeoutBoolError
	require.Contains(t, classify(timeoutErr), "timeout: ")
	require.Contains(t, classify(errors.New("lookup xyz: no such host")), "dns: ")
	require.Contains(t, classify(errors.New("connection refused")), "network: ")
}

// timeoutBoolError 只为实现 Timeout() bool 的分类分支。
type timeoutBoolError struct{}

func (timeoutBoolError) Error() string   { return "context deadline exceeded" }
func (timeoutBoolError) Timeout() bool   { return true }
func (timeoutBoolError) Temporary() bool { return false }
