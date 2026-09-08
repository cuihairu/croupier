package croupier

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// atomicWriteFile — rename failure branch (target is an existing directory)
// ---------------------------------------------------------------------------

func TestGapAtomicWriteFile_RenameFailsWhenTargetIsDir(t *testing.T) {
	dir := t.TempDir()
	// target exists as a directory → os.Rename(file, dir) fails
	target := filepath.Join(dir, "subdir")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	err := atomicWriteFile(target, []byte("data"))
	if err == nil {
		t.Fatal("expected rename failure when target is an existing directory")
	}
	// temp files must be cleaned up: no .push-* leftovers in dir
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".push-") {
			t.Fatalf("temp file leaked: %s", e.Name())
		}
	}
}

// ---------------------------------------------------------------------------
// tcpInvoker.validatePayload — all reachable branches
// ---------------------------------------------------------------------------

func newGapTCPInvoker() *tcpInvoker {
	return &tcpInvoker{
		config:  &InvokerConfig{Address: "localhost:19090"},
		schemas: make(map[string]map[string]interface{}),
	}
}

func TestGapTCPInvokerValidatePayload_EmptySchemaEmptyPayload(t *testing.T) {
	inv := newGapTCPInvoker()
	err := inv.validatePayload("", nil)
	if err == nil || !strings.Contains(err.Error(), "payload cannot be empty") {
		t.Fatalf("expected empty-payload error, got %v", err)
	}
}

func TestGapTCPInvokerValidatePayload_EmptySchemaNonEmptyPayload(t *testing.T) {
	inv := newGapTCPInvoker()
	if err := inv.validatePayload(`{"a":1}`, map[string]interface{}{}); err != nil {
		t.Fatalf("expected nil for empty schema, got %v", err)
	}
}

func TestGapTCPInvokerValidatePayload_SchemaMarshalError(t *testing.T) {
	inv := newGapTCPInvoker()
	// chan is not JSON-serializable → marshal error
	schema := map[string]interface{}{"x": make(chan int)}
	err := inv.validatePayload(`{}`, schema)
	if err == nil || !strings.Contains(err.Error(), "failed to marshal schema") {
		t.Fatalf("expected marshal error, got %v", err)
	}
}

func TestGapTCPInvokerValidatePayload_SchemaCompileError(t *testing.T) {
	inv := newGapTCPInvoker()
	schema := map[string]interface{}{"type": "bogus-type"}
	err := inv.validatePayload(`{}`, schema)
	if err == nil || !strings.Contains(err.Error(), "schema validation error") {
		t.Fatalf("expected compile error, got %v", err)
	}
}

func TestGapTCPInvokerValidatePayload_PayloadUnmarshalError(t *testing.T) {
	inv := newGapTCPInvoker()
	schema := map[string]interface{}{"type": "object"}
	err := inv.validatePayload("not-json", schema)
	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal payload") {
		t.Fatalf("expected payload unmarshal error, got %v", err)
	}
}

func TestGapTCPInvokerValidatePayload_ValidationFails(t *testing.T) {
	inv := newGapTCPInvoker()
	schema := map[string]interface{}{
		"type":     "object",
		"required": []interface{}{"name"},
	}
	err := inv.validatePayload(`{}`, schema)
	if err == nil {
		t.Fatal("expected validation error for missing required field")
	}
}

