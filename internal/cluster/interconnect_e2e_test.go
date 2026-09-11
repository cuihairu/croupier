package cluster

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// interconnectServeHandler 模拟 owner 实例的互联入站处理（对齐
// cmd/server.interconnectHandler 的两类消息：hello + forward）。
type interconnectServeHandler struct {
	epoch     uint64
	payload   []byte
	rawResult []byte
}

func (h *interconnectServeHandler) Handle(ctx context.Context, msgID uint32, _ uint32, body []byte) ([]byte, error) {
	switch msgID {
	case protocol.MsgServerHelloRequest:
		resp, _ := json.Marshal(helloResponse{InstanceID: "owner-inst", Epoch: h.epoch})
		return resp, nil
	case protocol.MsgForwardInvokeReq:
		forward := ServeForwardHandler(h.epoch, func(_ context.Context, req *ForwardedInvoke) (*ForwardedResult, error) {
			return &ForwardedResult{OK: true, Payload: h.rawResult}, nil
		})
		return forward(ctx, body), nil
	default:
		return nil, errUnexpectedMsg
	}
}

var errUnexpectedMsg = &testError{"unexpected msg"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

// TestInterconnect_E2EForwardDialAndServe 回归：互联链路端到端——owner 起
// tcp.Server(ServeForwardHandler)，调用方 mesh 经真实 NewTCPDialer 拨号
// Forward 并拿回结果。历史缺陷：server 漏设 Insecure 走空 TLS 监听必败、
// dialer 从未注入（defaultDialPeer 桩恒错）——任一回归本测试都会红。
func TestInterconnect_E2EForwardDialAndServe(t *testing.T) {
	srvCfg := &tcp.Config{
		Address:     "127.0.0.1:0",
		RecvTimeout: 5 * time.Second,
		SendTimeout: 5 * time.Second,
		Insecure:    true, // 与生产接线同语义：内网明文
	}
	// owner 侧返回 proto 编码的 InvokeResponse（ForwardedResult.Payload
	// 的真实契约；真实场景由 localInvoker 填充 dispatcher 输出）。
	protoResp, err := proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`{"ok":true}`)})
	if err != nil {
		t.Fatalf("marshal fixture response: %v", err)
	}
	srv, err := tcp.NewServer(srvCfg, &interconnectServeHandler{epoch: 7, rawResult: protoResp})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	go func() { _ = srv.Serve(context.Background()) }()
	defer func() { _ = srv.Close() }()

	owner := PeerInfo{InstanceID: "owner-inst", AdvertiseAddr: srv.Addr(), Epoch: 7}
	caller := NewMeshInterconnect(PeerInfo{InstanceID: "caller-inst"}, &staticOwnerResolver{owner: &owner}, nil)
	caller.dial = NewTCPDialer(&tcp.Config{
		RecvTimeout:    5 * time.Second,
		SendTimeout:    5 * time.Second,
		ConnectTimeout: 3 * time.Second,
		Insecure:       true,
	})

	res, err := caller.Forward(context.Background(), "agent-on-owner", &ForwardedInvoke{
		FunctionID: "demo.fn",
		Payload:    []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if !res.OK {
		t.Fatalf("forward result not ok: %+v", res)
	}
	var resp sdkv1.InvokeResponse
	if err := proto.Unmarshal(res.Payload, &resp); err != nil {
		t.Fatalf("payload must be proto-encoded InvokeResponse: %v (raw=%x)", err, res.Payload)
	}
	if string(resp.Payload) != `{"ok":true}` {
		t.Fatalf("inner payload = %s", resp.Payload)
	}
}

var _ net.Addr = (net.Addr)(nil)

type staticOwnerResolver struct {
	owner *PeerInfo
}

func (s *staticOwnerResolver) ResolveOwner(context.Context, string) (*PeerInfo, error) {
	return s.owner, nil
}

// startTaskServeHandler 模拟 owner：捕获解码后的转发帧（验证 kind/taskId
// 经 JSON 帧往返不丢），返回固定的 proto 编码响应。
type startTaskServeHandler struct {
	epoch   uint64
	capture func(*ForwardedInvoke)
	raw     []byte
}

func (h *startTaskServeHandler) Handle(ctx context.Context, msgID uint32, _ uint32, body []byte) ([]byte, error) {
	switch msgID {
	case protocol.MsgServerHelloRequest:
		resp, _ := json.Marshal(helloResponse{InstanceID: "owner-inst", Epoch: h.epoch})
		return resp, nil
	case protocol.MsgForwardInvokeReq:
		forward := ServeForwardHandler(h.epoch, func(_ context.Context, req *ForwardedInvoke) (*ForwardedResult, error) {
			h.capture(req)
			return &ForwardedResult{OK: true, Payload: h.raw}, nil
		})
		return forward(ctx, body), nil
	default:
		return nil, errUnexpectedMsg
	}
}

// TestInterconnect_E2EForwardKindRoundTrip 回归：三类 kind 共用同一帧
// 格式（防环/fencing/盖戳共用）。kind=start_task 的帧经真实 TCP mesh 往返
// 后 owner 侧 Kind/Metadata.taskId/Caller 不丢；旧 caller 不填 Kind 时
// owner 解码为零值 ""（invoke 兼容）。
func TestInterconnect_E2EForwardKindRoundTrip(t *testing.T) {
	srvCfg := &tcp.Config{
		Address:     "127.0.0.1:0",
		RecvTimeout: 5 * time.Second,
		SendTimeout: 5 * time.Second,
		Insecure:    true,
	}
	startResp, err := proto.Marshal(&sdkv1.StartTaskResponse{TaskId: "srv-task-1"})
	if err != nil {
		t.Fatalf("marshal fixture response: %v", err)
	}
	var got *ForwardedInvoke
	srv, err := tcp.NewServer(srvCfg, &startTaskServeHandler{
		epoch: 7,
		capture: func(req *ForwardedInvoke) {
			got = req
		},
		raw: startResp,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	go func() { _ = srv.Serve(context.Background()) }()
	defer func() { _ = srv.Close() }()

	owner := PeerInfo{InstanceID: "owner-inst", AdvertiseAddr: srv.Addr(), Epoch: 7}
	caller := NewMeshInterconnect(PeerInfo{InstanceID: "caller-inst"}, &staticOwnerResolver{owner: &owner}, nil)
	caller.dial = NewTCPDialer(&tcp.Config{
		RecvTimeout:    5 * time.Second,
		SendTimeout:    5 * time.Second,
		ConnectTimeout: 3 * time.Second,
		Insecure:       true,
	})

	res, err := caller.Forward(context.Background(), "agent-on-owner", &ForwardedInvoke{
		Kind:       ForwardKindStartTask,
		FunctionID: "demo.fn",
		Metadata:   map[string]string{"taskId": "srv-task-1"},
		Caller:     CallerContext{Username: "alice", GameID: "demo", Env: "prod"},
	})
	if err != nil {
		t.Fatalf("Forward(start_task): %v", err)
	}
	if !res.OK {
		t.Fatalf("forward result not ok: %+v", res)
	}
	var taskResp sdkv1.StartTaskResponse
	if err := proto.Unmarshal(res.Payload, &taskResp); err != nil {
		t.Fatalf("payload must be proto-encoded StartTaskResponse: %v", err)
	}
	if got == nil || got.Kind != ForwardKindStartTask {
		t.Fatalf("owner saw kind = %q, want %q", gotKind(got), ForwardKindStartTask)
	}
	if got.Metadata["taskId"] != "srv-task-1" {
		t.Fatalf("taskId lost in transit: %+v", got.Metadata)
	}
	if got.Caller.Username != "alice" || got.Caller.GameID != "demo" {
		t.Fatalf("caller context lost in transit: %+v", got.Caller)
	}

	// 旧 caller（不填 Kind）→ owner 解码为 ""，按 invoke 分发。
	if _, err := caller.Forward(context.Background(), "agent-on-owner", &ForwardedInvoke{
		FunctionID: "demo.fn",
	}); err != nil {
		t.Fatalf("Forward(legacy no kind): %v", err)
	}
	if got == nil || got.Kind != ForwardKindInvoke {
		t.Fatalf("legacy frame should decode with empty kind, got %q", gotKind(got))
	}
}

func gotKind(req *ForwardedInvoke) string {
	if req == nil {
		return "<nil>"
	}
	return req.Kind
}
