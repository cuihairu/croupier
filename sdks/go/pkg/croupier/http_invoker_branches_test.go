package croupier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// httpInvoker: request validation branches
// ---------------------------------------------------------------------------

func TestHTTPInvoker_Invoke_RejectsEmptyFunctionID(t *testing.T) {
	t.Parallel()

	impl := NewHTTPInvoker(nil).(*httpInvoker)
	if _, err := impl.Invoke(context.Background(), "   ", `{}`, InvokeOptions{}); err == nil || !strings.Contains(err.Error(), "function ID cannot be empty") {
		t.Fatalf("Invoke() error = %v, want empty function ID rejection", err)
	}
	if err := validateFunctionID(""); err == nil {
		t.Fatal("validateFunctionID(\"\") = nil, want error")
	}
}

func TestHTTPInvoker_Invoke_RejectsInvalidJSONPayload(t *testing.T) {
	t.Parallel()

	impl := NewHTTPInvoker(nil).(*httpInvoker)
	if _, err := impl.Invoke(context.Background(), "player.ban", `{invalid`, InvokeOptions{}); err == nil || !strings.Contains(err.Error(), "valid JSON") {
		t.Fatalf("Invoke() error = %v, want payload JSON rejection", err)
	}
}

func TestHTTPInvoker_Invoke_SchemaConfiguredPayloadPasses(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"banned":true}}`))
	}))
	defer server.Close()

	impl := NewHTTPInvoker(&InvokerConfig{Address: server.URL}).(*httpInvoker)
	if err := impl.SetSchema("player.ban", map[string]interface{}{
		"type":     "object",
		"required": []interface{}{"playerId"},
		"properties": map[string]interface{}{
			"playerId": map[string]interface{}{"type": "string"},
		},
	}); err != nil {
		t.Fatalf("SetSchema() error = %v", err)
	}
	result, err := impl.Invoke(context.Background(), "player.ban", `{"playerId":"p-1"}`, InvokeOptions{})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result, "banned") {
		t.Fatalf("result = %q, want banned payload", result)
	}
}

func TestHTTPInvoker_SetSchema_RejectsEmptyFunctionID(t *testing.T) {
	t.Parallel()

	impl := NewHTTPInvoker(nil).(*httpInvoker)
	if err := impl.SetSchema("  ", nil); err == nil || !strings.Contains(err.Error(), "function ID cannot be empty") {
		t.Fatalf("SetSchema() error = %v, want empty function ID rejection", err)
	}
}

// ---------------------------------------------------------------------------
// httpInvoker: response decoding failures
// ---------------------------------------------------------------------------

func TestHTTPInvoker_Invoke_DecodeFailures(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "non-json body", body: "not-json", wantErr: "decode invoke response"},
		{name: "missing result field", body: `{"status":"ok"}`, wantErr: "does not contain result"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			impl := NewHTTPInvoker(&InvokerConfig{Address: server.URL}).(*httpInvoker)
			_, err := impl.Invoke(context.Background(), "player.ban", `{}`, InvokeOptions{})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Invoke() error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestHTTPInvoker_StartTask_Branches(t *testing.T) {
	t.Parallel()

	impl := NewHTTPInvoker(nil).(*httpInvoker)
	if _, err := impl.StartTask(context.Background(), "", `{}`, InvokeOptions{}); err == nil || !strings.Contains(err.Error(), "function ID cannot be empty") {
		t.Fatalf("StartTask() error = %v, want empty function ID rejection", err)
	}
	if _, err := impl.StartTask(context.Background(), "report.generate", `nope`, InvokeOptions{}); err == nil || !strings.Contains(err.Error(), "valid JSON") {
		t.Fatalf("StartTask() error = %v, want payload JSON rejection", err)
	}
	if err := impl.SetSchema("report.generate", map[string]interface{}{
		"type":     "object",
		"required": []interface{}{"range"},
	}); err != nil {
		t.Fatalf("SetSchema() error = %v", err)
	}
	if _, err := impl.StartTask(context.Background(), "report.generate", `{}`, InvokeOptions{}); err == nil || !strings.Contains(err.Error(), "payload validation") {
		t.Fatalf("StartTask() error = %v, want local schema rejection", err)
	}

	runners := []struct {
		name    string
		handler http.HandlerFunc
		wantErr string
	}{
		{
			name: "server error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message":"backend exploding"}`))
			},
			wantErr: "backend exploding",
		},
		{
			name: "non-json body",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("not-json"))
			},
			wantErr: "decode start task response",
		},
	}
	for _, tc := range runners {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()

			impl := NewHTTPInvoker(&InvokerConfig{Address: server.URL, Retry: &RetryConfig{Enabled: false}}).(*httpInvoker)
			_, err := impl.StartTask(context.Background(), "report.generate", `{"range":"daily"}`, InvokeOptions{})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("StartTask() error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// httpInvoker: StreamTask branches
// ---------------------------------------------------------------------------

func TestHTTPInvoker_StreamTask_RejectsEmptyTaskID(t *testing.T) {
	t.Parallel()

	impl := NewHTTPInvoker(nil).(*httpInvoker)
	if _, err := impl.StreamTask(context.Background(), "  "); err == nil || !strings.Contains(err.Error(), "task ID cannot be empty") {
		t.Fatalf("StreamTask() error = %v, want empty task ID rejection", err)
	}
}

func TestHTTPInvoker_StreamTask_ServerErrorEmitsErrorEvent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"events unavailable"}`))
	}))
	defer server.Close()

	impl := NewHTTPInvoker(&InvokerConfig{Address: server.URL, Retry: &RetryConfig{Enabled: false}}).(*httpInvoker)
	events, err := impl.StreamTask(context.Background(), "task-9")
	if err != nil {
		t.Fatalf("StreamTask() error = %v", err)
	}
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("events channel closed without an error event")
		}
		if event.EventType != "error" || !strings.Contains(event.Error, "events unavailable") || !event.Done {
			t.Fatalf("unexpected error event: %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for error event")
	}
}

func TestHTTPInvoker_StreamTask_DecodeFailureEmitsErrorEvent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()

	impl := NewHTTPInvoker(&InvokerConfig{Address: server.URL}).(*httpInvoker)
	events, err := impl.StreamTask(context.Background(), "task-9")
	if err != nil {
		t.Fatalf("StreamTask() error = %v", err)
	}
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("events channel closed without an error event")
		}
		if !strings.Contains(event.Error, "decode task events response") || !event.Done {
			t.Fatalf("unexpected error event: %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for error event")
	}
}

func TestHTTPInvoker_StreamTask_MessageOnlyAndFailedEvents(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[` +
			`{"seq":1,"type":"log","message":"compiling"}` + "," +
			`{"seq":2,"type":"failed","message":"boom"}` +
			`],"done":true}`))
	}))
	defer server.Close()

	impl := NewHTTPInvoker(&InvokerConfig{Address: server.URL}).(*httpInvoker)
	events, err := impl.StreamTask(context.Background(), "task-77")
	if err != nil {
		t.Fatalf("StreamTask() error = %v", err)
	}
	var received []TaskEvent
	for event := range events {
		received = append(received, event)
	}
	if len(received) != 2 {
		t.Fatalf("received %d events, want 2: %#v", len(received), received)
	}
	if received[0].Payload != "compiling" || received[0].Done {
		t.Fatalf("unexpected log event: %#v", received[0])
	}
	if received[1].Error != "boom" || !received[1].Done {
		t.Fatalf("unexpected failed event: %#v", received[1])
	}
}

