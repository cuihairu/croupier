package tcp

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"

	transportcore "github.com/cuihairu/croupier/core/transport"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// hostileErrText 覆盖三类会破坏手写 JSON 的字符：双引号、反斜杠、换行。
const hostileErrText = `he"llo\world` + "\n" + "second line"

// TestEncodeErrorPayload_EscapesHostileText 验证错误文档经标准编码器生成：
// 引号/反斜杠/换行不再破坏 JSON，客户端 json.Unmarshal 能还原原文。
func TestEncodeErrorPayload_EscapesHostileText(t *testing.T) {
	raw := EncodeErrorPayload(hostileErrText)
	if string(raw) == `{"error":"`+hostileErrText+`"}` {
		t.Fatalf("error text was spliced raw instead of JSON-encoded: %s", raw)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("error payload is not valid JSON (AUD-002 regression): %v\n%s", err, raw)
	}
	if doc["error"] != hostileErrText {
		t.Fatalf("decoded error = %q, want %q", doc["error"], hostileErrText)
	}
}

// TestEncodeVersionRejectedPayload 固定版本拒绝文档的字节形态（既有
// internal/transport/tcp/server_test.go 逐字节断言，此处锁定同一编码口径）。
func TestEncodeVersionRejectedPayload(t *testing.T) {
	want := `{"error":"unsupported protocol version: 2","supportedVersions":[1]}`
	if got := string(EncodeVersionRejectedPayload(2, []int{1})); got != want {
		t.Fatalf("rejection payload = %s, want %s", got, want)
	}
}

// TestInvokeErrorBody_PayloadIsClientReadable 是 Invoke 的客户端可观测结果：
// 响应体解出 InvokeResponse，其 Payload 是合法 JSON 且错误原文可还原。
func TestInvokeErrorBody_PayloadIsClientReadable(t *testing.T) {
	body := RPCFailureBody(protocol.MsgInvokeRequest, hostileErrText)
	if len(body) == 0 {
		t.Fatal("Invoke failure body is empty")
	}
	var resp sdkv1.InvokeResponse
	if err := proto.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal InvokeResponse: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(resp.GetPayload(), &doc); err != nil {
		t.Fatalf("payload is not valid JSON: %v\n%s", err, resp.GetPayload())
	}
	if doc["error"] != hostileErrText {
		t.Fatalf("payload error = %q, want %q", doc["error"], hostileErrText)
	}
}

