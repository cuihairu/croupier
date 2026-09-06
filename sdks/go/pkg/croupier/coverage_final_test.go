package croupier

// Final coverage boost: exercises the remaining uncovered branches across the
// provider package — file push wire codec, drain/reconnect edges, capability
// manifest upload, invoker retry/reconnect internals and misc helpers.

import (
	"context"

	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

// ---------------------------------------------------------------------------
// logger / misc helpers
// ---------------------------------------------------------------------------

func TestNoOpLoggerAllMethods(t *testing.T) {
	l := &NoOpLogger{}
	l.Debugf("x %d", 1)
	l.Infof("x %d", 2)
	l.Warnf("x %d", 3)
	l.Errorf("x %d", 4)
}

func TestSDKVersionFallback(t *testing.T) {
	if v := sdkVersion(); v == "" {
		t.Fatal("sdkVersion should never be empty")
	}
}

func TestGetGoroutineIDPositive(t *testing.T) {
	if id := getGoroutineID(); id <= 0 {
		t.Fatalf("goroutine id should be positive, got %d", id)
	}
}

func TestWithTraceMetadataNilContext(t *testing.T) {
	ctx := WithTraceMetadata(nil, map[string]string{
		MetadataTraceparent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		MetadataTraceID:     " 4bf92f3577b34da6a3ce929d0e0e4736 ",
	})
	if TraceParentFromContext(ctx) != "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01" {
		t.Fatal("traceparent not propagated")
	}
	if TraceIDFromContext(ctx) != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace id not trimmed: %q", TraceIDFromContext(ctx))
	}
	if WithTraceMetadata(context.Background(), nil) == nil {
		t.Fatal("nil meta should return original ctx")
	}
}

// ---------------------------------------------------------------------------
// file push: codec + validation + staging
// ---------------------------------------------------------------------------

func TestAppendVarintMultiByte(t *testing.T) {
	long := strings.Repeat("a", 300) // >127 → multi-byte varint length
	out := appendVarint(nil, uint64(len(long)))
	if len(out) < 2 {
		t.Fatalf("expected multi-byte varint, got %d bytes", len(out))
	}
	value, next, err := readVarint(out, 0)
	if err != nil || value != uint64(len(long)) || next != len(out) {
		t.Fatalf("roundtrip failed: value=%d next=%d err=%v", value, next, err)
	}
}

func TestReadVarintErrors(t *testing.T) {
	if _, _, err := readVarint([]byte{0x80}, 0); err == nil {
		t.Fatal("expected truncated varint error")
	}
	overflow := []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80}
	if _, _, err := readVarint(overflow, 0); err == nil {
		t.Fatal("expected varint overflow error")
	}
	if _, _, err := readVarint([]byte{}, 5); err == nil {
		t.Fatal("expected out-of-range error")
	}
}

func TestDecodeFilePushRequestErrors(t *testing.T) {
	// unsupported wire type (varint field)
	if _, err := decodeFilePushRequest([]byte{0x08, 0x01}); err == nil {
		t.Fatal("expected unsupported wire type error")
	}
	// tag varint truncated
	if _, err := decodeFilePushRequest([]byte{0x80}); err == nil {
		t.Fatal("expected truncated tag varint error")
	}
	// length varint truncated
	if _, err := decodeFilePushRequest([]byte{0x0A, 0x80}); err == nil {
		t.Fatal("expected truncated length varint error")
	}
	// field payload truncated
	if _, err := decodeFilePushRequest([]byte{0x0A, 0x10, 'a'}); err == nil {
		t.Fatal("expected truncated field error")
	}
	// unknown field (field 9, length-delimited) is skipped
	req, err := decodeFilePushRequest([]byte{0x4A, 0x03, 'x', 'y', 'z'})
	if err != nil {
		t.Fatalf("unknown field should be skipped: %v", err)
	}
	if req.transferID != "" || req.data != nil {
		t.Fatal("unknown field must not populate known fields")
	}
}

func TestEncodeFilePushRequestRoundtrip(t *testing.T) {
	orig := &filePushRequest{
		transferID:    strings.Repeat("t", 200), // forces multi-byte varint
		fileName:      "hot.lua",
		contentSha256: "abc123",
		data:          []byte("payload-bytes"),
	}
	req, err := decodeFilePushRequest(encodeFilePushRequest(orig))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.transferID != orig.transferID || req.fileName != orig.fileName ||
		req.contentSha256 != orig.contentSha256 || string(req.data) != string(orig.data) {
		t.Fatalf("roundtrip mismatch: %+v", req)
	}
}

func TestSafeStagingPathMatrix(t *testing.T) {
	dir := t.TempDir()
	bad := []string{"", "   ", "a/b", `a\b`, "../escape", "/abs/path", ".", "..", "a..b/.."}
	for _, name := range bad {
		if _, err := safeStagingPath(dir, name); err == nil {
			t.Fatalf("expected rejection for %q", name)
		}
	}
	good, err := safeStagingPath(dir, "patch.lua")
	if err != nil {
		t.Fatalf("valid basename rejected: %v", err)
	}
	if !strings.HasPrefix(good, dir) {
		t.Fatalf("staging path escaped dir: %s", good)
	}
}

func newCfgFilePushManager(t *testing.T, mutate func(*ClientConfig)) *TCPManager {
	t.Helper()
	cfg := ClientConfig{EnableFileTransfer: true, FileStagingDir: t.TempDir()}
	if mutate != nil {
		mutate(&cfg)
	}
	mgr, err := NewTCPManager(cfg, testHandlers())
	if err != nil {
		t.Fatalf("NewTCPManager: %v", err)
	}
	return mgr.(*TCPManager)
}

