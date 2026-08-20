package croupier

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

// disableRetry isolates single-attempt behavior for deterministic error paths.
func disableRetry() *RetryConfig {
	return &RetryConfig{Enabled: false}
}

// tcpTestInvoker builds an insecure tcpInvoker aimed at the fake agent.
func tcpTestInvoker(address string) *tcpInvoker {
	return newTCPInvoker(&InvokerConfig{
		Address:        address,
		TimeoutSeconds: 5,
		Insecure:       true,
		Reconnect:      &ReconnectConfig{Enabled: false},
		Retry:          disableRetry(),
	}).(*tcpInvoker)
}

// ---------------------------------------------------------------------------
// connectLocked: TLS branches
// ---------------------------------------------------------------------------

func TestTCPInvoker_Connect_TLSConfigFailure(t *testing.T) {
	t.Parallel()

	inv := newTCPInvoker(&InvokerConfig{
		Address:   "127.0.0.1:1",
		Insecure:  false,
		CAFile:    "/nonexistent/ca.pem",
		Reconnect: &ReconnectConfig{Enabled: false},
		Retry:     disableRetry(),
	}).(*tcpInvoker)

	err := inv.Connect(context.Background())
	if err == nil || !strings.Contains(err.Error(), "failed to read CA file") {
		t.Fatalf("Connect() error = %v, want CA file read failure", err)
	}
}

func TestTCPInvoker_Connect_TLSModeUsesSystemPool(t *testing.T) {
	t.Parallel()

	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-tls"))
	inv := tcpTestInvoker(agent.addr())
	inv.config.Insecure = false // TLS config built from system roots; transport stays raw TCP

	if err := inv.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer inv.Close()
	inv.mu.RLock()
	connected := inv.connected
	inv.mu.RUnlock()
	if !connected {
		t.Fatal("invoker should be connected")
	}
}

// ---------------------------------------------------------------------------
// Invoke / StartTask / StreamTask / CancelTask error branches
// ---------------------------------------------------------------------------

func TestTCPInvoker_Invoke_ReconnectingLoopExhausted(t *testing.T) {
	t.Parallel()

	inv := tcpTestInvoker("127.0.0.1:1")
	inv.mu.Lock()
	inv.isReconnecting = true
	inv.mu.Unlock()

	started := time.Now()
	_, err := inv.Invoke(context.Background(), "demo.echo", `{}`, InvokeOptions{})
	if err == nil || !strings.Contains(err.Error(), "not connected to server: reconnection in progress") {
		t.Fatalf("Invoke() error = %v, want reconnection in progress exhaustion", err)
	}
	if elapsed := time.Since(started); elapsed < 20*time.Millisecond {
		t.Fatalf("Invoke() returned after %v, want three 10ms retries", elapsed)
	}
}

func TestTCPInvoker_StreamTask_WaitsForReconnectToClear(t *testing.T) {
	t.Parallel()

	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
		if msgID == protocol.MsgStreamTaskRequest {
			event, _ := proto.Marshal(&sdkv1.TaskEvent{Type: "done", Message: "finished"})
			return protocol.MsgInvokeResponse, event, true
		}
		return defaultAgentHandler("sess-streamwait")(msgID, reqID, body)
	})
	inv := tcpTestInvoker(agent.addr())

	inv.mu.Lock()
	inv.isReconnecting = true
	inv.mu.Unlock()
	go func() {
		time.Sleep(3 * time.Millisecond)
		inv.mu.Lock()
		inv.isReconnecting = false
		inv.mu.Unlock()
	}()

	events, err := inv.StreamTask(context.Background(), "task-wait")
	if err != nil {
		t.Fatalf("StreamTask() error = %v", err)
	}
	var last TaskEvent
	count := 0
	for event := range events {
		last, count = event, count+1
	}
	if count != 1 || !last.Done || last.Payload != "finished" {
		t.Fatalf("unexpected events (count=%d): %#v", count, last)
	}
}

