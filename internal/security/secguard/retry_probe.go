// 出站调用策略与第三方服务探针（OPEN-ISSUES #57）。
//
// DoWithRetry：网络错误与 5xx 按指数退避重试（at-least-once 语义，POST
// 重试可能造成对端重复处理；4xx 客户端错误不重试）。
// Probe/ProbeSMTP：运维探针端点的实现——HTTP GET 连通性探测与 SMTP
// TCP+EHLO 探活（不发邮件、不产生业务副作用）。

package secguard

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"time"
)

// DoWithRetry 执行请求并在网络错误或 5xx 时重试（指数退避 backoff<<attempt）。
// body 非 nil 时逐轮重建请求体；4xx 不重试；ctx 取消（含整体超时预算）即退。
// 返回最后一次响应/错误：重试耗尽时 5xx 响应原样交回调用方（body 未读）。
func DoWithRetry(ctx context.Context, client *http.Client, method, rawURL string, body []byte, headers map[string]string, retries int, backoff time.Duration) (*http.Response, error) {
	if backoff <= 0 {
		backoff = DefaultRetryBackoff
	}
	var resp *http.Response
	var err error
	for attempt := 0; ; attempt++ {
		req, reqErr := http.NewRequestWithContext(ctx, method, rawURL, nil)
		if reqErr != nil {
			return nil, reqErr
		}
		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
			req.ContentLength = int64(len(body))
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err = client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}
		if attempt >= retries {
			return resp, err
		}
		if resp != nil {
			// 可重试失败的响应体排干再关，连接才能回池复用
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff << attempt):
		}
	}
}

// ProbeResult 第三方服务探测结果（探针端点响应体）。
type ProbeResult struct {
	OK        bool   `json:"ok"`
	Status    int    `json:"status"`    // HTTP 状态码（SMTP 探测为 0）
	LatencyMs int64  `json:"latencyMs"` // 端到端耗时
	Error     string `json:"error,omitempty"`
}

// Probe 对 http(s) URL 做连通性探测：守卫校验（CheckURL + 拨号拦截）→
// GET（最多跟 3 跳）→ 状态码 <400 即健康。只读探测，不发通知类 POST。
func Probe(ctx context.Context, s Settings, rawURL string) ProbeResult {
	start := time.Now()
	fail := func(err error) ProbeResult {
		return ProbeResult{LatencyMs: time.Since(start).Milliseconds(), Error: err.Error()}
	}
	if err := CheckURL(ctx, s, rawURL); err != nil {
		return fail(err)
	}
	client := HTTPClient(s, &http.Client{Timeout: s.TimeoutOrDefault(10 * time.Second)})
	client.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return fmt.Errorf("secguard: 重定向超过 3 跳")
		}
		return nil
	}
	resp, err := client.Get(rawURL)
	if err != nil {
		return fail(err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	res := ProbeResult{
		OK:        resp.StatusCode < 400,
		Status:    resp.StatusCode,
		LatencyMs: time.Since(start).Milliseconds(),
	}
	if !res.OK {
		res.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return res
}

// ProbeSMTP 对 SMTP 服务做 TCP+EHLO 探活：拨号 → 220 问候 → EHLO →
// NOOP → QUIT。SSRF 保护开启时拨号同样走 Control 钩子。不认证、不发信。
func ProbeSMTP(ctx context.Context, s Settings, host string, port int) ProbeResult {
	start := time.Now()
	fail := func(err error) ProbeResult {
		return ProbeResult{LatencyMs: time.Since(start).Milliseconds(), Error: err.Error()}
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	// 与 HTTPClient 同语义：SSRF 保护关闭时不挂拨号钩子（回环探活放行）
	dialer := &net.Dialer{Timeout: s.TimeoutOrDefault(10 * time.Second)}
	if s.SSRFProtection {
		dialer.Control = dialControl(s)
	}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fail(err)
	}
	defer func() { _ = conn.Close() }()
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fail(err)
	}
	defer func() { _ = client.Quit() }()
	if err = client.Hello("croupier"); err != nil {
		return fail(err)
	}
	if err = client.Noop(); err != nil {
		return fail(err)
	}
	return ProbeResult{OK: true, LatencyMs: time.Since(start).Milliseconds()}
}