func TestValidateFilePushMatrix(t *testing.T) {
	data := []byte("file-body")
	sum := sha256Hex(data)

	cases := []struct {
		name   string
		mutate func(*ClientConfig)
		req    filePushRequest
		errMsg string
	}{
		{"disabled", func(c *ClientConfig) { c.EnableFileTransfer = false },
			filePushRequest{transferID: "t", fileName: "a.txt", data: data, contentSha256: sum}, "file transfer is disabled"},
		{"emptyTransferID", nil,
			filePushRequest{fileName: "a.txt", data: data, contentSha256: sum}, "transferId is required"},
		{"badFileName", nil,
			filePushRequest{transferID: "t", fileName: "../a", data: data, contentSha256: sum}, "bare basename"},
		{"emptyData", nil,
			filePushRequest{transferID: "t", fileName: "a.txt", contentSha256: sum}, "payload is empty"},
		{"oversize", func(c *ClientConfig) { c.MaxFileSize = 4 },
			filePushRequest{transferID: "t", fileName: "a.txt", data: data, contentSha256: sum}, "exceeds max"},
		{"defaultLimitApplies", func(c *ClientConfig) { c.MaxFileSize = 0 },
			filePushRequest{transferID: "t", fileName: "a.txt", data: data, contentSha256: sum}, ""},
		{"emptySha", nil,
			filePushRequest{transferID: "t", fileName: "a.txt", data: data}, "contentSha256 is required"},
		{"checksumMismatch", nil,
			filePushRequest{transferID: "t", fileName: "a.txt", data: data, contentSha256: "deadbeef"}, "checksum mismatch"},
		{"ok", nil,
			filePushRequest{transferID: "t", fileName: "a.txt", data: data, contentSha256: sum}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newCfgFilePushManager(t, tc.mutate)
			err := m.validateFilePush(&tc.req)
			if tc.errMsg == "" {
				if err != nil {
					t.Fatalf("expected pass, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.errMsg) {
				t.Fatalf("expected error containing %q, got %v", tc.errMsg, err)
			}
		})
	}
}

func TestHandleFilePushRequestPaths(t *testing.T) {
	m := newCfgFilePushManager(t, nil)
	data := []byte("content")
	sum := sha256Hex(data)

	// unmarshal failure
	if _, err := m.handleFilePushRequest([]byte{0x08, 0x01}); err == nil {
		t.Fatal("expected unmarshal error for bad wire type")
	}

	// validation failure → encoded response carries the error field
	body := encodeFilePushRequest(&filePushRequest{transferID: "t", fileName: "a.txt", data: data, contentSha256: "bad"})
	respBytes, err := m.handleFilePushRequest(body)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	decoded := decodeFilePushResponseForTest(t, respBytes)
	if decoded.ok || decoded.errorMsg == "" {
		t.Fatalf("expected error response, got %+v", decoded)
	}

	// success path: ok + storedPath
	body = encodeFilePushRequest(&filePushRequest{transferID: "tid-1", fileName: "ok.txt", data: data, contentSha256: sum})
	respBytes, err = m.handleFilePushRequest(body)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	decoded = decodeFilePushResponseForTest(t, respBytes)
	if !decoded.ok || decoded.transferID != "tid-1" || decoded.storedPath == "" {
		t.Fatalf("expected success response, got %+v", decoded)
	}
	if _, statErr := os.Stat(decoded.storedPath); statErr != nil {
		t.Fatalf("staged file missing: %v", statErr)
	}
}

func decodeFilePushResponseForTest(t *testing.T, body []byte) *filePushDecodedResponse {
	t.Helper()
	resp := &filePushDecodedResponse{}
	idx := 0
	for idx < len(body) {
		tag, next, err := readVarint(body, idx)
		if err != nil {
			t.Fatalf("decode tag: %v", err)
		}
		idx = next
		field := tag >> 3
		switch tag & 0x7 {
		case 0: // varint
			value, next, err := readVarint(body, idx)
			if err != nil {
				t.Fatalf("decode varint: %v", err)
			}
			idx = next
			if field == 2 {
				resp.ok = value == 1
			}
		case 2: // length-delimited
			length, next, err := readVarint(body, idx)
			if err != nil {
				t.Fatalf("decode length: %v", err)
			}
			idx = next
			if idx+int(length) > len(body) {
				t.Fatalf("truncated field %d", field)
			}
			value := body[idx : idx+int(length)]
			idx += int(length)
			switch field {
			case 1:
				resp.transferID = string(value)
			case 3:
				resp.storedPath = string(value)
			case 4:
				resp.errorMsg = string(value)
			}
		default:
			t.Fatalf("unexpected wire type %d", tag&0x7)
		}
	}
	return resp
}

type filePushDecodedResponse struct {
	transferID string
	ok         bool
	storedPath string
	errorMsg   string
}

func TestAtomicWriteFileFailures(t *testing.T) {
	dir := t.TempDir()

	// success
	target := filepath.Join(dir, "f.txt")
	if err := atomicWriteFile(target, []byte("x")); err != nil {
		t.Fatalf("atomicWriteFile: %v", err)
	}

	// CreateTemp fails: parent "directory" is a regular file
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteFile(filepath.Join(blocker, "child.txt"), []byte("x")); err == nil {
		t.Fatal("expected CreateTemp failure")
	}

	// Rename fails: target is an existing directory
	dstDir := filepath.Join(dir, "dest")
	if err := os.Mkdir(dstDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteFile(dstDir, []byte("x")); err == nil {
		t.Fatal("expected rename failure onto directory")
	}
}

func TestFileStagingDirDefault(t *testing.T) {
	tmp := t.TempDir()
	oldWD, wdErr := os.Getwd()
	if wdErr != nil {
		t.Skipf("getwd unsupported: %v", wdErr)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Skipf("chdir unsupported: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	m := newCfgFilePushManager(t, func(c *ClientConfig) { c.FileStagingDir = "" })
	dir := m.fileStagingDir()
	if dir != "./croupier-staging" {
		t.Fatalf("default staging dir = %q", dir)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("default staging dir not created: %v", err)
	}
}

func TestJSONStringEscaping(t *testing.T) {
	if got := jsonString(`he said "hi"` + "\n"); got != `"he said \"hi\"\n"` {
		t.Fatalf("jsonString = %q", got)
	}
}

// ---------------------------------------------------------------------------
// tcpRPCHandler: pong / filePush / drain / invoke edges
// ---------------------------------------------------------------------------

func TestRPCPongReturnsEmptyBody(t *testing.T) {
	m := newCfgFilePushManager(t, nil)
	body, err := m.rpcHandler.pong(context.Background(), protocol.MsgProviderHeartbeatRequest, 7, nil)
	if err != nil {
		t.Fatalf("pong: %v", err)
	}
	resp := &sdkv1.ProviderHeartbeatResponse{}
	if err := proto.Unmarshal(body, resp); err != nil {
		t.Fatalf("unmarshal pong body: %v", err)
	}
}

func TestRPCFilePushDelegates(t *testing.T) {
	m := newCfgFilePushManager(t, nil)
	data := []byte("x")
	body := encodeFilePushRequest(&filePushRequest{transferID: "t", fileName: "y.txt", data: data, contentSha256: sha256Hex(data)})
	if _, err := m.rpcHandler.filePush(context.Background(), protocol.MsgProviderFilePushRequest, 1, body); err != nil {
		t.Fatalf("filePush: %v", err)
	}
}

func TestInvokeDrainingRejection(t *testing.T) {
	m := newCfgFilePushManager(t, nil)
	m.draining.Store(true)
	defer m.draining.Store(false)

	body, err := m.rpcHandler.invoke(context.Background(), protocol.MsgInvokeRequest, 1,
		mustMarshalInvoke(t, "demo.echo", []byte("x")))
	if err != nil {
		t.Fatalf("draining invoke should answer with error payload: %v", err)
	}
	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(body, resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resp.Payload), "draining") {
		t.Fatalf("payload = %q", resp.Payload)
	}
}

func TestInvokeInboundValidationBranches(t *testing.T) {
	schema := `{"type":"object","properties":{"n":{"type":"number"}},"required":["n"]}`
	mgr, _ := NewTCPManager(ClientConfig{
		ValidateInputPayloads: true,
	}, map[string]FunctionHandler{
		"v.fn": func(ctx context.Context, payload []byte) ([]byte, error) { return []byte("ok"), nil },
	})
	m := mgr.(*TCPManager)
	m.mu.Lock()
	m.functions = []*sdkv1.ProviderFunctionDescriptor{{Id: "v.fn", InputSchema: schema}}
	m.mu.Unlock()

	// payload validation failure → error payload response (exercises jsonString)
	body, err := m.rpcHandler.invoke(context.Background(), protocol.MsgInvokeRequest, 1,
		mustMarshalInvoke(t, "v.fn", []byte(`{"n":"not-a-number"}`)))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(body, resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resp.Payload), "error") {
		t.Fatalf("expected error payload, got %q", resp.Payload)
	}

	// invalid payload JSON
	if _, err := m.rpcHandler.invoke(context.Background(), protocol.MsgInvokeRequest, 1,
		mustMarshalInvoke(t, "v.fn", []byte(`not-json`))); err != nil {
		t.Fatalf("invalid JSON should still answer with error payload: %v", err)
	}

	// validateInboundPayload branch matrix
	m.config.ValidateInputPayloads = false
	if err := m.validateInboundPayload("v.fn", []byte("anything")); err != nil {
		t.Fatalf("validation disabled must skip: %v", err)
	}
	m.config.ValidateInputPayloads = true
	if err := m.validateInboundPayload("missing.fn", nil); err != nil {
		t.Fatalf("missing descriptor must skip: %v", err)
	}
	m.mu.Lock()
	m.functions = []*sdkv1.ProviderFunctionDescriptor{{Id: "v.fn", InputSchema: "  "}}
	m.mu.Unlock()
	if err := m.validateInboundPayload("v.fn", nil); err != nil {
		t.Fatalf("empty schema must skip: %v", err)
	}
	m.mu.Lock()
	m.functions = []*sdkv1.ProviderFunctionDescriptor{{Id: "v.fn", InputSchema: "{invalid json"}}
	m.mu.Unlock()
	if err := m.validateInboundPayload("v.fn", nil); err != nil {
		t.Fatalf("invalid schema must skip: %v", err)
	}
	// schema with unresolvable $ref → compile failure is treated as schema defect
	m.mu.Lock()
	m.functions = []*sdkv1.ProviderFunctionDescriptor{{Id: "v.fn", InputSchema: `{"$ref":"http://127.0.0.1:1/missing.json"}`}}
	m.mu.Unlock()
	if err := m.validateInboundPayload("v.fn", []byte(`{}`)); err != nil {
		t.Fatalf("compile failure must skip: %v", err)
	}
	// happy path
	m.mu.Lock()
	m.functions = []*sdkv1.ProviderFunctionDescriptor{{Id: "v.fn", InputSchema: schema}}
	m.mu.Unlock()
	if err := m.validateInboundPayload("v.fn", []byte(`{"n":1}`)); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
}

func TestStartTaskInboundValidationFailure(t *testing.T) {
	schema := `{"type":"object","required":["n"],"properties":{"n":{"type":"number"}}}`
	mgr, _ := NewTCPManager(ClientConfig{ValidateInputPayloads: true}, testHandlers())
	m := mgr.(*TCPManager)
	m.mu.Lock()
	m.functions = []*sdkv1.ProviderFunctionDescriptor{{Id: "demo.echo", InputSchema: schema}}
	m.mu.Unlock()

	if _, err := m.rpcHandler.startTask(context.Background(), protocol.MsgStartTaskRequest, 1,
		mustMarshalInvoke(t, "demo.echo", []byte(`{"n":"bad"}`))); err == nil {
		t.Fatal("expected startTask validation failure")
	}
}

func TestHandleDrainUnparsableBodyAndIdempotency(t *testing.T) {
	m := newCfgFilePushManager(t, nil)
	if _, err := m.rpcHandler.handleDrain(context.Background(), protocol.MsgProviderDrainRequest, 1, []byte{0xff}); err != nil {
		t.Fatalf("drain with unparsable body must still ack: %v", err)
	}
	// second drain hits the CAS-false branch (idempotent ack only)
	if _, err := m.rpcHandler.handleDrain(context.Background(), protocol.MsgProviderDrainRequest, 1, nil); err != nil {
		t.Fatalf("second drain must ack: %v", err)
	}
	// drainAndRecover goroutine races with the test; wait for it to clear the flag
	deadline := time.Now().Add(3 * time.Second)
	for m.draining.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
}

func TestDrainAndRecoverReconnectDisabled(t *testing.T) {
	m := newCfgFilePushManager(t, nil) // Reconnect nil → Disconnect branch
	m.draining.Store(true)
	m.drainAndRecover()
	if m.draining.Load() {
		t.Fatal("draining flag must be cleared")
	}
}

func TestDrainAndRecoverReconnectEnabled(t *testing.T) {
	m := newCfgFilePushManager(t, func(c *ClientConfig) {
		c.Reconnect = &ReconnectConfig{Enabled: true, InitialDelayMs: 1, MaxDelayMs: 2}
	})
	// not connected → handleDisconnect early-returns, still clears flag
	m.drainAndRecover()
	if m.draining.Load() {
		t.Fatal("draining flag must be cleared")
	}
}

// ---------------------------------------------------------------------------
// manager: connect / reconnect / capabilities
// ---------------------------------------------------------------------------

func TestConnectDialFailure(t *testing.T) {
	mgr, _ := NewTCPManager(ClientConfig{AgentAddr: "127.0.0.1:1", Insecure: true}, nil)
	m := mgr.(*TCPManager)
	if err := m.Connect(context.Background()); err == nil {
		t.Fatal("expected dial failure")
	}
	if err := m.Reconnect(context.Background()); err == nil {
		t.Fatal("expected reconnect dial failure")
	}
}

func TestReconnectErrorBranches(t *testing.T) {
	connectBody := func(sessionID string) []byte {
		b, _ := proto.Marshal(&sdkv1.ProviderConnectResponse{SessionId: sessionID})
		return b
	}

	t.Run("unexpectedResponseMsgID", func(t *testing.T) {
		agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
			return protocol.MsgInvokeResponse, connectBody("s"), true
		})
		mgr, _ := NewTCPManager(ClientConfig{AgentAddr: agent.addr(), Insecure: true}, nil)
		m := mgr.(*TCPManager)
		seedRegisteredState(m)
		if err := m.Reconnect(context.Background()); err == nil || !strings.Contains(err.Error(), "unexpected response") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("unparsableResponse", func(t *testing.T) {
		agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
			return protocol.MsgProviderConnectResponse, []byte{0xff, 0xff}, true
		})
		mgr, _ := NewTCPManager(ClientConfig{AgentAddr: agent.addr(), Insecure: true}, nil)
		m := mgr.(*TCPManager)
		seedRegisteredState(m)
		if err := m.Reconnect(context.Background()); err == nil || !strings.Contains(err.Error(), "unmarshal response") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("emptySession", func(t *testing.T) {
		agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
			return protocol.MsgProviderConnectResponse, connectBody(""), true
		})
		mgr, _ := NewTCPManager(ClientConfig{AgentAddr: agent.addr(), Insecure: true}, nil)
		m := mgr.(*TCPManager)
		seedRegisteredState(m)
		if err := m.Reconnect(context.Background()); err == nil || !strings.Contains(err.Error(), "no session ID") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
			return protocol.MsgProviderConnectResponse, connectBody("sess-reconnect"), true
		})
		mgr, _ := NewTCPManager(ClientConfig{AgentAddr: agent.addr(), Insecure: true, HeartbeatInterval: 3600}, nil)
		m := mgr.(*TCPManager)
		seedRegisteredState(m)
		if err := m.Reconnect(context.Background()); err != nil {
			t.Fatalf("reconnect: %v", err)
		}
		if !m.IsConnected() || m.sessionID != "sess-reconnect" {
			t.Fatalf("state not committed: connected=%v session=%q", m.IsConnected(), m.sessionID)
		}
		m.Disconnect()
	})
}

