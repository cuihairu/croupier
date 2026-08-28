package cluster

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/transport/tcp"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// interconnectServeHandler 模拟 owner 实例的互联入站处理（对齐
// cmd/server.interconnectHandler 的两类消息：hello + forward）。
type interconnectServeHandler struct {
	epoch   uint64
	payload []byte
}

func (h *interconnectServeHandler) Handle(ctx context.Context, msgID uint32, _ uint32, body []byte) ([]byte, error) {
	switch msgID {
	case protocol.MsgServerHelloRequest:
		resp, _ := json.Marshal(helloResponse{InstanceID: "owner-inst", Epoch: h.epoch})
		return resp, nil
	case protocol.MsgForwardInvokeReq:
		forward := ServeForwardHandler(h.epoch, func(_ context.Context, req *ForwardedInvoke) (*ForwardedResult, error) {
			return &ForwardedResult{OK: true, Payload: h.payload}, nil
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
	srv, err := tcp.NewServer(srvCfg, &interconnectServeHandler{epoch: 7, payload: []byte(`{"ok":true}`)})
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
	if string(res.Payload) != `{"ok":true}` {
		t.Fatalf("payload = %s", res.Payload)
	}
}

var _ net.Addr = (net.Addr)(nil)

type staticOwnerResolver struct {
	owner *PeerInfo
}

func (s *staticOwnerResolver) ResolveOwner(context.Context, string) (*PeerInfo, error) {
	return s.owner, nil
}