func TestHTTPInvoker_StreamTask_EmptyPageWaitsThenCompletes(t *testing.T) {
	t.Parallel()

	var polls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&polls, 1) == 1 {
			_, _ = w.Write([]byte(`{"items":[],"done":false}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"seq":1,"type":"completed","payload":{"ok":true}}],"done":true}`))
	}))
	defer server.Close()

	impl := NewHTTPInvoker(&InvokerConfig{
		Address:          server.URL,
		TaskPollInterval: 10 * time.Millisecond,
	}).(*httpInvoker)
	events, err := impl.StreamTask(context.Background(), "task-poll")
	if err != nil {
		t.Fatalf("StreamTask() error = %v", err)
	}
	var last TaskEvent
	count := 0
	for event := range events {
		last, count = event, count+1
	}
	if count != 1 || !last.Done || last.EventType != "completed" {
		t.Fatalf("unexpected events (count=%d): %#v", count, last)
	}
	if atomic.LoadInt32(&polls) < 2 {
		t.Fatalf("polled %d times, want >= 2 (poll interval wait skipped)", polls)
	}
}

func TestHTTPInvoker_StreamTask_ContextCancelledStopsPolling(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[],"done":false}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	impl := NewHTTPInvoker(&InvokerConfig{
		Address:          server.URL,
		TaskPollInterval: 50 * time.Millisecond,
		Retry:            &RetryConfig{Enabled: false},
	}).(*httpInvoker)
	events, err := impl.StreamTask(ctx, "task-cancel")
	if err != nil {
		t.Fatalf("StreamTask() error = %v", err)
	}

	// Cancel while the poller is inside the inter-poll wait window.
	time.Sleep(25 * time.Millisecond)
	cancel()

	for {
		select {
		case _, ok := <-events:
			if !ok {
				return // channel closed without events: poller stopped cleanly
			}
		case <-time.After(2 * time.Second):
			t.Fatal("events channel did not close after context cancellation")
		}
	}
}