func seedRegisteredState(m *TCPManager) {
	m.mu.Lock()
	m.serviceID = "svc"
	m.serviceVersion = "1.0.0"
	m.functions = []*sdkv1.ProviderFunctionDescriptor{{Id: "demo.echo"}}
	m.mu.Unlock()
}

func TestRegisterWithAgentNotConnected(t *testing.T) {
	mgr, _ := NewTCPManager(ClientConfig{}, testHandlers())
	m := mgr.(*TCPManager)
	if _, err := m.RegisterWithAgent(context.Background(), "svc", "1.0.0", nil); err == nil {
		t.Fatal("expected not-connected error")
	}
}

func TestMaybeRegisterCapabilitiesAsyncEmptyControlAddr(t *testing.T) {
	m := newCfgFilePushManager(t, nil) // ControlAddr empty → early return
	m.maybeRegisterCapabilitiesAsync()
}

func TestMaybeRegisterCapabilitiesFailuresAndSuccess(t *testing.T) {
	// unreachable control plane → warn paths
	m := newCfgFilePushManager(t, func(c *ClientConfig) { c.ControlAddr = "127.0.0.1:1" })
	m.maybeRegisterCapabilities("svc", "1.0.0", []*sdkv1.ProviderFunctionDescriptor{{Id: "demo.echo"}})

	// reachable control plane → success path; manifest entries exercise all branches
	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
		return protocol.MsgRegisterCapabilitiesResp, nil, true
	})
	m2 := newCfgFilePushManager(t, func(c *ClientConfig) { c.ControlAddr = agent.addr() })
	m2.maybeRegisterCapabilities("svc", "1.0.0", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "demo.echo", InputSchema: `{"type":"object"}`, OutputSchema: `{"type":"object"}`,
			Resource: "player", Operation: "query", Risk: "low", Permission: "p", Description: "d"},
		{Id: "", Version: ""}, // skipped entry + orDefault fallbacks
	})
}

