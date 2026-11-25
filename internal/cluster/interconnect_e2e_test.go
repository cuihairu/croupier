package cluster

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/transport/tcp"
	internaltcp "github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// interconnectServeHandler simulates the owner instance's interconnect inbound
// handling (mirroring cmd/server.interconnectHandler's two message kinds:
// hello + forward).
type interconnectServeHandler struct {
	epoch     uint64
	rawResult []byte
}

func (h *interconnectServeHandler) Handle(ctx context.Context, msgID uint32, _ uint32, body []byte) ([]byte, error) {
	switch msgID {
	case protocol.MsgServerHelloRequest:
		return ServeHelloHandler(PeerInfo{InstanceID: "owner-inst"}, h.epoch)(ctx, body), nil
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

// TestInterconnect_E2EForwardDialAndServe is a regression test for the
// interconnect path end to end — the owner runs tcp.Server(ServeForwardHandler)
// and the caller mesh dials with a real NewTCPDialer via Forward and gets the
// result back. Past defects: the server omitted Insecure and listened with a
// nil TLS config which always fails, and the dialer was never injected (the
// defaultDialPeer stub always errors) — any regression turns this test red.
func TestInterconnect_E2EForwardDialAndServe(t *testing.T) {
	srvCfg := &tcp.Config{
		Address:     "127.0.0.1:0",
		RecvTimeout: 5 * time.Second,
		SendTimeout: 5 * time.Second,
		Insecure:    true,
	}
	// The owner side returns the proto-encoded InvokeResponse (the real
	// contract of ForwardedResult.Payload; in real scenarios localInvoker fills
	// in the dispatcher output).
	protoResp, err := proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`{"ok":true}`)})
	if err != nil {
		t.Fatalf("marshal fixture response: %v", err)
	}
	srv, err := internaltcp.NewServer(srvCfg, &interconnectServeHandler{epoch: 7, rawResult: protoResp})
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
		Insecure:       true, // same semantics as the production wiring: plaintext on the intranet
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

// startTaskServeHandler simulates the owner: it captures the decoded forward
// frame (verifying kind/taskId survive the JSON frame round trip) and returns
// a fixed proto-encoded response.
type startTaskServeHandler struct {
	epoch   uint64
	capture func(*ForwardedInvoke)
	raw     []byte
}

func (h *startTaskServeHandler) Handle(ctx context.Context, msgID uint32, _ uint32, body []byte) ([]byte, error) {
	switch msgID {
	case protocol.MsgServerHelloRequest:
		return ServeHelloHandler(PeerInfo{InstanceID: "owner-inst"}, h.epoch)(ctx, body), nil
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

// TestInterconnect_E2EForwardKindRoundTrip is a regression test: all three
// kinds share the same frame format (loop prevention/fencing/stamping use it
// alike). A kind=start_task frame round-trips over a real TCP mesh without
// losing Kind/Metadata.taskId/Caller on the owner side; when an older caller
// leaves Kind unset the owner decodes it as the zero value "" (invoke
// compatibility).
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
	srv, err := internaltcp.NewServer(srvCfg, &startTaskServeHandler{
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

	// Older caller (no Kind set) → the owner decodes "" and dispatches as
	// invoke.
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