func TestGapTCPInvokerValidatePayload_Valid(t *testing.T) {
	inv := newGapTCPInvoker()
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		},
	}
	if err := inv.validatePayload(`{"name":"abc"}`, schema); err != nil {
		t.Fatalf("expected valid payload to pass, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// tcpInvoker.connect — reconnection-in-progress branches
// ---------------------------------------------------------------------------

func TestGapTCPInvokerConnect_ReconnectingFastPath(t *testing.T) {
	inv := newGapTCPInvoker()
	inv.isReconnecting = true
	err := inv.connect(context.Background())
	if err == nil || !strings.Contains(err.Error(), "reconnection in progress") {
		t.Fatalf("expected reconnection-in-progress error, got %v", err)
	}
}

func TestGapTCPInvokerConnect_AlreadyConnectedFastPath(t *testing.T) {
	inv := newGapTCPInvoker()
	inv.connected = true
	if err := inv.connect(context.Background()); err != nil {
		t.Fatalf("expected nil for connected invoker, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// tcpInvoker.Close — pending reconnection cancel branch
// ---------------------------------------------------------------------------

func TestGapTCPInvokerClose_CancelsPendingReconnect(t *testing.T) {
	inv := newGapTCPInvoker()
	canceled := false
	inv.reconnectCancelCtx = func() { canceled = true }
	inv.isReconnecting = true
	inv.reconnectAttempts = 3
	inv.connected = true

	if err := inv.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !canceled {
		t.Fatal("pending reconnection must be canceled on Close")
	}
	if inv.isReconnecting || inv.reconnectAttempts != 0 || inv.connected {
		t.Fatal("Close must reset connection/reconnection state")
	}
	if len(inv.schemas) != 0 {
		t.Fatal("Close must clear schemas")
	}
}

// ---------------------------------------------------------------------------
// httpInvoker.validatePayload — all reachable branches
// ---------------------------------------------------------------------------

func newGapHTTPInvoker(t *testing.T) *httpInvoker {
	t.Helper()
	inv := NewHTTPInvoker(&InvokerConfig{
		Address: "http://127.0.0.1:1",
		Retry:   &RetryConfig{Enabled: false},
	})
	h, ok := inv.(*httpInvoker)
	if !ok {
		t.Fatalf("expected *httpInvoker, got %T", inv)
	}
	return h
}

func TestGapHTTPInvokerValidatePayload_EmptySchema(t *testing.T) {
	h := newGapHTTPInvoker(t)
	if err := h.validatePayload("fn", "anything", nil); err != nil {
		t.Fatalf("nil schema should skip validation, got %v", err)
	}
}

func TestGapHTTPInvokerValidatePayload_InvalidJSONPayload(t *testing.T) {
	h := newGapHTTPInvoker(t)
	schema := map[string]interface{}{"type": "object"}
	err := h.validatePayload("fn", "not-json", schema)
	if err == nil {
		t.Fatal("expected invalid JSON payload error")
	}
}

func TestGapHTTPInvokerValidatePayload_SchemaCompileError(t *testing.T) {
	h := newGapHTTPInvoker(t)
	schema := map[string]interface{}{"type": "bogus-type"}
	err := h.validatePayload("fn", `{}`, schema)
	if err == nil || !strings.Contains(err.Error(), "compile schema validator") {
		t.Fatalf("expected compile error, got %v", err)
	}
}

func TestGapHTTPInvokerValidatePayload_ValidationFails(t *testing.T) {
	h := newGapHTTPInvoker(t)
	schema := map[string]interface{}{
		"type":     "object",
		"required": []interface{}{"name"},
	}
	err := h.validatePayload("fn", `{}`, schema)
	if err == nil {
		t.Fatal("expected validation error for missing required field")
	}
}

func TestGapHTTPInvokerValidatePayload_Valid(t *testing.T) {
	h := newGapHTTPInvoker(t)
	schema := map[string]interface{}{"type": "object"}
	if err := h.validatePayload("fn", `{"a":1}`, schema); err != nil {
		t.Fatalf("expected valid payload to pass, got %v", err)
	}
}

// 说明：以下分支经分析为不可达防御代码，无法通过常规测试覆盖：
//   - http_invoker Invoke/StartTask 的 request marshal 失败（params 来自
//     parseJSONPayload，产物必然可序列化）
//   - validatePayload 的 AddResource 失败（编译器延迟编码资源）
//   - atomicWriteFile 的 tmp.Write 失败（内存缓冲不失败）
//   - gzipBytes/jsonString 的 writer/marshal 失败（同理）
//   - Close 中 if i.client != nil（此前已置 nil）
//   - NewManager 失败（当前实现恒成功）
//   - getGoroutineID 解析失败（goroutine 栈首行恒为 "goroutine N [...]"）
//   - drainAndRecover 30s 超时告警（等价于让测试阻塞 30s）