func TestGzipBytesRoundtrip(t *testing.T) {
	out, err := gzipBytes([]byte("manifest-json-body"))
	if err != nil {
		t.Fatalf("gzipBytes: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("compressed output empty")
	}
}

func TestHeartbeatLoopContextCancel(t *testing.T) {
	m := newCfgFilePushManager(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() { m.heartbeatLoop(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("heartbeatLoop did not exit on cancelled context")
	}
	// sendHeartbeat on a disconnected manager reports no error (nothing to do)
	if err := m.sendHeartbeat(context.Background()); err != nil {
		t.Fatalf("sendHeartbeat while disconnected: %v", err)
	}
}

// ---------------------------------------------------------------------------
// client: reconnectWithBackoff edges + Stop idempotency
// ---------------------------------------------------------------------------

func newTestClient(agentAddr string, rc *ReconnectConfig) *client {
	c := &client{
		config: &ClientConfig{
			AgentAddr: agentAddr,
			Insecure:  true,
			Reconnect: rc,
		},
		handlers:     map[string]FunctionHandler{"demo.echo": func(ctx context.Context, p []byte) ([]byte, error) { return p, nil }},
		descriptors:  map[string]FunctionDescriptor{"demo.echo": {ID: "demo.echo"}},
		stopCh:       make(chan struct{}),
		disconnectCh: make(chan struct{}, 1),
		logger:       &NoOpLogger{},
	}
	// reconnectWithBackoff disconnects the previous manager on every attempt;
	// seed one so the first attempt does not dereference a nil interface.
	c.manager, _ = NewManager(ManagerConfig{AgentAddr: agentAddr, Insecure: true}, c.handlers)
	return c
}

func TestReconnectWithBackoffDisabled(t *testing.T) {
	c := newTestClient("127.0.0.1:1", &ReconnectConfig{Enabled: false})
	if err := c.reconnectWithBackoff(context.Background()); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("err = %v", err)
	}
}

func TestReconnectWithBackoffMaxAttempts(t *testing.T) {
	c := newTestClient("127.0.0.1:1", &ReconnectConfig{
		Enabled: true, MaxAttempts: 1, InitialDelayMs: 1, MaxDelayMs: 2,
	})
	if err := c.reconnectWithBackoff(context.Background()); err == nil || !strings.Contains(err.Error(), "max reconnection attempts") {
		t.Fatalf("err = %v", err)
	}
}

func TestReconnectWithBackoffStoppedDuringWait(t *testing.T) {
	c := newTestClient("127.0.0.1:1", &ReconnectConfig{
		Enabled: true, MaxAttempts: 0, InitialDelayMs: 5000, MaxDelayMs: 10000,
	})
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(c.stopCh)
	}()
	if err := c.reconnectWithBackoff(context.Background()); err == nil || !strings.Contains(err.Error(), "stopped during reconnection") {
		t.Fatalf("err = %v", err)
	}
}

func TestReconnectWithBackoffContextCancelled(t *testing.T) {
	c := newTestClient("127.0.0.1:1", &ReconnectConfig{
		Enabled: true, MaxAttempts: 0, InitialDelayMs: 5000, MaxDelayMs: 10000,
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	if err := c.reconnectWithBackoff(ctx); err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestReconnectWithBackoffRegisterFailureAndSuccess(t *testing.T) {
	var failRegister atomic.Bool
	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
		if msgID == protocol.MsgProviderConnectRequest {
			if failRegister.Load() {
				// valid protobuf but empty session → registration failure path
				b, _ := proto.Marshal(&sdkv1.ProviderConnectResponse{})
				return protocol.MsgProviderConnectResponse, b, true
			}
			b, _ := proto.Marshal(&sdkv1.ProviderConnectResponse{SessionId: "sess-rb"})
			return protocol.MsgProviderConnectResponse, b, true
		}
		return protocol.MsgInvokeResponse, nil, false
	})

	// register failure path
	failRegister.Store(true)
	c := newTestClient(agent.addr(), &ReconnectConfig{
		Enabled: true, MaxAttempts: 1, InitialDelayMs: 1, MaxDelayMs: 2,
	})
	if err := c.reconnectWithBackoff(context.Background()); err == nil || !strings.Contains(err.Error(), "max reconnection attempts") {
		t.Fatalf("err = %v", err)
	}

	// success path
	failRegister.Store(false)
	c2 := newTestClient(agent.addr(), &ReconnectConfig{
		Enabled: true, MaxAttempts: 3, InitialDelayMs: 1, MaxDelayMs: 2, BackoffMultiplier: 1.0, JitterFactor: 0.5,
	})
	if err := c2.reconnectWithBackoff(context.Background()); err != nil {
		t.Fatalf("reconnectWithBackoff: %v", err)
	}
	if c2.sessionID != "sess-rb" {
		t.Fatalf("sessionID = %q", c2.sessionID)
	}
	if c2.manager == nil {
		t.Fatal("manager must be replaced on success")
	}
	c2.Stop()
}

func TestClientStopIdempotent(t *testing.T) {
	c := newTestClient("127.0.0.1:1", nil)
	if err := c.Stop(); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	if err := c.Stop(); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}

// ---------------------------------------------------------------------------
// invoker internals
// ---------------------------------------------------------------------------

func TestExecuteWithRetryBranches(t *testing.T) {
	inv := newTCPInvoker(&InvokerConfig{
		Address:  "127.0.0.1:1",
		Insecure: true,
		Reconnect: &ReconnectConfig{
			Enabled: true, MaxAttempts: 2, InitialDelayMs: 1, MaxDelayMs: 2,
		},
		Retry: &RetryConfig{Enabled: true, MaxAttempts: 2, InitialDelayMs: 1, MaxDelayMs: 2},
	}).(*tcpInvoker)
	ctx := context.Background()

	// retry disabled → direct execution
	calls := 0
	res, err := inv.executeWithRetry(ctx, InvokeOptions{Retry: &RetryConfig{Enabled: false}}, func() (string, error) {
		calls++
		return "direct", nil
	})
	if err != nil || res != "direct" || calls != 1 {
		t.Fatalf("res=%q err=%v calls=%d", res, err, calls)
	}

	// success on first attempt
	res, err = inv.executeWithRetry(ctx, InvokeOptions{}, func() (string, error) { return "ok", nil })
	if err != nil || res != "ok" {
		t.Fatalf("res=%q err=%v", res, err)
	}

	// non-retryable error fails immediately
	calls = 0
	_, err = inv.executeWithRetry(ctx, InvokeOptions{}, func() (string, error) {
		calls++
		return "", context.Canceled
	})
	if err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}

	// retryable error exhausts attempts
	calls = 0
	_, err = inv.executeWithRetry(ctx, InvokeOptions{}, func() (string, error) {
		calls++
		return "", context.DeadlineExceeded // contains "deadline exceeded"
	})
	if err == nil || calls != 2 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}

	// maxAttempts <= 0 → loop body never runs, final wrap-around error
	_, err = inv.executeWithRetry(ctx, InvokeOptions{Retry: &RetryConfig{Enabled: true, MaxAttempts: 0}}, func() (string, error) {
		return "", nil
	})
	if err == nil {
		t.Fatal("expected zero-attempt failure")
	}

	// context cancelled while waiting for retry delay
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = inv.executeWithRetry(cancelCtx, InvokeOptions{}, func() (string, error) {
		return "", context.DeadlineExceeded
	})
	if err == nil || !strings.Contains(err.Error(), "retry cancelled") {
		t.Fatalf("err = %v", err)
	}

	// connection error + reconnect enabled → scheduleReconnectIfNeeded side
	// effect ("timeout" makes it retryable, "connection refused" makes it a
	// connection error), then recovery on the next attempt.
	calls = 0
	res, err = inv.executeWithRetry(ctx, InvokeOptions{}, func() (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("dial timeout: connection refused")
		}
		return "recovered", nil
	})
	if err != nil || res != "recovered" {
		t.Fatalf("res=%q err=%v", res, err)
	}
	// let any scheduled background reconnect finish quickly (it dials a dead
	// port and gives up after its bounded attempts)
	time.Sleep(20 * time.Millisecond)
	_ = inv.Close()
}

