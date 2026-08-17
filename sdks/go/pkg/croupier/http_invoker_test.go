package croupier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewInvoker_UsesServerHTTP(t *testing.T) {
	t.Parallel()

	invoker := NewInvoker(nil)
	impl, ok := invoker.(*httpInvoker)
	if !ok {
		t.Fatalf("NewInvoker() returned %T, want *httpInvoker", invoker)
	}
	if got, want := impl.GetAddress(), defaultServerAPIURL; got != want {
		t.Fatalf("default Server API URL = %q, want %q", got, want)
	}
	if impl.config.AuthToken != "" {
		t.Fatal("default invoker unexpectedly has an auth token")
	}
}

func TestHTTPInvoker_Invoke_UsesServerContract(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/functions/player.ban/invoke"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		if got, want := r.Method, http.MethodPost; got != want {
			t.Fatalf("method = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer server-token"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("X-Game-ID"), "game-a"; got != want {
			t.Fatalf("X-Game-ID = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("X-Env"), "staging"; got != want {
			t.Fatalf("X-Env = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Idempotency-Key"), "invoke-1"; got != want {
			t.Fatalf("Idempotency-Key = %q, want %q", got, want)
		}

		var body struct {
			Params map[string]interface{} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got, want := body.Params["playerId"], "p-1"; got != want {
			t.Fatalf("params.playerId = %v, want %v", got, want)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{"status": "banned"},
		})
	}))
	defer server.Close()

	invoker := NewInvoker(&InvokerConfig{
		Address:   server.URL,
		AuthToken: "server-token",
	})
	result, err := invoker.Invoke(context.Background(), "player.ban", `{"playerId":"p-1"}`, InvokeOptions{
		IdempotencyKey: "invoke-1",
		Headers: map[string]string{
			"X-Game-ID": "game-a",
			"X-Env":     "staging",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if got, want := result, `{"status":"banned"}`; got != want {
		t.Fatalf("result = %q, want %q", got, want)
	}
}

func TestHTTPInvoker_DefaultScopeAndExplicitAuthorization(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer explicit-token"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("X-Game-ID"), "default-game"; got != want {
			t.Fatalf("X-Game-ID = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("X-Env"), "production"; got != want {
			t.Fatalf("X-Env = %q, want %q", got, want)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok"})
	}))
	defer server.Close()

	impl := NewHTTPInvoker(&InvokerConfig{
		Address: server.URL, AuthToken: "configured-token", GameID: "default-game", Env: "production",
	}).(*httpInvoker)
	_, err := impl.Invoke(context.Background(), "health.check", `{}`, InvokeOptions{Headers: map[string]string{
		"Authorization": "Bearer explicit-token",
	}})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
}

func TestHTTPInvoker_StartTask_UsesServerIssuedID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/tasks"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		var body struct {
			FunctionID string                 `json:"functionId"`
			Params     map[string]interface{} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got, want := body.FunctionID, "report.generate"; got != want {
			t.Fatalf("functionId = %q, want %q", got, want)
		}
		if got, want := body.Params["range"], "daily"; got != want {
			t.Fatalf("params.range = %v, want %v", got, want)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"taskId": "server-task-42", "status": "dispatching"})
	}))
	defer server.Close()

	invoker := NewHTTPInvoker(&InvokerConfig{Address: server.URL})
	taskID, err := invoker.StartTask(context.Background(), "report.generate", `{"range":"daily"}`, InvokeOptions{})
	if err != nil {
		t.Fatalf("StartTask() error = %v", err)
	}
	if got, want := taskID, "server-task-42"; got != want {
		t.Fatalf("taskID = %q, want %q", got, want)
	}
}

func TestHTTPInvoker_StartTask_RejectsMissingServerTaskID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "dispatching"})
	}))
	defer server.Close()

	_, err := NewHTTPInvoker(&InvokerConfig{Address: server.URL}).StartTask(context.Background(), "report.generate", `{}`, InvokeOptions{})
	if err == nil || !strings.Contains(err.Error(), "taskId") {
		t.Fatalf("StartTask() error = %v, want missing taskId error", err)
	}
}

