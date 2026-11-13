package controlconn

import (
	"context"
	"net"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/core/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// muxPeerServer 起一台真 MuxConn 服务端：处理注册/心跳/自定义下行请求，
// 并可向 agent 方向单发事件帧（会话事件流的反向验证面）。
type muxPeerServer struct {
	t *testing.T
	// onRequest 由测试注入：返回 (response, handled)。handled=false 时回空体。
	onRequest func(msgID uint32, body []byte) (proto.Message, bool)
	ln        net.Listener
	mux       *tcp.MuxConn
}

func newMuxPeerServer(t *testing.T, onRequest func(uint32, []byte) (proto.Message, bool)) *muxPeerServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	s := &muxPeerServer{t: t, onRequest: onRequest, ln: ln}
	go s.serve()
	return s
}

func (s *muxPeerServer) serve() {
	conn, err := s.ln.Accept()
	if err != nil {
		return
	}
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		if s.onRequest == nil {
			return nil, nil
		}
		resp, ok := s.onRequest(msgID, body)
		if !ok {
			return nil, nil
		}
		return proto.Marshal(resp)
	})
	s.mux = tcp.NewMuxConn(conn, &tcp.Config{RecvTimeout: 2 * time.Second, SendTimeout: 2 * time.Second}, handler)
	_ = s.mux.Run(context.Background())
}

// Addr 服务端地址。
func (s *muxPeerServer) Addr() string { return s.ln.Addr().String() }

// SendEvent 向 agent 方向单发事件帧（验证 agent 侧 handler 收事件）。
func (s *muxPeerServer) SendEvent(ctx context.Context, msgID uint32, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	return s.mux.Send(ctx, msgID, data)
}

// TestMuxSessionRegisterHeartbeatAndInbound 全链路：MuxSession 上行注册/心跳
// （MuxConn Call 面），server 下行请求经 handler 回帧，事件单发抵达 handler。
func TestMuxSessionRegisterHeartbeatAndInbound(t *testing.T) {
	inboundOpen := make(chan *opsv1.DevopsSessionOpenRequest, 1)
	upEvent := make(chan *opsv1.DevopsSessionEvent, 1)
	downEvent := make(chan *opsv1.DevopsSessionEvent, 1)
	srv := newMuxPeerServer(t, func(msgID uint32, body []byte) (proto.Message, bool) {
		switch msgID {
		case protocol.MsgRegisterRequest:
			return &agentv1.RegisterResponse{SessionId: "sess-mux"}, true
		case protocol.MsgHeartbeatRequest:
			return &agentv1.HeartbeatResponse{}, true
		case protocol.MsgDevopsSessionOpenRequest:
			req := &opsv1.DevopsSessionOpenRequest{}
			if err := proto.Unmarshal(body, req); err != nil {
				t.Errorf("unmarshal open request: %v", err)
				return &opsv1.DevopsSessionOpenResponse{Ok: false}, true
			}
			inboundOpen <- req
			return &opsv1.DevopsSessionOpenResponse{Ok: true, SessionId: req.SessionId, Channel: "ssh"}, true
		case protocol.MsgDevopsSessionEvent:
			ev := &opsv1.DevopsSessionEvent{}
			if err := proto.Unmarshal(body, ev); err == nil {
				upEvent <- ev
			}
			return nil, false
		default:
			return nil, false
		}
	})

	sess, err := DialMux(srv.Addr(), nil, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		if msgID == protocol.MsgDevopsSessionEvent {
			ev := &opsv1.DevopsSessionEvent{}
			if err := proto.Unmarshal(body, ev); err != nil {
				t.Errorf("unmarshal event: %v", err)
				return nil, nil
			}
			downEvent <- ev
		}
		return nil, nil
	}))
	if err != nil {
		t.Fatalf("DialMux: %v", err)
	}
	defer func() { _ = sess.Close() }()

	reg, err := sess.Register(context.Background(), &agentv1.RegisterRequest{AgentId: "devops-1"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if reg.GetSessionId() != "sess-mux" {
		t.Fatalf("session token = %q", reg.GetSessionId())
	}
	if _, err := sess.Heartbeat(context.Background(), &agentv1.HeartbeatRequest{AgentId: "devops-1"}); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	// server 下行会话请求 → agent handler 已回帧给 Call。
	openResp := &opsv1.DevopsSessionOpenResponse{}
	if err := sess.Call(context.Background(), protocol.MsgDevopsSessionOpenRequest,
		&opsv1.DevopsSessionOpenRequest{SessionId: "s1", Operator: "admin", Protocol: "ssh"}, openResp); err != nil {
		t.Fatalf("call open: %v", err)
	}
	if !openResp.GetOk() || openResp.GetSessionId() != "s1" {
		t.Fatalf("open response = %+v", openResp)
	}
	select {
	case req := <-inboundOpen:
		if req.GetOperator() != "admin" {
			t.Fatalf("operator = %q", req.GetOperator())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("open request never reached the agent handler")
	}

	// agent → server 事件单发（server 读循环 → handler）。
	if err := sess.Send(context.Background(), protocol.MsgDevopsSessionEvent,
		&opsv1.DevopsSessionEvent{SessionId: "s1", Kind: "output", Data: []byte("hi")}); err != nil {
		t.Fatalf("send event: %v", err)
	}
	select {
	case ev := <-upEvent:
		if string(ev.GetData()) != "hi" {
			t.Fatalf("uplink event data = %q", ev.GetData())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("uplink event never reached the server handler")
	}

	// server → agent 事件单发（agent 读循环 → handler）。
	if err := srv.SendEvent(context.Background(), protocol.MsgDevopsSessionEvent,
		&opsv1.DevopsSessionEvent{SessionId: "s1", Kind: "end", Outcome: "eof"}); err != nil {
		t.Fatalf("server send event: %v", err)
	}
	select {
	case ev := <-downEvent:
		if ev.GetOutcome() != "eof" {
			t.Fatalf("downlink event outcome = %q", ev.GetOutcome())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("downlink event never reached the agent handler")
	}
}

// TestMuxSessionOnClosed 断连回调恰发一次：server 关闭后读循环退出触发。
func TestMuxSessionOnClosed(t *testing.T) {
	srv := newMuxPeerServer(t, func(msgID uint32, _ []byte) (proto.Message, bool) {
		if msgID == protocol.MsgRegisterRequest {
			return &agentv1.RegisterResponse{}, true
		}
		return nil, false
	})
	closed := make(chan struct{}, 2)
	sess, err := DialMux(srv.Addr(), nil, nil)
	if err != nil {
		t.Fatalf("DialMux: %v", err)
	}
	sess.SetOnClosed(func() { closed <- struct{}{} })
	if _, err := sess.Register(context.Background(), &agentv1.RegisterRequest{AgentId: "devops-1"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if !sess.Connected() {
		t.Fatal("session should be connected after register")
	}
	// 服务端收尾（Cleanup 关 listener；直接关 mux 更快）。
	if srv.mux != nil {
		_ = srv.mux.Close()
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("onClosed never fired after server closed")
	}
	// 幂等：连接自身再关不重复触发。
	_ = sess.Close()
	select {
	case <-closed:
		t.Fatal("onClosed fired twice")
	default:
	}
}