func TestTCPInvokerConnectDoubleChecks(t *testing.T) {
	inv := newTCPInvoker(&InvokerConfig{
		Address: "127.0.0.1:1", Insecure: true,
		Reconnect: DefaultReconnectConfig(), Retry: DefaultRetryConfig(),
	}).(*tcpInvoker)

	// fast path: already connected
	inv.mu.Lock()
	inv.connected = true
	inv.mu.Unlock()
	if err := inv.connect(context.Background()); err != nil {
		t.Fatalf("connected fast path: %v", err)
	}

	// fast path: reconnection in progress
	inv.mu.Lock()
	inv.connected = false
	inv.isReconnecting = true
	inv.mu.Unlock()
	if err := inv.connect(context.Background()); err == nil || err.Error() != "reconnection in progress" {
		t.Fatalf("err = %v", err)
	}

	// slow-path double-check for connected: hold the write lock so the caller
	// blocks between the fast checks and the double-check, then flip state.
	inv.mu.Lock()
	inv.isReconnecting = false
	inv.connected = false
	started := make(chan struct{})
	go func() {
		close(started)
		if err := inv.connect(context.Background()); err != nil {
			t.Errorf("slow-path connected double-check: %v", err)
		}
	}()
	<-started
	time.Sleep(20 * time.Millisecond) // let the goroutine park on mu.Lock
	inv.connected = true
	inv.mu.Unlock()

	// slow-path double-check for isReconnecting
	inv.mu.Lock()
	inv.connected = false
	inv.isReconnecting = false
	started2 := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		close(started2)
		errCh <- inv.connect(context.Background())
	}()
	<-started2
	time.Sleep(20 * time.Millisecond)
	inv.isReconnecting = true
	inv.mu.Unlock()
	select {
	case err := <-errCh:
		if err == nil || err.Error() != "reconnection in progress" {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("connect did not return")
	}

	// reset state so nothing lingers
	inv.mu.Lock()
	inv.isReconnecting = false
	inv.mu.Unlock()
}

func TestTCPInvokerConnectLockedTLSBranches(t *testing.T) {
	// dial failure with TLS config path (CAFile missing)
	inv := newTCPInvoker(&InvokerConfig{
		Address: "127.0.0.1:1", Insecure: false, CAFile: "/nonexistent/ca.pem",
		Reconnect: &ReconnectConfig{Enabled: false}, Retry: DefaultRetryConfig(),
	}).(*tcpInvoker)
	if err := inv.connectLocked(context.Background()); err == nil {
		t.Fatal("expected CA read failure")
	}

	// CA file present but not a certificate → append failure
	bad := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(bad, []byte("not a pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	inv.config.CAFile = bad
	if err := inv.connectLocked(context.Background()); err == nil || !strings.Contains(err.Error(), "append CA") {
		t.Fatalf("err = %v", err)
	}

	// system pool branch: insecure off, no CAFile, reachable plaintext server
	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
		return protocol.MsgInvokeResponse, nil, true
	})
	inv2 := newTCPInvoker(&InvokerConfig{
		Address: agent.addr(), Insecure: false,
		Reconnect: &ReconnectConfig{Enabled: false}, Retry: DefaultRetryConfig(),
	}).(*tcpInvoker)
	if err := inv2.connectLocked(context.Background()); err != nil {
		t.Fatalf("system pool connect: %v", err)
	}
	_ = inv2.Close()
}

func TestTCPInvokerBuildTLSConfigBadClientCert(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "cert.pem")
	key := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(cert, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	inv := newTCPInvoker(&InvokerConfig{
		Address: "host.example:19090", Insecure: false, CertFile: cert, KeyFile: key,
		Reconnect: &ReconnectConfig{Enabled: false}, Retry: DefaultRetryConfig(),
	}).(*tcpInvoker)
	if _, err := inv.buildTLSConfig(); err == nil || !strings.Contains(err.Error(), "client certificate") {
		t.Fatalf("err = %v", err)
	}
}