func TestTCPInvoker_StartTask_SucceedsAfterReconnectingClears(t *testing.T) {
	t.Parallel()

	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
		if msgID == protocol.MsgStartTaskRequest {
			return protocol.MsgStartTaskResponse, []byte("task-123"), true
		}
		return defaultAgentHandler("sess-start")(msgID, reqID, body)
	})
	inv := tcpTestInvoker(agent.addr())

	inv.mu.Lock()
	inv.isReconnecting = true
	inv.mu.Unlock()
	go func() {
		time.Sleep(3 * time.Millisecond)
		inv.mu.Lock()
		inv.isReconnecting = false
		inv.mu.Unlock()
	}()

	taskID, err := inv.StartTask(context.Background(), "demo.echo", `{}`, InvokeOptions{})
	if err != nil {
		t.Fatalf("StartTask() error = %v", err)
	}
	if taskID != "task-123" {
		t.Fatalf("taskID = %q, want task-123", taskID)
	}
}

func TestTCPInvoker_Operations_ConnectionRefused(t *testing.T) {
	t.Parallel()

	inv := tcpTestInvoker("127.0.0.1:1") // nothing listens on port 1

	if _, err := inv.Invoke(context.Background(), "demo.echo", `{}`, InvokeOptions{}); err == nil ||
		!strings.Contains(err.Error(), "not connected to server") || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("Invoke() error = %v, want connection refused wrapper", err)
	}
	if _, err := inv.StartTask(context.Background(), "demo.echo", `{}`, InvokeOptions{}); err == nil ||
		!strings.Contains(err.Error(), "not connected to server") {
		t.Fatalf("StartTask() error = %v, want connection refused wrapper", err)
	}
	events, err := inv.StreamTask(context.Background(), "task-1")
	if err == nil || !strings.Contains(err.Error(), "not connected to server") {
		t.Fatalf("StreamTask() error = %v, want connection refused wrapper", err)
	}
	if _, ok := <-events; ok {
		t.Fatal("error StreamTask() channel must be closed")
	}
	if err := inv.CancelTask(context.Background(), "task-1"); err == nil ||
		!strings.Contains(err.Error(), "not connected to server") {
		t.Fatalf("CancelTask() error = %v, want connection refused wrapper", err)
	}
}

func TestTCPInvoker_Invoke_LocalSchemaValidationFailure(t *testing.T) {
	t.Parallel()

	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-schema"))
	inv := tcpTestInvoker(agent.addr())
	if err := inv.SetSchema("demo.echo", map[string]interface{}{
		"type":     "object",
		"required": []string{"name"},
	}); err != nil {
		t.Fatalf("SetSchema() error = %v", err)
	}

	_, err := inv.Invoke(context.Background(), "demo.echo", `{}`, InvokeOptions{})
	if err == nil || !strings.Contains(err.Error(), "payload validation failed for function demo.echo") {
		t.Fatalf("Invoke() error = %v, want local schema rejection", err)
	}
}

func TestTCPInvoker_Invoke_HeadersAndTimeoutMetadata(t *testing.T) {
	t.Parallel()

	var gotRequest *sdkv1.InvokeRequest
	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
		if msgID == protocol.MsgInvokeRequest {
			req := &sdkv1.InvokeRequest{}
			if err := proto.Unmarshal(body, req); err == nil {
				gotRequest = req
			}
			resp, _ := proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`"ok"`)})
			return protocol.MsgInvokeResponse, resp, true
		}
		return defaultAgentHandler("sess-meta")(msgID, reqID, body)
	})
	inv := tcpTestInvoker(agent.addr())

	result, err := inv.Invoke(context.Background(), "demo.echo", `{"a":1}`, InvokeOptions{
		IdempotencyKey: "idem-7",
		Timeout:        2 * time.Second,
		Headers:        map[string]string{"X-Game-ID": "game-a", "X-Env": "dev"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result != `"ok"` {
		t.Fatalf("result = %q", result)
	}
	if gotRequest == nil {
		t.Fatal("fake agent received no invoke request")
	}
	if gotRequest.IdempotencyKey != "idem-7" {
		t.Fatalf("idempotency key = %q", gotRequest.IdempotencyKey)
	}
	if gotRequest.Metadata["X-Game-ID"] != "game-a" || gotRequest.Metadata["X-Env"] != "dev" {
		t.Fatalf("metadata headers not propagated: %v", gotRequest.Metadata)
	}
}

func TestTCPInvoker_Invoke_UnmarshalResponseFailure(t *testing.T) {
	t.Parallel()

	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
		if msgID == protocol.MsgInvokeRequest {
			return protocol.MsgInvokeResponse, []byte{0xff, 0xff, 0xff}, true // invalid protobuf
		}
		return defaultAgentHandler("sess-badresp")(msgID, reqID, body)
	})
	inv := tcpTestInvoker(agent.addr())

	_, err := inv.Invoke(context.Background(), "demo.echo", `{}`, InvokeOptions{})
	if err == nil || !strings.Contains(err.Error(), "unmarshal response") {
		t.Fatalf("Invoke() error = %v, want unmarshal failure", err)
	}
}

