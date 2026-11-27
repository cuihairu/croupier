package croupier

// AUD-068: write requests must not be replayed automatically. A write may
// have taken effect before the response was lost, so the default retry gate
// is single-shot for non-GET requests and the final failure is marked as a
// result-unknown outcome. GET requests keep the previous retry behavior.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func newWriteRetryInvoker(t *testing.T, url string, cfg *RetryConfig) Invoker {
	t.Helper()
	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:   url,
		Insecure:  true,
		Retry:     cfg,
		Reconnect: &ReconnectConfig{Enabled: false},
	})
	if invoker == nil {
		t.Fatal("NewHTTPInvoker returned nil")
	}
	t.Cleanup(func() { invoker.Close() })
	return invoker
}

func fastRetry(enabled bool) *RetryConfig {
	cfg := DefaultRetryConfig()
	cfg.Enabled = enabled
	cfg.InitialDelayMs = 1
	cfg.MaxDelayMs = 2
	return cfg
}

// 首个 POST 在服务端完成副作用后丢弃响应（http.ErrAbortHandler 切断连接）：
// 默认配置不得重发（count==1），且失败必须带 result-unknown 标记。
func TestWriteNoAutoRetryAfterSideEffect(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		panic(http.ErrAbortHandler)
	}))
	defer srv.Close()

	invoker := newWriteRetryInvoker(t, srv.URL, nil) // default retry: enabled, maxAttempts 3
	_, err := invoker.Invoke(context.Background(), "player.ban", `{}`, InvokeOptions{})
	if err == nil {
		t.Fatal("invoke should fail when the response is dropped")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("write replayed after dropped response: %d calls, want 1", got)
	}
	if !IsResultUnknown(err) {
		t.Fatalf("dropped-response failure should be result-unknown, got: %v", err)
	}
}

// GET 仍按原行为退避重试；最终 5xx 失败不标 result-unknown（读无副作用）。
func TestGetStillRetriesAndNotMarkedUnknown(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal_error","message":"boom"}`))
			return
		}
		_, _ = w.Write([]byte(`{"taskId":"t1","status":"running"}`))
	}))
	defer srv.Close()

	invoker := newWriteRetryInvoker(t, srv.URL, fastRetry(true))
	result, err := invoker.GetTaskStatus(context.Background(), "t1")
	if err != nil {
		t.Fatalf("GET should succeed on 3rd attempt: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("GET attempts = %d, want 3", got)
	}
	_ = result

	// 持续 5xx 的读请求：重试耗尽后返回普通错误，不标 result-unknown。
	atomic.StoreInt32(&calls, 0)
	srvFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal_error","message":"boom"}`))
	}))
	defer srvFail.Close()

	invoker2 := newWriteRetryInvoker(t, srvFail.URL, fastRetry(true))
	_, err = invoker2.GetTaskStatus(context.Background(), "t1")
	if err == nil {
		t.Fatal("GET should fail after exhausting retries")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("GET attempts = %d, want 3", got)
	}
	if IsResultUnknown(err) {
		t.Fatalf("read failures must not be marked result-unknown: %v", err)
	}
}

// retryWrites=true 时写请求恢复自动重试（调用方自声明可重放）。
func TestWriteRetryOptInRetries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal_error","message":"boom"}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{"ok":true}}`))
	}))
	defer srv.Close()

	cfg := fastRetry(true)
	cfg.RetryWrites = true
	invoker := newWriteRetryInvoker(t, srv.URL, cfg)
	_, err := invoker.Invoke(context.Background(), "player.ban", `{}`, InvokeOptions{})
	if err != nil {
		t.Fatalf("retryWrites=true should retry the write to success: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("write attempts = %d, want 2", got)
	}
}

// 写请求重试耗尽（retryWrites=true，持续 503）→ result-unknown 标记。
func TestWriteExhaustedRetryIsUnknown(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"unavailable","message":"try later"}`))
	}))
	defer srv.Close()

	cfg := fastRetry(true)
	cfg.RetryWrites = true
	invoker := newWriteRetryInvoker(t, srv.URL, cfg)
	_, err := invoker.Invoke(context.Background(), "player.ban", `{}`, InvokeOptions{})
	if err == nil {
		t.Fatal("write should fail after exhausting retries")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("write attempts = %d, want 3", got)
	}
	if !IsResultUnknown(err) {
		t.Fatalf("5xx write failure should be result-unknown, got: %v", err)
	}
}

// 明确拒绝（4xx）不标 result-unknown，也不重试。
func TestWriteExplicitRejectionNotUnknown(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"permission_denied","message":"no"}`))
	}))
	defer srv.Close()

	cfg := fastRetry(true)
	cfg.RetryWrites = true // even opted in, 4xx is not retryable and not unknown
	invoker := newWriteRetryInvoker(t, srv.URL, cfg)
	_, err := invoker.Invoke(context.Background(), "player.ban", `{}`, InvokeOptions{})
	if err == nil {
		t.Fatal("403 should fail the invoke")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("403 must not be retried: %d calls, want 1", got)
	}
	if IsResultUnknown(err) {
		t.Fatalf("explicit rejection must not be marked result-unknown: %v", err)
	}
}

// 任务取消同样是写：默认单发；无响应时标 result-unknown。
func TestCancelTaskWriteIsSingleShot(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		panic(http.ErrAbortHandler)
	}))
	defer srv.Close()

	invoker := newWriteRetryInvoker(t, srv.URL, nil)
	err := invoker.CancelTask(context.Background(), "t1")
	if err == nil {
		t.Fatal("cancel should fail when the response is dropped")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("cancel replayed after dropped response: %d calls, want 1", got)
	}
	if !IsResultUnknown(err) {
		t.Fatalf("dropped cancel response should be result-unknown, got: %v", err)
	}
}

// result-unknown 包装保留原始错误供 Unwrap / errors.As。
func TestUnknownResultErrorUnwraps(t *testing.T) {
	inner := &httpInvokerError{statusCode: http.StatusGatewayTimeout, message: "gateway timeout"}
	wrapped := wrapResultUnknown(inner)
	if !IsResultUnknown(wrapped) {
		t.Fatalf("504 should be marked result-unknown: %v", wrapped)
	}
	var httpErr *httpInvokerError
	if !errors.As(wrapped, &httpErr) || httpErr.statusCode != http.StatusGatewayTimeout {
		t.Fatalf("original error not preserved through Unwrap: %v", wrapped)
	}
	if IsResultUnknown(wrapResultUnknown(&httpInvokerError{statusCode: http.StatusBadRequest, message: "bad"})) {
		t.Fatalf("400 must not be marked unknown")
	}
	// 纯传输错误（非 httpInvokerError）→ unknown，StatusCode=0。
	transport := wrapResultUnknown(context.DeadlineExceeded)
	if !IsResultUnknown(transport) {
		t.Fatalf("transport failure should be result-unknown: %v", transport)
	}
}