// TestRPCFailureBody_OpsRPCReportsFailure 是「非 Invoke RPC 不得以空 body
// 伪装零值成功」的逐类型验证：客户端解出类型后能靠自身失败通道识别错误。
func TestRPCFailureBody_OpsRPCReportsFailure(t *testing.T) {
	cases := []struct {
		name  string
		msgID uint32
		// decode 返回该类型解析出的失败通道是否如实呈现错误。
		decode func([]byte) (failed bool, detail string, ok bool)
	}{
		{
			name:  "RestartProcess",
			msgID: protocol.MsgRestartProcessRequest,
			decode: func(b []byte) (bool, string, bool) {
				var r opsv1.RestartProcessResponse
				if err := proto.Unmarshal(b, &r); err != nil {
					return false, "", false
				}
				return !r.GetSuccess(), r.GetMessage(), true
			},
		},
		{
			name:  "StopProcess",
			msgID: protocol.MsgStopProcessRequest,
			decode: func(b []byte) (bool, string, bool) {
				var r opsv1.StopProcessResponse
				if err := proto.Unmarshal(b, &r); err != nil {
					return false, "", false
				}
				return !r.GetSuccess(), r.GetMessage(), true
			},
		},
		{
			name:  "StartProcess",
			msgID: protocol.MsgStartProcessRequest,
			decode: func(b []byte) (bool, string, bool) {
				var r opsv1.StartProcessResponse
				if err := proto.Unmarshal(b, &r); err != nil {
					return false, "", false
				}
				return !r.GetSuccess(), r.GetMessage(), true
			},
		},
		{
			name:  "UpdateProcess",
			msgID: protocol.MsgUpdateProcessRequest,
			decode: func(b []byte) (bool, string, bool) {
				var r opsv1.UpdateProcessResponse
				if err := proto.Unmarshal(b, &r); err != nil {
					return false, "", false
				}
				return !r.GetSuccess(), r.GetMessage(), true
			},
		},
		{
			name:  "SchedulePause",
			msgID: protocol.MsgSchedulePauseRequest,
			decode: func(b []byte) (bool, string, bool) {
				var r opsv1.SchedulePauseResponse
				if err := proto.Unmarshal(b, &r); err != nil {
					return false, "", false
				}
				return !r.GetSuccess(), r.GetMessage(), true
			},
		},
		{
			name:  "PushFile",
			msgID: protocol.MsgPushFileRequest,
			decode: func(b []byte) (bool, string, bool) {
				var r opsv1.PushFileResponse
				if err := proto.Unmarshal(b, &r); err != nil {
					return false, "", false
				}
				return !r.GetSuccess(), r.GetMessage(), true
			},
		},
		{
			name:  "ExecuteCommand",
			msgID: protocol.MsgExecuteCommandRequest,
			decode: func(b []byte) (bool, string, bool) {
				var r opsv1.ExecuteCommandResponse
				if err := proto.Unmarshal(b, &r); err != nil {
					return false, "", false
				}
				return !r.GetSuccess(), r.GetError(), true
			},
		},
		{
			name:  "GetTaskResult",
			msgID: protocol.MsgGetTaskResultRequest,
			decode: func(b []byte) (bool, string, bool) {
				var r sdkv1.GetTaskResultResponse
				if err := proto.Unmarshal(b, &r); err != nil {
					return false, "", false
				}
				return r.GetState() == "failed", r.GetError(), true
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := RPCFailureBody(tc.msgID, hostileErrText)
			if len(body) == 0 {
				t.Fatalf("failure body is empty: AUD-002 requires a client-readable failure")
			}
			failed, detail, ok := tc.decode(body)
			if !ok {
				t.Fatalf("response type could not be decoded: %v", tc.name)
			}
			if !failed {
				t.Fatalf("client reads success instead of failure (zero-value masquerade)")
			}
			if detail != hostileErrText {
				t.Fatalf("failure detail = %q, want %q", detail, hostileErrText)
			}
		})
	}
}

// TestRPCFailureBody_NoChannelTypeReturnsEmpty 锁定诚实边界：无失败通道的
// wire 类型返回 nil（空 body），不伪造字节。
func TestRPCFailureBody_NoChannelTypeReturnsEmpty(t *testing.T) {
	if body := RPCFailureBody(protocol.MsgHeartbeatRequest, "boom"); body != nil {
		t.Fatalf("no-channel type must not fabricate a failure body, got %q", body)
	}
}

// TestMuxConnHandlerError_OpsResponseClientObservable 走真实帧路径：handler
// 报错时客户端读到的响应帧携带失败通道与原文错误，而不是零值成功。
func TestMuxConnHandlerError_OpsResponseClientObservable(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()
	mc := NewMuxConn(c1, nil, transportcore.HandlerFunc(func(_ context.Context, msgID uint32, _ uint32, _ []byte) ([]byte, error) {
		if msgID == protocol.MsgRestartProcessRequest {
			return nil, errors.New(hostileErrText)
		}
		return nil, nil
	}))
	errCh := make(chan error, 1)
	go func() {
		errCh <- mc.handleInboundRequest(context.Background(), protocol.MsgRestartProcessRequest, 9, []byte(`{}`))
	}()
	frame, err := ReadFrame(c2)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	_, msgID, reqID, body, err := protocol.ParseMessageFromBody(frame)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if msgID != protocol.MsgRestartProcessResponse || reqID != 9 {
		t.Fatalf("msgID=%x reqID=%d, want RestartProcessResponse/9", msgID, reqID)
	}
	var resp opsv1.RestartProcessResponse
	if err := proto.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.GetSuccess() {
		t.Fatal("handler error decoded as success (AUD-002 regression)")
	}
	if resp.GetMessage() != hostileErrText {
		t.Fatalf("client-observable message = %q, want %q", resp.GetMessage(), hostileErrText)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("handleInboundRequest: %v", err)
	}
}
