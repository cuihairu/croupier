package controlconn

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// manualServer 应答注册/心跳的帧级服务（reqID 原样回传，wire 与 server 侧
// ControlService 一致）。
func manualServer(t *testing.T, handle func(msgID uint32, req proto.Message) (proto.Message, bool)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		for {
			frame, err := tcp.ReadFrame(conn)
			if err != nil {
				return
			}
			_, msgID, reqID, body, err := protocol.ParseMessageFromBody(frame)
			if err != nil {
				return
			}
			var resp proto.Message
			var ok bool
			switch msgID {
			case protocol.MsgRegisterRequest:
				req := &agentv1.RegisterRequest{}
				if err := proto.Unmarshal(body, req); err != nil {
					return
				}
				if resp, ok = handle(msgID, req); !ok {
					return
				}
			case protocol.MsgHeartbeatRequest:
				req := &agentv1.HeartbeatRequest{}
				if err := proto.Unmarshal(body, req); err != nil {
					return
				}
				if resp, ok = handle(msgID, req); !ok {
					return
				}
			default:
				return
			}
			out, err := proto.Marshal(resp)
			if err != nil {
				return
			}
			respFrame := protocol.NewMessageBody(protocol.MsgRegisterResponse, reqID, out)
			if msgID == protocol.MsgHeartbeatRequest {
				respFrame = protocol.NewMessageBody(protocol.MsgHeartbeatResponse, reqID, out)
			}
			if err := tcp.WriteFrame(conn, respFrame); err != nil {
				return
			}
		}
	}()
	return ln.Addr().String()
}

func TestDialAndRegisterRoundTrip(t *testing.T) {
	addr := manualServer(t, func(msgID uint32, req proto.Message) (proto.Message, bool) {
		reg, ok := req.(*agentv1.RegisterRequest)
		if !ok || reg.AgentId != "capture-01" || reg.GameId != "demo" {
			return nil, false
		}
		return &agentv1.RegisterResponse{SessionId: "sess-1", ExpireAt: 12345}, true
	})
	conn, err := Dial(addr, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := conn.Register(ctx, &agentv1.RegisterRequest{AgentId: "capture-01", GameId: "demo"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if resp.SessionId != "sess-1" {
		t.Fatalf("SessionId = %q", resp.SessionId)
	}
	if !conn.Connected() {
		t.Fatal("conn should report connected")
	}
}

func TestHeartbeatRoundTrip(t *testing.T) {
	addr := manualServer(t, func(msgID uint32, req proto.Message) (proto.Message, bool) {
		hb, ok := req.(*agentv1.HeartbeatRequest)
		if !ok || hb.SessionId != "sess-1" {
			return nil, false
		}
		return &agentv1.HeartbeatResponse{}, true
	})
	conn, err := Dial(addr, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := conn.Heartbeat(ctx, &agentv1.HeartbeatRequest{SessionId: "sess-1"})
	if err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	if resp == nil {
		t.Fatal("HeartbeatResponse = nil")
	}
}

func TestDialFailureAddr(t *testing.T) {
	if _, err := Dial("127.0.0.1:1", nil); err == nil {
		t.Fatal("expected dial failure for closed port")
	}
}

func TestTLSConfigDisabledKeepsInsecure(t *testing.T) {
	// enabled=false 等价 nil：连一个不存在端口也应走裸连路径（不因缺证书报错）。
	_, err := Dial("127.0.0.1:1", &TLS{Enabled: false})
	if err == nil {
		t.Fatal("expected dial error")
	}
	if !contains(err.Error(), "dial control") {
		t.Fatalf("err = %v, want dial control wrap", err)
	}
}

func TestNormalizeAddrVariants(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1:19090":   "127.0.0.1:19090",
		"tcp://127.0.0.1:1": "127.0.0.1:1",
		"tcp:127.0.0.1:2":   "127.0.0.1:2",
	}
	for in, want := range cases {
		if got := normalizeAddr(in); got != want {
			t.Fatalf("normalizeAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNilConnSafe(t *testing.T) {
	var c *Conn
	if c.Connected() {
		t.Fatal("nil conn must not report connected")
	}
	if err := c.Close(); err != nil {
		t.Fatalf("nil Close = %v", err)
	}
	ctx := context.Background()
	if _, err := c.Register(ctx, &agentv1.RegisterRequest{}); err == nil {
		t.Fatal("nil Register must fail")
	}
	if _, err := c.Heartbeat(ctx, &agentv1.HeartbeatRequest{}); err == nil {
		t.Fatal("nil Heartbeat must fail")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOfSub(s, sub) >= 0)
}

func indexOfSub(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
