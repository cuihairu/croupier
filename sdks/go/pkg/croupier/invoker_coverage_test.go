package croupier

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

// ---------------------------------------------------------------------------
// http_invoker helpers
// ---------------------------------------------------------------------------

func TestIsRetryableHTTPError(t *testing.T) {
	if !isRetryableHTTPError(0, nil) {
		t.Fatal("status 0 (transport error) must be retryable")
	}
	if !isRetryableHTTPError(429, nil) {
		t.Fatal("429 must be retryable")
	}
	if !isRetryableHTTPError(503, nil) {
		t.Fatal("5xx must be retryable")
	}
	if isRetryableHTTPError(400, nil) {
		t.Fatal("400 must not be retryable by default")
	}
	custom := &RetryConfig{RetryableStatusCodes: []int32{418}}
	if !isRetryableHTTPError(418, custom) {
		t.Fatal("configured status must be retryable")
	}
	if isRetryableHTTPError(400, custom) {
		t.Fatal("unconfigured client errors stay non-retryable")
	}
}

func TestRetryDelay(t *testing.T) {
	if d := retryDelay(1, nil); d != 0 {
		t.Fatalf("nil retry config delay = %v", d)
	}
	cfg := &RetryConfig{InitialDelayMs: 10, MaxDelayMs: 50, BackoffMultiplier: 0, JitterFactor: 0}
	// Multiplier <= 0 falls back to 2.
	if d := retryDelay(0, cfg); d != 10*time.Millisecond {
		t.Fatalf("delay = %v, want 10ms", d)
	}
	// Capped at MaxDelayMs.
	if d := retryDelay(10, cfg); d != 50*time.Millisecond {
		t.Fatalf("delay = %v, want 50ms cap", d)
	}
	jittered := &RetryConfig{InitialDelayMs: 100, MaxDelayMs: 1000, BackoffMultiplier: 2, JitterFactor: 0.5}
	if d := retryDelay(2, jittered); d < 0 {
		t.Fatalf("negative delay: %v", d)
	}
}