func TestTCPInvoker_StartTask_CallFailure(t *testing.T) {
	t.Parallel()

	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, _ uint32, _ []byte) (uint32, []byte, bool) {
		if msgID == protocol.MsgStartTaskRequest {
			return 0, nil, false // hang up
		}
		return defaultAgentHandler("sess-startfail")(msgID, 0, nil)
	})
	inv := tcpTestInvoker(agent.addr())

	_, err := inv.StartTask(context.Background(), "demo.echo", `{}`, InvokeOptions{})
	if err == nil {
		t.Fatal("StartTask() = nil error, want call failure after hangup")
	}
}

func TestTCPInvoker_CancelTask_CallFailure(t *testing.T) {
	t.Parallel()

	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, _ uint32, _ []byte) (uint32, []byte, bool) {
		if msgID == protocol.MsgCancelTaskRequest {
			return 0, nil, false // hang up
		}
		return defaultAgentHandler("sess-cancelfail")(msgID, 0, nil)
	})
	inv := tcpTestInvoker(agent.addr())

	if err := inv.CancelTask(context.Background(), "task-1"); err == nil ||
		!strings.Contains(err.Error(), "cancel task failed") {
		t.Fatalf("CancelTask() error = %v, want call failure", err)
	}
}

func TestTCPInvoker_ExecuteWithRetry_ZeroAttempts(t *testing.T) {
	t.Parallel()

	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-zero"))
	inv := tcpTestInvoker(agent.addr())

	_, err := inv.Invoke(context.Background(), "demo.echo", `{}`, InvokeOptions{
		Retry: &RetryConfig{Enabled: true, MaxAttempts: 0},
	})
	if err == nil || !strings.Contains(err.Error(), "invoke failed after 0 attempts") {
		t.Fatalf("Invoke() error = %v, want zero-attempt exhaustion", err)
	}
}

// ---------------------------------------------------------------------------
// StreamTask polling goroutine branches
// ---------------------------------------------------------------------------

func TestTCPInvoker_StreamTask_ProgressThenDone(t *testing.T) {
	t.Parallel()

	var polls int32
	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
		if msgID == protocol.MsgStreamTaskRequest {
			var event *sdkv1.TaskEvent
			if atomic.AddInt32(&polls, 1) == 1 {
				event = &sdkv1.TaskEvent{Type: "progress", Progress: 40}
			} else {
				event = &sdkv1.TaskEvent{Type: "done", Message: "finished"}
			}
			resp, _ := proto.Marshal(event)
			return protocol.MsgInvokeResponse, resp, true
		}
		return defaultAgentHandler("sess-stream")(msgID, reqID, body)
	})
	inv := tcpTestInvoker(agent.addr())

	events, err := inv.StreamTask(context.Background(), "task-5")
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
	if received[0].Payload != "Progress: 40%" || received[0].Done {
		t.Fatalf("unexpected progress event: %#v", received[0])
	}
	if received[1].Payload != "finished" || !received[1].Done {
		t.Fatalf("unexpected done event: %#v", received[1])
	}
}