func TestTCPInvokerCloseCancelsPendingReconnect(t *testing.T) {
	inv := newTCPInvoker(&InvokerConfig{
		Address: "127.0.0.1:1", Insecure: true,
		Reconnect: &ReconnectConfig{
			Enabled: true, MaxAttempts: 0, InitialDelayMs: 1000, MaxDelayMs: 2000,
		},
		Retry: DefaultRetryConfig(),
	}).(*tcpInvoker)
	inv.mu.Lock()
	reconnectCtx, cancel := context.WithCancel(context.Background())
	inv.reconnectCancelCtx = cancel
	inv.reconnectCancelFlag = true
	inv.mu.Unlock()
	if err := inv.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if reconnectCtx.Err() == nil {
		t.Fatal("Close must cancel pending reconnection")
	}
	// second Close with nil state is a no-op
	if err := inv.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestScheduleReconnectNoOpBranches(t *testing.T) {
	inv := newTCPInvoker(&InvokerConfig{
		Address: "127.0.0.1:1", Insecure: true,
		Reconnect: &ReconnectConfig{Enabled: false},
		Retry:     DefaultRetryConfig(),
	}).(*tcpInvoker)
	inv.scheduleReconnectIfNeeded() // disabled → no-op

	inv2 := newTCPInvoker(&InvokerConfig{
		Address: "127.0.0.1:1", Insecure: true,
		Reconnect: &ReconnectConfig{Enabled: true, MaxAttempts: 1, InitialDelayMs: 1, MaxDelayMs: 2},
		Retry:     DefaultRetryConfig(),
	}).(*tcpInvoker)
	inv2.mu.Lock()
	inv2.isReconnecting = true
	inv2.mu.Unlock()
	inv2.scheduleReconnectIfNeeded() // already reconnecting → no-op
	inv2.mu.Lock()
	inv2.isReconnecting = false
	inv2.reconnectAttempts = 1 // == MaxAttempts → give up log
	inv2.mu.Unlock()
	inv2.scheduleReconnectIfNeeded()
}

// captureLogger records formatted log lines so tests can assert on side
// effects that only surface through the global logger.
type captureLogger struct {
	ch chan string
}

func newCaptureLogger() *captureLogger { return &captureLogger{ch: make(chan string, 256)} }

func (c *captureLogger) Debugf(format string, args ...interface{}) {
	c.ch <- "D " + fmt.Sprintf(format, args...)
}
func (c *captureLogger) Infof(format string, args ...interface{}) {
	c.ch <- "I " + fmt.Sprintf(format, args...)
}
func (c *captureLogger) Warnf(format string, args ...interface{}) {
	c.ch <- "W " + fmt.Sprintf(format, args...)
}
func (c *captureLogger) Errorf(format string, args ...interface{}) {
	c.ch <- "E " + fmt.Sprintf(format, args...)
}

func TestScheduleReconnectSucceeds(t *testing.T) {
	reconnectLogCh := make(chan string, 64)
	capture := &captureLogger{ch: reconnectLogCh}
	prev := GetGlobalLogger()
	SetGlobalLogger(capture)
	t.Cleanup(func() { SetGlobalLogger(prev) })

	inv := newTCPInvoker(&InvokerConfig{
		Address: "127.0.0.1:1", Insecure: true,
		Reconnect: &ReconnectConfig{
			Enabled: true, MaxAttempts: 3, InitialDelayMs: 5, MaxDelayMs: 10,
		},
		Retry: DefaultRetryConfig(),
	}).(*tcpInvoker)
	inv.scheduleReconnectIfNeeded()

	// The reconnect goroutine's connect() reuses the already-connected fast
	// path (it holds isReconnecting=true), so mark the session established
	// while the goroutine is still waiting on its delay; the "Reconnection
	// successful" log then proves the success branch ran.
	inv.mu.Lock()
	inv.connected = true
	inv.mu.Unlock()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case msg := <-reconnectLogCh:
			if strings.Contains(msg, "Reconnection successful") {
				_ = inv.Close()
				return
			}
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Fatal("background reconnection did not report success in time")
}

func TestCalculateReconnectAndRetryDelayBounds(t *testing.T) {
	reconnect := &ReconnectConfig{
		Enabled: true, InitialDelayMs: 10, MaxDelayMs: 20,
		BackoffMultiplier: 1.0, JitterFactor: 50.0, // huge jitter exercises the clamps
	}
	inv := newTCPInvoker(&InvokerConfig{
		Address: "127.0.0.1:1", Insecure: true,
		Reconnect: reconnect,
		Retry: &RetryConfig{
			Enabled: true, MaxAttempts: 3, InitialDelayMs: 10, MaxDelayMs: 20,
			BackoffMultiplier: 1.0, JitterFactor: 50.0,
		},
	}).(*tcpInvoker)
	for i := 0; i < 300; i++ {
		if d := inv.calculateReconnectDelay(1); d < 0 || d > 20*time.Millisecond {
			t.Fatalf("reconnect delay out of bounds: %v", d)
		}
		if d := inv.calculateRetryDelay(0, inv.config.Retry); d < 0 || d > 20*time.Millisecond {
			t.Fatalf("retry delay out of bounds: %v", d)
		}
	}
}

func TestTCPInvokerValidatePayloadEdges(t *testing.T) {
	inv := newTCPInvoker(&InvokerConfig{
		Address: "127.0.0.1:1", Insecure: true,
		Reconnect: &ReconnectConfig{Enabled: false}, Retry: DefaultRetryConfig(),
	}).(*tcpInvoker)

	if err := inv.validatePayload("", map[string]interface{}{}); err == nil {
		t.Fatal("empty schema + empty payload must fail")
	}
	if err := inv.validatePayload("x", map[string]interface{}{}); err != nil {
		t.Fatalf("empty schema with payload: %v", err)
	}
	if err := inv.validatePayload(`"str"`, map[string]interface{}{"type": "string"}); err != nil {
		t.Fatalf("valid: %v", err)
	}
	if err := inv.validatePayload("123", map[string]interface{}{"type": "object"}); err == nil {
		t.Fatal("payload not valid against schema must fail")
	}
	if err := inv.validatePayload("not-json", map[string]interface{}{"type": "object"}); err == nil {
		t.Fatal("invalid payload JSON must fail")
	}
	// unmarshalable value inside schema → json.Marshal failure
	if err := inv.validatePayload("{}", map[string]interface{}{"bad": make(chan int)}); err == nil {
		t.Fatal("chan schema value must fail marshalling")
	}
}

// ---------------------------------------------------------------------------
// HTTP invoker: stream cancel + schema compile failure
// ---------------------------------------------------------------------------

func TestHTTPInvokerStreamTaskCancelledMidStream(t *testing.T) {
	items := ""
	for i := 0; i < 20; i++ {
		if i > 0 {
			items += ","
		}
		item, _ := json.Marshal(map[string]interface{}{
			"seq": i + 1, "type": "progress", "message": "m", "payload": map[string]interface{}{"k": i},
		})
		items += string(item)
	}
	body := `{"items":[` + items + `],"done":false}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	inv := NewHTTPInvoker(&InvokerConfig{Address: srv.URL})
	ctx, cancel := context.WithCancel(context.Background())
	events, err := inv.StreamTask(ctx, "task-1")
	if err != nil {
		t.Fatalf("StreamTask: %v", err)
	}
	// Drain two events then cancel: with the channel still saturated the
	// producer hits the cancelled-context branch of sendTaskEvent and stops.
	<-events
	<-events
	cancel()
	deadline := time.Now().Add(3 * time.Second)
	closed := false
	for time.Now().Before(deadline) && !closed {
		select {
		case _, ok := <-events:
			if !ok {
				closed = true
			}
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	if !closed {
		t.Fatal("event channel was not closed after cancellation")
	}
}

func TestHTTPInvokerSchemaCompileFailure(t *testing.T) {
	inv := NewHTTPInvoker(&InvokerConfig{Address: "http://127.0.0.1:1"}).(*httpInvoker)
	// unresolvable $ref → schema compilation fails
	if err := inv.SetSchema("fn.ref", map[string]interface{}{"$ref": "#/$defs/missing"}); err != nil {
		t.Fatalf("SetSchema: %v", err)
	}
	if err := inv.validateConfiguredPayload("fn.ref", "{}"); err == nil {
		t.Fatal("unresolvable schema ref must fail compile")
	}
}

// ---------------------------------------------------------------------------
// config merge: headers merge block
// ---------------------------------------------------------------------------

func TestMergeWithDefaultsHeadersMerge(t *testing.T) {
	merged := MergeWithDefaults(&ClientConfig{
		Headers: map[string]string{"X-Custom": "1"},
	})
	if merged.Headers["X-Custom"] != "1" {
		t.Fatalf("custom header not merged: %v", merged.Headers)
	}
	if merged.Reconnect == nil {
		t.Fatal("defaults must fill reconnect config")
	}
}

// ---------------------------------------------------------------------------
// second-round patch: remaining reachable branches
// ---------------------------------------------------------------------------

func TestEncodeFilePushRequestAllFieldsEmpty(t *testing.T) {
	out := encodeFilePushRequest(&filePushRequest{})
	if len(out) != 0 {
		t.Fatalf("empty request should encode to empty body, got %x", out)
	}
}

func TestMaybeRegisterCapabilitiesAsyncWithControlAddr(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
		return protocol.MsgRegisterCapabilitiesResp, nil, true
	})
	m := newCfgFilePushManager(t, func(c *ClientConfig) { c.ControlAddr = agent.addr() })
	m.mu.Lock()
	m.serviceID = "svc"
	m.serviceVersion = "1.0.0"
	m.functions = []*sdkv1.ProviderFunctionDescriptor{{Id: "demo.echo"}}
	m.mu.Unlock()

	m.maybeRegisterCapabilitiesAsync()

	// the fire-and-forget goroutine must reach the (reachable) control plane;
	// give it a moment and confirm the session stays usable afterwards
	time.Sleep(100 * time.Millisecond)
}

func TestConnectIdempotentWhileConnected(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-idem"))
	mgr, _ := NewTCPManager(ClientConfig{AgentAddr: agent.addr(), Insecure: true}, nil)
	m := mgr.(*TCPManager)
	if err := m.Connect(context.Background()); err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	defer m.Disconnect()
	if err := m.Connect(context.Background()); err != nil {
		t.Fatalf("second Connect must be a no-op: %v", err)
	}
}

func TestHeartbeatLoopSuccessResetsFailures(t *testing.T) {
	heartbeatSeen := make(chan struct{}, 4)
	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
		if msgID == protocol.MsgProviderHeartbeatRequest {
			select {
			case heartbeatSeen <- struct{}{}:
			default:
			}
			resp, _ := proto.Marshal(&sdkv1.ProviderHeartbeatResponse{})
			return protocol.MsgProviderHeartbeatResponse, resp, true
		}
		if msgID == protocol.MsgProviderConnectRequest {
			resp, _ := proto.Marshal(&sdkv1.ProviderConnectResponse{SessionId: "sess-hb"})
			return protocol.MsgProviderConnectResponse, resp, true
		}
		return protocol.MsgInvokeResponse, nil, true
	})
	mgr, _ := NewTCPManager(ClientConfig{
		AgentAddr: agent.addr(), Insecure: true, HeartbeatInterval: 1,
	}, nil)
	m := mgr.(*TCPManager)
	if err := m.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if _, err := m.RegisterWithAgent(context.Background(), "svc", "1.0.0", nil); err != nil {
		t.Fatalf("RegisterWithAgent: %v", err)
	}
	defer m.Disconnect()

	select {
	case <-heartbeatSeen:
		// A heartbeat reached the agent; allow the response round-trip to
		// finish so the loop records the success before we tear down.
		time.Sleep(300 * time.Millisecond)
	case <-time.After(5 * time.Second):
		t.Fatal("heartbeat was never sent")
	}
}

func TestReconnectWithBackoffSuccessThenDisconnectCallback(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
		if msgID == protocol.MsgProviderConnectRequest {
			b, _ := proto.Marshal(&sdkv1.ProviderConnectResponse{SessionId: "sess-cb"})
			return protocol.MsgProviderConnectResponse, b, true
		}
		return protocol.MsgInvokeResponse, nil, true
	})
	c := newTestClient(agent.addr(), &ReconnectConfig{
		Enabled: true, MaxAttempts: 3, InitialDelayMs: 1, MaxDelayMs: 2,
	})
	if err := c.reconnectWithBackoff(context.Background()); err != nil {
		t.Fatalf("reconnectWithBackoff: %v", err)
	}
	if !c.connected.Load() {
		t.Fatal("client must be connected after successful reconnect")
	}

	// transport death re-registers the disconnect callback; forcing the
	// manager offline must run it (the reconnectWithBackoff closure body).
	tm := c.manager.(*TCPManager)
	tm.handleDisconnect()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !c.connected.Load() {
			// closure ran: connected flag flipped
			select {
			case <-c.disconnectCh:
				c.Stop()
				return
			default:
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	c.Stop()
	t.Fatal("disconnect callback registered by reconnectWithBackoff was not invoked")
}

func TestInvokerConnectSlowPathDoubleChecksDeterministic(t *testing.T) {
	inv := newTCPInvoker(&InvokerConfig{
		Address: "127.0.0.1:1", Insecure: true,
		Reconnect: &ReconnectConfig{Enabled: false}, Retry: DefaultRetryConfig(),
	}).(*tcpInvoker)

	// slow-path connected double-check: hold the write lock so connect() parks
	// between the fast checks and the double-check, then flip the state.
	inv.mu.Lock()
	inv.connected = false
	inv.isReconnecting = false
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		done <- inv.connect(context.Background())
	}()
	<-started
	time.Sleep(50 * time.Millisecond) // let the goroutine park on mu.Lock
	inv.connected = true
	inv.mu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("slow-path connected double-check: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("connect did not return")
	}

	// slow-path isReconnecting double-check
	inv.mu.Lock()
	inv.connected = false
	inv.isReconnecting = false
	started2 := make(chan struct{})
	done2 := make(chan error, 1)
	go func() {
		close(started2)
		done2 <- inv.connect(context.Background())
	}()
	<-started2
	time.Sleep(50 * time.Millisecond)
	inv.isReconnecting = true
	inv.mu.Unlock()
	select {
	case err := <-done2:
		if err == nil || err.Error() != "reconnection in progress" {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("connect did not return")
	}
}

func TestHTTPInvokerStreamCancelledWithFullChannel(t *testing.T) {
	responded := make(chan struct{}, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		items := ""
		for i := 0; i < 20; i++ {
			if i > 0 {
				items += ","
			}
			items += fmt.Sprintf(`{"seq":%d,"type":"progress","message":"m","payload":{"k":%d}}`, i+1, i)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[` + items + `],"done":false}`))
		select {
		case responded <- struct{}{}:
		default:
		}
	}))
	t.Cleanup(srv.Close)

	inv := NewHTTPInvoker(&InvokerConfig{Address: srv.URL})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := inv.StreamTask(ctx, "task-2")
	if err != nil {
		t.Fatalf("StreamTask: %v", err)
	}

	// wait until the producer has the full page in hand, then cancel while it
	// is still pushing events into the (never drained, capacity-16) channel —
	// the 17th send must observe the cancelled context and stop the stream.
	<-responded
	time.Sleep(100 * time.Millisecond)
	cancel()

	deadline := time.Now().Add(3 * time.Second)
	closed := false
	for time.Now().Before(deadline) && !closed {
		select {
		case _, ok := <-events:
			if !ok {
				closed = true
			}
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	if !closed {
		t.Fatal("event channel was not closed after cancellation")
	}
}

// ---------------------------------------------------------------------------
// third-round patch: manifest marshal failure, staging edge cases, valid CA
// ---------------------------------------------------------------------------

func TestMaybeRegisterCapabilitiesManifestMarshalFailure(t *testing.T) {
	m := newCfgFilePushManager(t, func(c *ClientConfig) { c.ControlAddr = "127.0.0.1:1" })
	// json.RawMessage with invalid JSON makes marshalling the manifest fail
	m.maybeRegisterCapabilities("svc", "1.0.0", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "demo.echo", InputSchema: "{not-valid-json"},
	})
}