func TestWaitForContext(t *testing.T) {
	if !waitForContext(context.Background(), 0) {
		t.Fatal("zero delay with live context should return true")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if waitForContext(canceled, 0) {
		t.Fatal("zero delay with canceled context should return false")
	}
	if waitForContext(canceled, 50*time.Millisecond) {
		t.Fatal("timer path with canceled context should return false")
	}
	if !waitForContext(context.Background(), 10*time.Millisecond) {
		t.Fatal("timer path with live context should return true")
	}
}

func TestSendTaskEvent(t *testing.T) {
	events := make(chan TaskEvent, 1)
	if !sendTaskEvent(context.Background(), events, TaskEvent{EventType: "progress"}) {
		t.Fatal("event should be delivered")
	}

	// Canceled context with no receiver: the select deterministically picks
	// ctx.Done because the channel is never ready.
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if sendTaskEvent(canceled, make(chan TaskEvent), TaskEvent{}) {
		t.Fatal("canceled context should reject the event")
	}
}

func TestMaxInt64(t *testing.T) {
	if maxInt64(1, 2) != 2 || maxInt64(3, 2) != 3 {
		t.Fatal("maxInt64 broken")
	}
}

func TestServerErrorMessage(t *testing.T) {
	if got := serverErrorMessage([]byte(`{"message":"boom"}`)); got != "boom" {
		t.Fatalf("message = %q", got)
	}
	if got := serverErrorMessage([]byte(`{"error":"bad"}`)); got != "bad" {
		t.Fatalf("error = %q", got)
	}
	if got := serverErrorMessage([]byte("raw text")); got != "raw text" {
		t.Fatalf("raw = %q", got)
	}
	if got := serverErrorMessage([]byte("")); got != "empty response body" {
		t.Fatalf("empty = %q", got)
	}
	// A blank message falls back to the trimmed raw body.
	if got := serverErrorMessage([]byte(`{"message":"  "}`)); got != `{"message":"  "}` {
		t.Fatalf("blank message fallback = %q", got)
	}
}

func TestParseJSONPayload(t *testing.T) {
	v, err := parseJSONPayload("")
	if err != nil || v == nil {
		t.Fatalf("empty payload = %v, %v", v, err)
	}
	v, err = parseJSONPayload(`{"a":1}`)
	if err != nil {
		t.Fatalf("valid payload: %v", err)
	}
	if _, ok := v.(map[string]interface{}); !ok {
		t.Fatalf("payload type = %T", v)
	}
	if _, err = parseJSONPayload("not json"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidateTaskID(t *testing.T) {
	if err := validateTaskID(""); err == nil {
		t.Fatal("empty task ID must fail")
	}
	if err := validateTaskID(" t1 "); err != nil {
		t.Fatalf("valid task ID: %v", err)
	}
}

func genSDKSelfSignedCert(t *testing.T, dir string) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(7),
		Subject:               pkix.Name{CommonName: "sdk-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certPath, keyPath
}

func TestBuildHTTPInvokerTLSConfig(t *testing.T) {
	// No TLS files: minimal config.
	cfg, err := buildHTTPInvokerTLSConfig(&InvokerConfig{Insecure: false})
	if err != nil {
		t.Fatalf("default TLS config: %v", err)
	}
	if cfg.RootCAs != nil {
		t.Fatal("RootCAs should be nil without CAFile")
	}

	// Missing CA file.
	if _, err := buildHTTPInvokerTLSConfig(&InvokerConfig{CAFile: filepath.Join(t.TempDir(), "missing.pem")}); err == nil {
		t.Fatal("expected read CA file error")
	}

	// Invalid CA PEM.
	badCA := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(badCA, []byte("junk"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := buildHTTPInvokerTLSConfig(&InvokerConfig{CAFile: badCA}); err == nil {
		t.Fatal("expected CA parse error")
	}

	dir := t.TempDir()
	certPath, keyPath := genSDKSelfSignedCert(t, dir)

	// Valid CA + client certificate.
	cfg, err = buildHTTPInvokerTLSConfig(&InvokerConfig{CAFile: certPath, CertFile: certPath, KeyFile: keyPath})
	if err != nil {
		t.Fatalf("TLS config: %v", err)
	}
	if cfg.RootCAs == nil {
		t.Fatal("RootCAs should be set")
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("Certificates = %d", len(cfg.Certificates))
	}

	// Client cert with missing key.
	if _, err := buildHTTPInvokerTLSConfig(&InvokerConfig{CertFile: certPath, KeyFile: "missing.pem"}); err == nil {
		t.Fatal("expected client certificate load error")
	}
}

func TestNewHTTPClient_TLSFallback(t *testing.T) {
	// Secure config with an unreadable CA: the constructor keeps a usable
	// client rather than downgrading silently.
	cfg := &InvokerConfig{
		Insecure:       false,
		CAFile:         filepath.Join(t.TempDir(), "missing.pem"),
		DefaultTimeout: time.Second,
	}
	client := newHTTPClient(cfg)
	if client == nil {
		t.Fatal("newHTTPClient returned nil")
	}
}

func TestHTTPInvokerTaskPollInterval(t *testing.T) {
	inv := &httpInvoker{config: &InvokerConfig{}}
	if d := inv.taskPollInterval(); d != defaultTaskPollInterval {
		t.Fatalf("default interval = %v", d)
	}
	inv.config.TaskPollInterval = 3 * time.Second
	if d := inv.taskPollInterval(); d != 3*time.Second {
		t.Fatalf("interval = %v", d)
	}
}

func TestHTTPInvokerParseJSONPayload(t *testing.T) {
	inv := &httpInvoker{}
	if got := inv.parseJSONPayload(`{"x":1}`); got == nil {
		t.Fatal("valid JSON should parse")
	}
	if got, ok := inv.parseJSONPayload("bad json").(string); !ok || got != "bad json" {
		t.Fatalf("invalid JSON should fall back to the raw string, got %#v", got)
	}
}

func TestHTTPInvokerValidatePayload(t *testing.T) {
	inv := &httpInvoker{}
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		},
	}

	if err := inv.validatePayload("fn", `{"name":"x"}`, schema); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
	if err := inv.validatePayload("fn", "not json", schema); err == nil {
		t.Fatal("invalid JSON payload must fail")
	}
	if err := inv.validatePayload("fn", `{"name":1}`, schema); err == nil {
		t.Fatal("schema violation must fail")
	}
	if err := inv.validatePayload("fn", "anything", map[string]interface{}{}); err != nil {
		t.Fatalf("empty schema should skip validation: %v", err)
	}
	badSchema := map[string]interface{}{"type": map[string]interface{}{"invalid": true}}
	if err := inv.validatePayload("fn", `{}`, badSchema); err == nil {
		t.Fatal("broken schema must fail")
	}
}

// ---------------------------------------------------------------------------
// tcpInvoker (legacy transport invoker)
// ---------------------------------------------------------------------------

func newTCPInvokerForTest(t *testing.T, addr string) *tcpInvoker {
	// Reconnection is disabled by default in tests: leaving it on can make
	// connect() return "reconnection in progress", which Invoke/StartTask
	// mishandle (nil client dereference — recorded as a product bug).
	inv := newTCPInvoker(&InvokerConfig{
		Address:        addr,
		Insecure:       true,
		TimeoutSeconds: 5,
		Reconnect:      &ReconnectConfig{Enabled: false},
		Retry:          &RetryConfig{Enabled: false},
	})
	t.Cleanup(func() { _ = inv.Close() })
	return inv.(*tcpInvoker)
}

func TestTCPInvoker_ConnectAndInvoke(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID uint32, reqID uint32, body []byte) (uint32, []byte, bool) {
		switch msgID {
		case protocol.MsgInvokeRequest:
			req := &sdkv1.InvokeRequest{}
			_ = proto.Unmarshal(body, req)
			resp, _ := proto.Marshal(&sdkv1.InvokeResponse{Payload: append([]byte("echo:"+req.FunctionId+":"), req.Payload...)})
			return protocol.MsgInvokeResponse, resp, true
		case protocol.MsgStartTaskRequest:
			resp, _ := proto.Marshal(&sdkv1.StartTaskResponse{TaskId: "task-42"})
			return protocol.MsgStartTaskResponse, resp, true
		case protocol.MsgCancelTaskRequest:
			resp, _ := proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`{"cancelled":true}`)})
			return protocol.MsgInvokeResponse, resp, true
		case protocol.MsgStreamTaskRequest:
			// The legacy TCP invoker unmarshals the response body as a
			// TaskEvent directly (no InvokeResponse wrapper).
			event, _ := proto.Marshal(&sdkv1.TaskEvent{TaskId: "task-42", Type: "done"})
			return protocol.MsgInvokeResponse, event, true
		default:
			resp, _ := proto.Marshal(&sdkv1.InvokeResponse{})
			return protocol.MsgInvokeResponse, resp, true
		}
	})

	inv := newTCPInvokerForTest(t, agent.addr())
	if err := inv.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	// Second connect is a no-op.
	if err := inv.Connect(context.Background()); err != nil {
		t.Fatalf("idempotent connect: %v", err)
	}

	out, err := inv.Invoke(context.Background(), "player.ban", `{"id":1}`, InvokeOptions{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if out == "" {
		t.Fatal("invoke result should not be empty")
	}

	taskID, err := inv.StartTask(context.Background(), "player.ban", `{"id":1}`, InvokeOptions{})
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if taskID == "" {
		t.Fatal("task ID should not be empty")
	}

	if err := inv.CancelTask(context.Background(), taskID); err != nil {
		t.Fatalf("CancelTask: %v", err)
	}

	events, err := inv.StreamTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("StreamTask: %v", err)
	}
	select {
	case ev, ok := <-events:
		if !ok {
			t.Fatal("event channel closed before terminal event")
		}
		if !ev.Done {
			t.Fatalf("expected done event, got %+v", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no task events received")
	}

	if err := inv.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestTCPInvoker_NotConnectedErrors(t *testing.T) {
	inv := newTCPInvokerForTest(t, "127.0.0.1:1")

	if _, err := inv.Invoke(context.Background(), "f", "{}", InvokeOptions{}); err == nil {
		t.Fatal("expected not-connected error for Invoke")
	}
	if _, err := inv.StartTask(context.Background(), "f", "{}", InvokeOptions{}); err == nil {
		t.Fatal("expected not-connected error for StartTask")
	}
	if err := inv.CancelTask(context.Background(), "t1"); err == nil {
		t.Fatal("expected not-connected error for CancelTask")
	}
	if _, err := inv.StreamTask(context.Background(), "t1"); err == nil {
		t.Fatal("expected not-connected error for StreamTask")
	}
}

func TestTCPInvoker_GetTaskStatusUnsupported(t *testing.T) {
	inv := newTCPInvokerForTest(t, "127.0.0.1:1")
	if _, err := inv.GetTaskStatus(context.Background(), "t1"); err == nil {
		t.Fatal("GetTaskStatus must report unsupported on the TCP invoker")
	}
}

func TestTCPInvoker_SetSchemaAndValidation(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler(""))
	inv := newTCPInvokerForTest(t, agent.addr())

	schema := map[string]interface{}{"type": "object", "required": []interface{}{"id"}}
	if err := inv.SetSchema("f.valid", schema); err != nil {
		t.Fatalf("SetSchema: %v", err)
	}
	if err := inv.validatePayload(`{"id":1}`, schema); err != nil {
		t.Fatalf("valid payload: %v", err)
	}
	if err := inv.validatePayload(`{}`, schema); err == nil {
		t.Fatal("missing required field must fail")
	}
	if err := inv.validatePayload("", map[string]interface{}{}); err == nil {
		t.Fatal("empty payload with empty schema must fail")
	}
	if err := inv.validatePayload("x", map[string]interface{}{}); err != nil {
		t.Fatalf("non-empty payload with empty schema: %v", err)
	}
}

func TestTCPInvoker_IsConnectionError(t *testing.T) {
	inv := &tcpInvoker{}
	cases := map[string]bool{
		"dial tcp: connection refused":   true,
		"read: connection reset by peer": true,
		"write: broken pipe":             true,
		"lookup: no such host":           true,
		"i/o timeout":                    true,
		"grpc: transport is closing":     true,
		"some unrelated failure":         false,
		"":                               false,
	}
	for msg, want := range cases {
		got := inv.isConnectionError(errString(msg))
		if got != want {
			t.Fatalf("isConnectionError(%q) = %v, want %v", msg, got, want)
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestTCPInvoker_IsRetryableError(t *testing.T) {
	inv := &tcpInvoker{}
	if !inv.isRetryableError(errString("rpc error: unavailable")) {
		t.Fatal("unavailable should be retryable")
	}
	if !inv.isRetryableError(errString("context deadline exceeded")) {
		t.Fatal("deadline exceeded should be retryable")
	}
	if inv.isRetryableError(errString("not authorized")) {
		t.Fatal("authorization errors should not be retryable")
	}
	if inv.isRetryableError(nil) {
		t.Fatal("nil error should not be retryable")
	}
}

func TestTCPInvoker_ExecuteWithRetry(t *testing.T) {
	inv := &tcpInvoker{config: &InvokerConfig{Retry: &RetryConfig{Enabled: false}}}

	// Retry disabled: single attempt.
	calls := 0
	out, err := inv.executeWithRetry(context.Background(), InvokeOptions{}, func() (string, error) {
		calls++
		return "ok", nil
	})
	if err != nil || out != "ok" || calls != 1 {
		t.Fatalf("direct execution: %q, %v, calls=%d", out, err, calls)
	}

	// Retry until success.
	retryCfg := &RetryConfig{Enabled: true, MaxAttempts: 3, InitialDelayMs: 1, MaxDelayMs: 5, BackoffMultiplier: 2, JitterFactor: 0}
	calls = 0
	out, err = inv.executeWithRetry(context.Background(), InvokeOptions{Retry: retryCfg}, func() (string, error) {
		calls++
		if calls < 3 {
			return "", errString("service unavailable")
		}
		return "recovered", nil
	})
	if err != nil || out != "recovered" || calls != 3 {
		t.Fatalf("retry recovery: %q, %v, calls=%d", out, err, calls)
	}

	// Non-retryable error stops immediately.
	calls = 0
	_, err = inv.executeWithRetry(context.Background(), InvokeOptions{Retry: retryCfg}, func() (string, error) {
		calls++
		return "", errString("not authorized")
	})
	if err == nil || calls != 1 {
		t.Fatalf("non-retryable: %v, calls=%d", err, calls)
	}

	// Context cancellation aborts the retry loop.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = inv.executeWithRetry(ctx, InvokeOptions{Retry: retryCfg}, func() (string, error) {
		return "", errString("service unavailable")
	})
	if err == nil {
		t.Fatal("expected cancellation error")
	}

	// Exhausted attempts report the last error.
	_, err = inv.executeWithRetry(context.Background(), InvokeOptions{Retry: retryCfg}, func() (string, error) {
		return "", errString("service unavailable")
	})
	if err == nil || !containsStr(err.Error(), "invoke failed after 3 attempts") {
		t.Fatalf("exhausted retries: %v", err)
	}
}

func TestTCPInvoker_CalculateReconnectAndRetryDelay(t *testing.T) {
	inv := &tcpInvoker{config: &InvokerConfig{Reconnect: &ReconnectConfig{
		InitialDelayMs: 100, MaxDelayMs: 1000, BackoffMultiplier: 2, JitterFactor: 0.5,
	}}}
	d := inv.calculateReconnectDelay(1)
	if d < 0 || d > 1000*time.Millisecond {
		t.Fatalf("reconnect delay out of range: %v", d)
	}

	r := inv.calculateRetryDelay(0, &RetryConfig{InitialDelayMs: 10, MaxDelayMs: 20, BackoffMultiplier: 3, JitterFactor: 0.5})
	if r < 0 || r > 20*time.Millisecond {
		t.Fatalf("retry delay out of range: %v", r)
	}
}

func TestTCPInvoker_ScheduleReconnect(t *testing.T) {
	inv := &tcpInvoker{config: &InvokerConfig{Reconnect: &ReconnectConfig{Enabled: false}}}
	inv.scheduleReconnectIfNeeded() // disabled: no-op

	inv.config.Reconnect = &ReconnectConfig{Enabled: true, InitialDelayMs: 20, MaxDelayMs: 50, BackoffMultiplier: 1.5, JitterFactor: 0}
	inv.scheduleReconnectIfNeeded()
	if !inv.isReconnecting {
		t.Fatal("should be reconnecting after schedule")
	}
	// Second schedule while reconnecting is a no-op.
	inv.scheduleReconnectIfNeeded()

	if err := inv.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if inv.isReconnecting {
		t.Fatal("Close must reset reconnecting state")
	}
}

func TestTCPInvoker_ConnectInProgress(t *testing.T) {
	inv := &tcpInvoker{config: &InvokerConfig{
		Address:   "127.0.0.1:1",
		Insecure:  true,
		Reconnect: &ReconnectConfig{Enabled: false},
		Retry:     &RetryConfig{Enabled: false},
	}}
	inv.mu.Lock()
	inv.isReconnecting = true
	inv.mu.Unlock()
	if err := inv.connect(context.Background()); err == nil || !containsStr(err.Error(), "reconnection in progress") {
		t.Fatalf("expected reconnection-in-progress error, got %v", err)
	}
}

func TestTCPInvoker_BuildTLSConfig(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := genSDKSelfSignedCert(t, dir)

	inv := &tcpInvoker{config: &InvokerConfig{Address: "example.com:443", CertFile: certPath, KeyFile: keyPath}}
	cfg, err := inv.buildTLSConfig()
	if err != nil {
		t.Fatalf("buildTLSConfig: %v", err)
	}
	if cfg.ServerName != "example.com" {
		t.Fatalf("ServerName = %q", cfg.ServerName)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("Certificates = %d", len(cfg.Certificates))
	}

	// Bad CA file.
	inv2 := &tcpInvoker{config: &InvokerConfig{Address: "h:1", CAFile: filepath.Join(dir, "missing.pem")}}
	if _, err := inv2.buildTLSConfig(); err == nil {
		t.Fatal("expected CA read error")
	}
	bad := filepath.Join(dir, "bad.pem")
	_ = os.WriteFile(bad, []byte("junk"), 0o600)
	inv3 := &tcpInvoker{config: &InvokerConfig{Address: "h:1", CAFile: bad}}
	if _, err := inv3.buildTLSConfig(); err == nil {
		t.Fatal("expected CA append error")
	}

	// Bad client cert.
	inv4 := &tcpInvoker{config: &InvokerConfig{Address: "h:1", CertFile: certPath, KeyFile: "missing.pem"}}
	if _, err := inv4.buildTLSConfig(); err == nil {
		t.Fatal("expected client cert load error")
	}
}

func TestTCPInvoker_ReconnectDuringInvoke(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler(""))
	inv := newTCPInvokerForTest(t, agent.addr())
	inv.config.Reconnect = &ReconnectConfig{Enabled: true, InitialDelayMs: 10, MaxDelayMs: 20, BackoffMultiplier: 1.1, JitterFactor: 0}
	inv.config.Retry = &RetryConfig{Enabled: true, MaxAttempts: 2, InitialDelayMs: 1, MaxDelayMs: 2, BackoffMultiplier: 1}

	if err := inv.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	// Kill the agent; a subsequent invoke fails on the dead connection and
	// schedules a reconnect attempt.
	agent.stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := inv.Invoke(ctx, "f", "{}", InvokeOptions{})
	if err == nil {
		t.Fatal("expected invoke failure after agent death")
	}
	// Wait for the scheduled reconnect goroutine to settle.
	time.Sleep(100 * time.Millisecond)
	_ = inv.Close()
}

var _ = net.JoinHostPort