func TestHTTPInvoker_CancelTask_RejectsEmptyTaskID(t *testing.T) {
	t.Parallel()

	impl := NewHTTPInvoker(nil).(*httpInvoker)
	if err := impl.CancelTask(context.Background(), " "); err == nil || !strings.Contains(err.Error(), "task ID cannot be empty") {
		t.Fatalf("CancelTask() error = %v, want empty task ID rejection", err)
	}
}

// ---------------------------------------------------------------------------
// httpInvoker: GetTaskStatus decode + fallback branches
// ---------------------------------------------------------------------------

func TestHTTPInvoker_GetTaskStatus_DecodeFailures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()

	impl := NewHTTPInvoker(&InvokerConfig{Address: server.URL}).(*httpInvoker)
	if _, err := impl.GetTaskStatus(context.Background(), "task-1"); err == nil || !strings.Contains(err.Error(), "decode task status response") {
		t.Fatalf("GetTaskStatus() error = %v, want decode failure", err)
	}
}

func TestHTTPInvoker_GetTaskStatus_MissingIDFallsBackToRequest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"running","progress":10}`))
	}))
	defer server.Close()

	impl := NewHTTPInvoker(&InvokerConfig{Address: server.URL}).(*httpInvoker)
	status, err := impl.GetTaskStatus(context.Background(), "task-fallback")
	if err != nil {
		t.Fatalf("GetTaskStatus() error = %v", err)
	}
	if status.TaskID != "task-fallback" || status.Status != "running" || status.Progress != 10 {
		t.Fatalf("unexpected status: %#v", status)
	}
}

// ---------------------------------------------------------------------------
// httpInvoker: transport-level failures in doJSON
// ---------------------------------------------------------------------------

func TestHTTPInvoker_DoJSON_RequestCreationFailure(t *testing.T) {
	t.Parallel()

	impl := NewHTTPInvoker(&InvokerConfig{Address: "http://127.0.0.1:1"}).(*httpInvoker)
	// A host that cannot round-trip through url.Parse makes
	// http.NewRequestWithContext fail before any socket is touched.
	impl.baseURL = &url.URL{Scheme: "http", Host: "in valid host", Path: "/api/v1"}

	_, err := impl.Invoke(context.Background(), "player.ban", `{}`, InvokeOptions{})
	if err == nil || !strings.Contains(err.Error(), "create HTTP request") {
		t.Fatalf("Invoke() error = %v, want request creation failure", err)
	}
}

func TestHTTPInvoker_DoJSON_SendFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	address := server.URL
	server.Close() // every subsequent dial is refused

	impl := NewHTTPInvoker(&InvokerConfig{Address: address, Retry: &RetryConfig{Enabled: false}}).(*httpInvoker)
	_, err := impl.Invoke(context.Background(), "player.ban", `{}`, InvokeOptions{})
	if err == nil || !strings.Contains(err.Error(), "send HTTP request") {
		t.Fatalf("Invoke() error = %v, want send failure", err)
	}
}

func TestHTTPInvoker_DoJSON_TruncatedBodyReadFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "512")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"partial":`)) // fewer bytes than declared
	}))
	defer server.Close()

	impl := NewHTTPInvoker(&InvokerConfig{Address: server.URL, Retry: &RetryConfig{Enabled: false}}).(*httpInvoker)
	_, err := impl.Invoke(context.Background(), "player.ban", `{}`, InvokeOptions{})
	if err == nil || !strings.Contains(err.Error(), "read HTTP response") {
		t.Fatalf("Invoke() error = %v, want response read failure", err)
	}
}

