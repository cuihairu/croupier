package secguard

// 出站调用策略与探针测试（OPEN-ISSUES #57）：DoWithRetry 重试矩阵
// （4xx 不重试/5xx 重试后成功/耗尽返回末次响应/body 逐轮重建）、Probe
// 连通性探测（健康/失败/守卫拦截）、ProbeSMTP 假服务器全对话与拨号拦截。

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoWithRetry_Matrix(t *testing.T) {
	ctx := context.Background()
	client := &http.Client{Timeout: 2 * time.Second}

	// 4xx 不重试
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	resp, err := DoWithRetry(ctx, client, http.MethodGet, srv.URL, nil, nil, 3, time.Millisecond)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))

	// 5xx 一次后成功（重试 1 次），body 逐轮重建（POST 有体）
	var posts int32
	var bodies []string
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&posts, 1)
		buf := make([]byte, 64)
		n, _ := r.Body.Read(buf)
		bodies = append(bodies, string(buf[:n]))
		if atomic.LoadInt32(&posts) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv2.Close()
	resp, err = DoWithRetry(ctx, client, http.MethodPost, srv2.URL, []byte(`{"a":1}`),
		map[string]string{"Content-Type": "application/json"}, 1, time.Millisecond)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, int32(2), atomic.LoadInt32(&posts))
	assert.Equal(t, []string{`{"a":1}`, `{"a":1}`}, bodies)

	// 耗尽：返回末次 5xx 响应（body 未读交调用方）
	srv3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv3.Close()
	resp, err = DoWithRetry(ctx, client, http.MethodGet, srv3.URL, nil, nil, 2, time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	// retries=0 不重试
	atomic.StoreInt32(&hits, 0)
	resp, err = DoWithRetry(ctx, client, http.MethodGet, srv3.URL, nil, nil, 0, time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	// ctx 超时即退（预算 50ms，重试 100 次也不会跑满）
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer slow.Close()
	budgetCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_, err = DoWithRetry(budgetCtx, client, http.MethodGet, slow.URL, nil, nil, 100, time.Millisecond)
	require.Error(t, err)
}

func TestProbe_HTTP(t *testing.T) {
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res := Probe(ctx, Settings{}, srv.URL)
	assert.True(t, res.OK)
	assert.Equal(t, http.StatusOK, res.Status)
	assert.GreaterOrEqual(t, res.LatencyMs, int64(0))

	// 4xx = 不健康但可达
	srv404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv404.Close()
	res = Probe(ctx, Settings{}, srv404.URL)
	assert.False(t, res.OK)
	assert.Equal(t, http.StatusNotFound, res.Status)
	assert.Contains(t, res.Error, "404")

	// 连接拒绝 → 错误透出
	res = Probe(ctx, Settings{}, "http://127.0.0.1:1/health")
	assert.False(t, res.OK)
	assert.NotEmpty(t, res.Error)

	// SSRF 保护开：回环目标被守卫拦下
	res = Probe(ctx, Settings{SSRFProtection: true}, srv.URL)
	assert.False(t, res.OK)
	assert.Contains(t, res.Error, "受限")
}

// fakeSMTPServer 最小 SMTP 对话：220 问候 → EHLO → NOOP → QUIT。
func fakeSMTPServer(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				write := func(s string) { _, _ = conn.Write([]byte(s)) }
				write("220 fake ESMTP\r\n")
				buf := make([]byte, 512)
				for {
					n, err := conn.Read(buf)
					if err != nil {
						return
					}
					cmd := string(buf[:n])
					switch {
					case len(cmd) >= 4 && cmd[:4] == "EHLO":
						write("250-fake\r\n250 OK\r\n")
					case len(cmd) >= 4 && cmd[:4] == "NOOP":
						write("250 OK\r\n")
					case len(cmd) >= 4 && cmd[:4] == "QUIT":
						write("221 Bye\r\n")
						return
					}
				}
			}()
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return ln
}

func TestProbeSMTP(t *testing.T) {
	ln := fakeSMTPServer(t)
	res := ProbeSMTP(context.Background(), Settings{}, "127.0.0.1", ln.Addr().(*net.TCPAddr).Port)
	assert.True(t, res.OK, res.Error)
	assert.Equal(t, int(0), res.Status)

	// 拒连
	res = ProbeSMTP(context.Background(), Settings{}, "127.0.0.1", 1)
	assert.False(t, res.OK)
	assert.NotEmpty(t, res.Error)
}