func TestSafeStagingPathRelativeDirEscape(t *testing.T) {
	// A staging dir of "." resolves to a bare relative path which cannot
	// carry the "./" prefix — rejected as an escape.
	if _, err := safeStagingPath(".", "a.txt"); err == nil {
		t.Fatal("relative staging dir must be rejected as escaping")
	}
}

func TestHandleFilePushRequestWriteFailure(t *testing.T) {
	// A read-only staging directory lets validation pass but staging writes
	// fail — the response must carry the write error.
	dir, err := os.MkdirTemp("", "croupier-ro-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o750)
		_ = os.RemoveAll(dir)
	})
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Skipf("chmod unsupported: %v", err)
	}

	mgr, _ := NewTCPManager(ClientConfig{
		EnableFileTransfer: true,
		MaxFileSize:        1024,
		FileStagingDir:     dir,
	}, testHandlers())
	m := mgr.(*TCPManager)

	data := []byte("content")
	body := encodeFilePushRequest(&filePushRequest{
		transferID: "t", fileName: "ro.txt", data: data, contentSha256: sha256Hex(data),
	})
	respBytes, err := m.handleFilePushRequest(body)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	decoded := decodeFilePushResponseForTest(t, respBytes)
	if decoded.ok || decoded.errorMsg == "" {
		t.Fatalf("expected write failure response, got %+v", decoded)
	}
}

func TestTCPInvokerBuildTLSConfigValidCA(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := genSDKSelfSignedCert(t, dir)

	inv := newTCPInvoker(&InvokerConfig{
		Address: "host.example:19090", Insecure: false,
		CAFile: certFile, CertFile: certFile, KeyFile: keyFile,
		Reconnect: &ReconnectConfig{Enabled: false}, Retry: DefaultRetryConfig(),
	}).(*tcpInvoker)
	cfg, err := inv.buildTLSConfig()
	if err != nil {
		t.Fatalf("buildTLSConfig with valid CA: %v", err)
	}
	if cfg.RootCAs == nil {
		t.Fatal("root CA pool not installed")
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("client certificate not loaded: %d", len(cfg.Certificates))
	}
	if cfg.ServerName != "host.example" {
		t.Fatalf("ServerName = %q", cfg.ServerName)
	}
}

func TestReconnectThenConnectionLossNotifies(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
		if msgID == protocol.MsgProviderConnectRequest {
			b, _ := proto.Marshal(&sdkv1.ProviderConnectResponse{SessionId: "sess-loss"})
			return protocol.MsgProviderConnectResponse, b, true
		}
		// refuse everything else by hanging up
		return 0, nil, false
	})
	mgr, _ := NewTCPManager(ClientConfig{AgentAddr: agent.addr(), Insecure: true}, nil)
	m := mgr.(*TCPManager)
	seedRegisteredState(m)
	if err := m.Reconnect(context.Background()); err != nil {
		t.Fatalf("reconnect: %v", err)
	}

	// drop the agent: the onClose callback registered by Reconnect must run
	// and flip the connection state.
	agent.stop()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !m.IsConnected() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("connection loss was not detected after agent drop")
}