func TestHTTPInvoker_TaskStatusStreamAndCancelUseServerEndpoints(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var eventRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/server-task-42":
			if got, want := r.Header.Get("Authorization"), "Bearer server-token"; got != want {
				t.Fatalf("Authorization = %q, want %q", got, want)
			}
			if got, want := r.Header.Get("X-Game-ID"), "game-a"; got != want {
				t.Fatalf("X-Game-ID = %q, want %q", got, want)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":         "server-task-42",
				"functionId": "report.generate",
				"status":     "running",
				"progress":   50,
				"message":    "halfway",
				"result":     map[string]bool{"partial": true},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/server-task-42/events":
			mu.Lock()
			eventRequests++
			requestNumber := eventRequests
			mu.Unlock()
			if requestNumber == 1 {
				if got, want := r.URL.Query().Get("after_seq"), "0"; got != want {
					t.Fatalf("first after_seq = %q, want %q", got, want)
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"items": []map[string]interface{}{{"seq": 1, "type": "progress", "progress": 50, "message": "halfway", "payload": map[string]int{"count": 1}}},
					"done":  false,
				})
				return
			}
			if got, want := r.URL.Query().Get("after_seq"), "1"; got != want {
				t.Fatalf("second after_seq = %q, want %q", got, want)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []map[string]interface{}{{"seq": 2, "type": "completed", "message": "finished", "payload": map[string]bool{"ok": true}}},
				"done":  true,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tasks/server-task-42/cancel":
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "accepted"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	impl := NewHTTPInvoker(&InvokerConfig{Address: server.URL, AuthToken: "server-token", TaskPollInterval: time.Millisecond}).(*httpInvoker)
	impl.SetDefaultGameEnv("game-a", "staging")
	status, err := impl.GetTaskStatus(context.Background(), "server-task-42")
	if err != nil {
		t.Fatalf("GetTaskStatus() error = %v", err)
	}
	if status.TaskID != "server-task-42" || status.FunctionID != "report.generate" || status.Status != "running" || status.Progress != 50 {
		t.Fatalf("unexpected task status: %#v", status)
	}
	if got, want := status.Result, `{"partial":true}`; got != want {
		t.Fatalf("task result = %q, want %q", got, want)
	}
	events, err := impl.StreamTask(context.Background(), "server-task-42")
	if err != nil {
		t.Fatalf("StreamTask() error = %v", err)
	}
	var received []TaskEvent
	for event := range events {
		received = append(received, event)
	}
	if len(received) != 2 {
		t.Fatalf("received %d task events, want 2: %#v", len(received), received)
	}
	if received[0].Payload != `{"count":1}` || received[0].Done {
		t.Fatalf("unexpected progress event: %#v", received[0])
	}
	if received[1].Payload != `{"ok":true}` || !received[1].Done {
		t.Fatalf("unexpected terminal event: %#v", received[1])
	}
	if err := impl.CancelTask(context.Background(), "server-task-42"); err != nil {
		t.Fatalf("CancelTask() error = %v", err)
	}
}

func TestHTTPInvoker_GetTaskStatusRejectsInvalidTaskIDAndServerError(t *testing.T) {
	t.Parallel()

	impl := NewHTTPInvoker(nil).(*httpInvoker)
	if _, err := impl.GetTaskStatus(context.Background(), " "); err == nil {
		t.Fatal("GetTaskStatus() accepted an empty task ID")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "task not found"})
	}))
	defer server.Close()

	_, err := NewHTTPInvoker(&InvokerConfig{Address: server.URL, Retry: &RetryConfig{Enabled: false}}).GetTaskStatus(context.Background(), "missing")
	if err == nil || !strings.Contains(err.Error(), "task not found") {
		t.Fatalf("GetTaskStatus() error = %v, want Server error", err)
	}
}

func TestHTTPInvoker_ReportsServerErrorsAndRejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/api/function/") {
			t.Fatal("Invoker called removed /api/function endpoint")
		}
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "scope denied"})
	}))
	defer server.Close()

	impl := NewHTTPInvoker(&InvokerConfig{Address: server.URL, Retry: &RetryConfig{Enabled: false}}).(*httpInvoker)
	if _, err := impl.Invoke(context.Background(), "player.ban", `{}`, InvokeOptions{}); err == nil || !strings.Contains(err.Error(), "scope denied") {
		t.Fatalf("Invoke() error = %v, want Server error", err)
	}
	if err := impl.SetSchema("player.ban", map[string]interface{}{
		"type": "object", "required": []string{"playerId"},
	}); err != nil {
		t.Fatalf("SetSchema() error = %v", err)
	}
	if _, err := impl.Invoke(context.Background(), "player.ban", `{}`, InvokeOptions{}); err == nil || !strings.Contains(err.Error(), "payload validation") {
		t.Fatalf("Invoke() error = %v, want local schema validation error", err)
	}
}

func TestHTTPInvoker_ConnectAndClose(t *testing.T) {
	t.Parallel()

	impl := NewHTTPInvoker(nil).(*httpInvoker)
	if impl.IsConnected() {
		t.Fatal("new HTTP invoker is unexpectedly connected")
	}
	if err := impl.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if !impl.IsConnected() {
		t.Fatal("HTTP invoker is not connected after Connect")
	}
	if err := impl.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if impl.IsConnected() {
		t.Fatal("HTTP invoker remains connected after Close")
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := impl.Connect(cancelled); err == nil {
		t.Fatal("Connect() unexpectedly accepted a cancelled context")
	}
}