func TestTCPInvoker_StreamTask_ContextCancelled(t *testing.T) {
	t.Parallel()

	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
		if msgID == protocol.MsgStreamTaskRequest {
			event, _ := proto.Marshal(&sdkv1.TaskEvent{Type: "progress", Progress: 1})
			return protocol.MsgInvokeResponse, event, true
		}
		return defaultAgentHandler("sess-ctx2")(msgID, reqID, body)
	})
	inv := tcpTestInvoker(agent.addr())

	ctx, cancel := context.WithCancel(context.Background())
	events, err := inv.StreamTask(ctx, "task-ctx")
	if err != nil {
		t.Fatalf("StreamTask() error = %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	cancel()

	for {
		select {
		case _, ok := <-events:
			if !ok {
				return
			}
		case <-time.After(2 * time.Second):
			t.Fatal("events channel did not close after context cancellation")
		}
	}
}

func TestTCPInvoker_StreamTask_ConnectionLost(t *testing.T) {
	t.Parallel()

	inv := tcpTestInvoker("127.0.0.1:1")
	// Mark connected without ever creating a client: the polling goroutine
	// must report "connection lost" instead of crashing.
	inv.mu.Lock()
	inv.connected = true
	inv.mu.Unlock()

	events, err := inv.StreamTask(context.Background(), "task-lost")
	if err != nil {
		t.Fatalf("StreamTask() error = %v", err)
	}
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("events channel closed without connection lost event")
		}
		if event.Error != "connection lost" || !event.Done {
			t.Fatalf("unexpected event: %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for connection lost event")
	}
}

func TestTCPInvoker_StreamTask_CallFailureEmitsErrorEvent(t *testing.T) {
	t.Parallel()

	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, _ uint32, _ []byte) (uint32, []byte, bool) {
		if msgID == protocol.MsgStreamTaskRequest {
			return 0, nil, false // hang up
		}
		return defaultAgentHandler("sess-streamfail")(msgID, 0, nil)
	})
	inv := tcpTestInvoker(agent.addr())

	events, err := inv.StreamTask(context.Background(), "task-callfail")
	if err != nil {
		t.Fatalf("StreamTask() error = %v", err)
	}
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("events channel closed without an error event")
		}
		if !strings.Contains(event.Error, "poll task status failed") || !event.Done {
			t.Fatalf("unexpected event: %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for poll failure event")
	}
}

func TestTCPInvoker_StreamTask_UnmarshalFailureEmitsErrorEvent(t *testing.T) {
	t.Parallel()

	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
		if msgID == protocol.MsgStreamTaskRequest {
			return protocol.MsgInvokeResponse, []byte{0xfe, 0xfe}, true // invalid protobuf
		}
		return defaultAgentHandler("sess-streambad")(msgID, reqID, body)
	})
	inv := tcpTestInvoker(agent.addr())

	events, err := inv.StreamTask(context.Background(), "task-badproto")
	if err != nil {
		t.Fatalf("StreamTask() error = %v", err)
	}
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("events channel closed without an error event")
		}
		if !strings.Contains(event.Error, "unmarshal response") || !event.Done {
			t.Fatalf("unexpected event: %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for unmarshal failure event")
	}
}

// ---------------------------------------------------------------------------
// Reconnection scheduling
// ---------------------------------------------------------------------------