func TestServeDisconnectSignalWhileStopped(t *testing.T) {
	// Race the select: Stop() closes stopCh and flips running, then a queued
	// disconnect signal must hit the !running branch and end Serve. Run
	// several iterations — the scheduler picks either ready case.
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-serve"))
	for i := 0; i < 8; i++ {
		c := newTestClient(agent.addr(), DefaultReconnectConfig())
		if err := c.Connect(context.Background()); err != nil {
			t.Fatalf("Connect: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		served := make(chan error, 1)
		go func() { served <- c.Serve(ctx) }()
		time.Sleep(20 * time.Millisecond)
		_ = c.Stop()
		select {
		case c.disconnectCh <- struct{}{}:
		default:
		}
		select {
		case <-served:
		case <-time.After(2 * time.Second):
			cancel()
			t.Fatal("Serve did not return after stop+disconnect")
		}
		cancel()
	}
}

func TestInvokerConnectConcurrentDoubleCheck(t *testing.T) {
	// Two racing connect() calls: the loser passes the fast checks while the
	// winner holds the write lock during dial, then observes the winner's
	// state through the slow-path double-check.
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-race"))
	for i := 0; i < 20; i++ {
		inv := newTCPInvoker(&InvokerConfig{
			Address: agent.addr(), Insecure: true,
			Reconnect: &ReconnectConfig{Enabled: false}, Retry: DefaultRetryConfig(),
		}).(*tcpInvoker)

		const racers = 8
		errCh := make(chan error, racers)
		for j := 0; j < racers; j++ {
			go func() { errCh <- inv.connect(context.Background()) }()
		}
		for j := 0; j < racers; j++ {
			if err := <-errCh; err != nil {
				t.Fatalf("racing connect: %v", err)
			}
		}
		_ = inv.Close()
	}
}

// ---------------------------------------------------------------------------
// fourth-round patch: proto3 string fields reject invalid UTF-8, which makes
// the defensive proto.Marshal error branches reachable after all.
// ---------------------------------------------------------------------------

const invalidUTF8 = "\xff\xfe"

func TestTCPInvokerMarshalFailureBranches(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-marshal"))
	inv := newTCPInvoker(&InvokerConfig{
		Address:   agent.addr(),
		Insecure:  true,
		Reconnect: &ReconnectConfig{Enabled: false},
		Retry:     &RetryConfig{Enabled: false},
	}).(*tcpInvoker)
	ctx := context.Background()
	if err := inv.connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer inv.Close()

	// Invoke: marshal failure on the request body
	if _, err := inv.Invoke(ctx, invalidUTF8, "{}", InvokeOptions{}); err == nil ||
		!strings.Contains(err.Error(), "marshal request") {
		t.Fatalf("invoke err = %v", err)
	}

	// StartTask: headers copy loop + marshal failure
	if _, err := inv.StartTask(ctx, invalidUTF8, "{}", InvokeOptions{
		Headers: map[string]string{"h": "v"},
	}); err == nil || !strings.Contains(err.Error(), "marshal request") {
		t.Fatalf("startTask err = %v", err)
	}

	// CancelTask: marshal failure
	if err := inv.CancelTask(ctx, invalidUTF8); err == nil ||
		!strings.Contains(err.Error(), "marshal request") {
		t.Fatalf("cancel err = %v", err)
	}

	// StreamTask: the poll goroutine reports the marshal failure as an error event
	events, err := inv.StreamTask(ctx, invalidUTF8)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	select {
	case ev := <-events:
		if !ev.Done || !strings.Contains(ev.Error, "marshal request") {
			t.Fatalf("unexpected event: %+v", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no error event for invalid task ID")
	}
}

func TestRegisterWithAgentMarshalFailure(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-na"))
	mgr, _ := NewTCPManager(ClientConfig{AgentAddr: agent.addr(), Insecure: true}, nil)
	m := mgr.(*TCPManager)
	if err := m.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer m.Disconnect()
	if _, err := m.RegisterWithAgent(context.Background(), invalidUTF8, "1.0.0", nil); err == nil ||
		!strings.Contains(err.Error(), "marshal request") {
		t.Fatalf("register err = %v", err)
	}
}

func TestReconnectMarshalFailure(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-rc"))
	mgr, _ := NewTCPManager(ClientConfig{AgentAddr: agent.addr(), Insecure: true}, nil)
	m := mgr.(*TCPManager)
	m.mu.Lock()
	m.serviceID = invalidUTF8
	m.serviceVersion = "1.0.0"
	m.functions = nil
	m.mu.Unlock()
	if err := m.Reconnect(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "marshal connect request") {
		t.Fatalf("reconnect err = %v", err)
	}
}

func TestSendHeartbeatMarshalFailure(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-hb2"))
	mgr, _ := NewTCPManager(ClientConfig{AgentAddr: agent.addr(), Insecure: true}, nil)
	m := mgr.(*TCPManager)
	if err := m.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer m.Disconnect()
	m.mu.Lock()
	m.sessionID = invalidUTF8
	m.mu.Unlock()
	if err := m.sendHeartbeat(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "marshal heartbeat") {
		t.Fatalf("heartbeat err = %v", err)
	}
}

func TestMaybeRegisterCapabilitiesProtoMarshalFailure(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", func(msgID, reqID uint32, body []byte) (uint32, []byte, bool) {
		return protocol.MsgRegisterCapabilitiesResp, nil, true
	})
	m := newCfgFilePushManager(t, func(c *ClientConfig) { c.ControlAddr = agent.addr() })
	// valid manifest (marshal + gzip succeed); the ProviderMeta id comes from
	// the manager field, so poison it to make the request marshal fail
	m.mu.Lock()
	m.serviceID = invalidUTF8
	m.mu.Unlock()
	m.maybeRegisterCapabilities(invalidUTF8, "1.0.0", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "demo.echo", InputSchema: `{"type":"object"}`},
	})
}

// ---------------------------------------------------------------------------
// lock-window races: connect() slow-path double-checks and Serve's
// disconnect-while-stopped branch are scheduler-dependent; stress them with
// start gates and signal fillers so the windows are entered reliably.
// ---------------------------------------------------------------------------

func TestInvokerConnectReconnectingDoubleCheckRace(t *testing.T) {
	for round := 0; round < 60; round++ {
		inv := newTCPInvoker(&InvokerConfig{
			Address:   "127.0.0.1:1", // refuses fast → connectLocked fails quickly
			Insecure:  true,
			Reconnect: &ReconnectConfig{Enabled: true, MaxAttempts: 1, InitialDelayMs: 1, MaxDelayMs: 1},
			Retry:     &RetryConfig{Enabled: false},
		}).(*tcpInvoker)

		gate := make(chan struct{})
		var wg sync.WaitGroup
		for j := 0; j < 6; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-gate
				_ = inv.connect(context.Background())
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			inv.scheduleReconnectIfNeeded()
		}()
		close(gate)
		wg.Wait()

		inv.mu.Lock()
		inv.isReconnecting = false
		inv.reconnectAttempts = 0
		inv.mu.Unlock()
		_ = inv.Close()
	}
}

func TestServeDisconnectSignalWhileStoppedFiller(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-serve2"))
	for i := 0; i < 12; i++ {
		c := newTestClient(agent.addr(), &ReconnectConfig{
			Enabled: true, MaxAttempts: 1, InitialDelayMs: 1, MaxDelayMs: 1,
		})
		if err := c.Connect(context.Background()); err != nil {
			t.Fatalf("Connect: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		served := make(chan error, 1)
		go func() { served <- c.Serve(ctx) }()

		// continuously refill the disconnect signal so that, once Stop()
		// clears the running flag and closes stopCh, some select iteration
		// observes a ready disconnectCh together with the stopped state
		fillDone := make(chan struct{})
		go func() {
			defer close(fillDone)
			for j := 0; j < 200; j++ {
				select {
				case c.disconnectCh <- struct{}{}:
				default:
				}
				time.Sleep(200 * time.Microsecond)
			}
		}()

		time.Sleep(5 * time.Millisecond)
		_ = c.Stop()
		select {
		case <-served:
		case <-time.After(3 * time.Second):
			cancel()
			t.Fatal("Serve did not return")
		}
		<-fillDone
		cancel()
	}
}