// ---------------------------------------------------------------------------
// httpInvoker: withRetry branches
// ---------------------------------------------------------------------------

func TestHTTPInvoker_WithOptionsRetryOverride(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	defer server.Close()

	impl := NewHTTPInvoker(&InvokerConfig{Address: server.URL}).(*httpInvoker)
	result, err := impl.Invoke(context.Background(), "health.check", `{}`, InvokeOptions{
		Retry: &RetryConfig{Enabled: true, MaxAttempts: 2, InitialDelayMs: 1},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result != `"ok"` {
		t.Fatalf("result = %q", result)
	}
}

func TestHTTPInvoker_RetryAbortedByContextCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"still broken"}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(80 * time.Millisecond)
		cancel()
	}()

	impl := NewHTTPInvoker(&InvokerConfig{Address: server.URL}).(*httpInvoker)
	_, err := impl.Invoke(ctx, "health.check", `{}`, InvokeOptions{
		Retry: &RetryConfig{Enabled: true, MaxAttempts: 5, InitialDelayMs: 400},
	})
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("Invoke() error = %v, want context cancellation during backoff", err)
	}
}

func TestHTTPInvoker_PerRequestTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(400 * time.Millisecond)
		_, _ = w.Write([]byte(`{"result":"late"}`))
	}))
	defer server.Close()

	impl := NewHTTPInvoker(&InvokerConfig{Address: server.URL}).(*httpInvoker)
	_, err := impl.Invoke(context.Background(), "health.check", `{}`, InvokeOptions{
		Timeout: 50 * time.Millisecond,
		Retry:   &RetryConfig{Enabled: false},
	})
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("Invoke() error = %v, want per-request timeout", err)
	}
}

// ---------------------------------------------------------------------------
// Server API URL normalization
// ---------------------------------------------------------------------------

func TestParseServerAPIURL_Variants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		address string
		want    string
	}{
		{address: "127.0.0.1:18780", want: "http://127.0.0.1:18780/api/v1"},
		{address: " 127.0.0.1:18780/ ", want: "http://127.0.0.1:18780/api/v1"},
		{address: "http://%zz", want: defaultServerAPIURL},        // unparseable → default
		{address: "http:///only/path", want: defaultServerAPIURL}, // empty host → default
		{address: "http://web.internal/gm", want: "http://web.internal/gm/api/v1"},
		{address: "https://host/api/v1", want: "https://host/api/v1"},
		{address: "http://host/api/v1/?verbose=1#frag", want: "http://host/api/v1"},
	}
	for _, tc := range cases {
		if got := parseServerAPIURL(tc.address).String(); got != tc.want {
			t.Fatalf("parseServerAPIURL(%q) = %q, want %q", tc.address, got, tc.want)
		}
	}
}

func TestRetryDelay_ClampsNegativeDelayToZero(t *testing.T) {
	t.Parallel()

	if got := retryDelay(0, &RetryConfig{InitialDelayMs: -100}); got != 0 {
		t.Fatalf("retryDelay() = %v, want 0 for negative delay", got)
	}
}