func TestTCPInvoker_ReconnectAttemptFailsAndReschedules(t *testing.T) {
	t.Parallel()

	inv := newTCPInvoker(&InvokerConfig{
		Address:        "127.0.0.1:1", // connection refused for every attempt
		TimeoutSeconds: 2,
		Insecure:       true,
		Reconnect: &ReconnectConfig{
			Enabled:           true,
			MaxAttempts:       2,
			InitialDelayMs:    1,
			MaxDelayMs:        2,
			JitterFactor:      0,
			BackoffMultiplier: 1,
		},
		Retry: disableRetry(),
	}).(*tcpInvoker)

	if _, err := inv.Invoke(context.Background(), "demo.echo", `{}`, InvokeOptions{}); err == nil {
		t.Fatal("Invoke() = nil error, want connection refused")
	}

	// The background attempt fails (fast-path "reconnection in progress") and
	// the scheduler must terminate cleanly without further attempts.
	deadline := time.Now().Add(2 * time.Second)
	for {
		inv.mu.RLock()
		attempts, reconnecting := inv.reconnectAttempts, inv.isReconnecting
		inv.mu.RUnlock()
		if attempts >= 1 && reconnecting {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("reconnect attempts = %d, reconnecting = %v, want >= 1 attempt scheduled", attempts, reconnecting)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestTCPInvoker_CalculateReconnectDelay_ClampedToMax(t *testing.T) {
	t.Parallel()

	inv := tcpTestInvoker("127.0.0.1:1")
	inv.config.Reconnect = &ReconnectConfig{
		InitialDelayMs:    100,
		MaxDelayMs:        100,
		JitterFactor:      1.0,
		BackoffMultiplier: 1,
	}
	clamped := false
	for i := 0; i < 100 && !clamped; i++ {
		delay := inv.calculateReconnectDelay(1)
		if delay < 0 || delay > 100*time.Millisecond {
			t.Fatalf("delay = %v, want within [0, 100ms]", delay)
		}
		if delay == 100*time.Millisecond {
			clamped = true
		}
	}
	if !clamped {
		t.Fatal("jittered delay never hit the max-delay clamp")
	}
}

func TestTCPInvoker_CalculateRetryDelay_ClampedToMax(t *testing.T) {
	t.Parallel()

	inv := tcpTestInvoker("127.0.0.1:1")
	cfg := &RetryConfig{
		InitialDelayMs:    100,
		MaxDelayMs:        100,
		JitterFactor:      1.0,
		BackoffMultiplier: 1,
	}
	clamped := false
	for i := 0; i < 100 && !clamped; i++ {
		delay := inv.calculateRetryDelay(0, cfg)
		if delay < 0 || delay > 100*time.Millisecond {
			t.Fatalf("delay = %v, want within [0, 100ms]", delay)
		}
		if delay == 100*time.Millisecond {
			clamped = true
		}
	}
	if !clamped {
		t.Fatal("jittered delay never hit the max-delay clamp")
	}
}

// ---------------------------------------------------------------------------
// validatePayload failure branches
// ---------------------------------------------------------------------------

func TestTCPInvoker_ValidatePayload_Branches(t *testing.T) {
	t.Parallel()

	inv := tcpTestInvoker("127.0.0.1:1")

	if err := inv.validatePayload("", map[string]interface{}{}); err == nil || !strings.Contains(err.Error(), "payload cannot be empty") {
		t.Fatalf("validatePayload(empty) error = %v, want empty payload rejection", err)
	}
	if err := inv.validatePayload("{}", nil); err != nil {
		t.Fatalf("validatePayload(no schema) error = %v", err)
	}
	if err := inv.validatePayload("{}", map[string]interface{}{"x-bad": make(chan int)}); err == nil ||
		!strings.Contains(err.Error(), "failed to marshal schema") {
		t.Fatalf("validatePayload(chan schema) error = %v, want schema marshal failure", err)
	}
	if err := inv.validatePayload("{}", map[string]interface{}{"type": 123}); err == nil ||
		!strings.Contains(err.Error(), "schema validation error") {
		t.Fatalf("validatePayload(bad keyword) error = %v, want compile failure", err)
	}
	if err := inv.validatePayload("not-json", map[string]interface{}{"type": "object"}); err == nil ||
		!strings.Contains(err.Error(), "failed to unmarshal payload") {
		t.Fatalf("validatePayload(bad payload) error = %v, want payload unmarshal failure", err)
	}
	if err := inv.validatePayload(`{"a":"x"}`, map[string]interface{}{
		"type":     "object",
		"required": []string{"a"},
		"properties": map[string]interface{}{
			"a": map[string]interface{}{"type": "integer"},
		},
	}); err == nil || !strings.Contains(err.Error(), "payload validation failed") {
		t.Fatalf("validatePayload(schema violation) error = %v, want validation failure", err)
	}
	if err := inv.validatePayload(`{"a":1}`, map[string]interface{}{
		"type":     "object",
		"required": []string{"a"},
		"properties": map[string]interface{}{
			"a": map[string]interface{}{"type": "integer"},
		},
	}); err != nil {
		t.Fatalf("validatePayload(valid) error = %v", err)
	}
}
