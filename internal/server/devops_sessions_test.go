package server

// AGENT-002 server 侧接线测试：ControlService 派发 MsgDevopsSessionEvent
// 到中继运营面（fanout 可验）；TCPListener 断连清理收束该 agent 的代理
// 会话（控制面丢失 = 会话面关闭，与 agent 侧 OnConnClosed 对称）。

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/internal/platform/devopssessions"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// devopsTestCaller devops 会话 wire 假 agent：open 恒 ok（echo 会话 ID）。
type devopsTestCaller struct{}

func (devopsTestCaller) Call(_ context.Context, msgID uint32, reqBody []byte) (uint32, []byte, error) {
	req := &opsv1.DevopsSessionOpenRequest{}
	_ = proto.Unmarshal(reqBody, req)
	body, _ := proto.Marshal(&opsv1.DevopsSessionOpenResponse{Ok: true, SessionId: req.GetSessionId(), Channel: "ssh"})
	return protocol.GetResponseMsgID(msgID), body, nil
}

type devopsTestResolver struct{}

func (devopsTestResolver) ResolveAgentConn(string) (transport.SessionCaller, bool) {
	return devopsTestCaller{}, true
}

// openDevopsSession 经真实 Open 链登记一条代理会话（operator=alice）。
func openDevopsSession(t *testing.T, hub *devopssessions.Hub, agentID string) *devopssessions.Session {
	t.Helper()
	sess, err := hub.Open(context.Background(), devopsTestResolver{}, agentID, &opsv1.DevopsSessionOpenRequest{
		Operator: "alice", Protocol: "ssh", Host: "h", Port: 22,
	}, "alice", "demo", "prod")
	require.NoError(t, err)
	return sess
}

func TestControlServiceRoutesDevopsSessionEvent(t *testing.T) {
	hub := devopssessions.NewHub(nil)
	svc := newTestControlService()
	svc.SetDevopsSessions(hub)

	sess := openDevopsSession(t, hub, "agent-1")
	sub, ok := hub.AttachConsumer(sess.SessionID)
	require.True(t, ok)

	data, err := proto.Marshal(&opsv1.DevopsSessionEvent{SessionId: sess.SessionID, Kind: "output", Data: []byte("hi")})
	require.NoError(t, err)
	// 单向事件帧：返回值恒 nil（MuxConn 事件路径丢弃返回值，不回帧）。
	resp, err := svc.handleRequestAsAgent(context.Background(), protocol.MsgDevopsSessionEvent, data,
		&AgentIdentity{AgentID: "agent-1", GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	require.Nil(t, resp)

	select {
	case ev := <-sub.C:
		require.Equal(t, "output", ev.GetKind())
		require.Equal(t, "hi", string(ev.GetData()))
	case <-time.After(2 * time.Second):
		t.Fatal("routed event never reached consumer")
	}

	// 未知会话 / 跨 agent 事件：静默丢弃不炸。
	resp, err = svc.handleRequestAsAgent(context.Background(), protocol.MsgDevopsSessionEvent, data,
		&AgentIdentity{AgentID: "agent-1"})
	require.NoError(t, err)
	require.Nil(t, resp)
	resp, err = svc.handleRequestAsAgent(context.Background(), protocol.MsgDevopsSessionEvent, data,
		&AgentIdentity{AgentID: "other-agent"})
	require.NoError(t, err)
	require.Nil(t, resp)

	// 坏载荷：静默丢弃。
	resp, err = svc.handleRequestAsAgent(context.Background(), protocol.MsgDevopsSessionEvent, []byte("{bad"),
		&AgentIdentity{AgentID: "agent-1"})
	require.NoError(t, err)
	require.Nil(t, resp)
}

// TestControlServiceDevopsEventWithoutHub 未装配中继（nil hub）时事件帧
// 不炸不回帧（事件面不是审计真值，可丢）。
func TestControlServiceDevopsEventWithoutHub(t *testing.T) {
	svc := newTestControlService()
	data, _ := proto.Marshal(&opsv1.DevopsSessionEvent{SessionId: "s", Kind: "output"})
	resp, err := svc.handleRequestAsAgent(context.Background(), protocol.MsgDevopsSessionEvent, data,
		&AgentIdentity{AgentID: "agent-1"})
	require.NoError(t, err)
	require.Nil(t, resp)
}

// TestTCPListenerDisconnectClosesDevopsSessions serveConn 断连清理块收束
// 该 agent 的全部代理会话：注册建会话 → hub 预登记 agent-1 的代理会话 →
// 断开管道 → 会话路由态被摘除，消费者收到合成 end。
func TestTCPListenerDisconnectClosesDevopsSessions(t *testing.T) {
	config := &TCPListenerConfig{Address: "127.0.0.1:0", Insecure: true}
	listener, err := NewTCPListener(config, NewAgentSessionStore(), nil, nil)
	require.NoError(t, err)
	svc := newTestControlService()
	listener.SetHandler(svc)

	hub := devopssessions.NewHub(nil)
	svc.SetDevopsSessions(hub)
	listener.SetDevopsSessions(hub)

	sess := openDevopsSession(t, hub, "agent-1")
	sub, ok := hub.AttachConsumer(sess.SessionID)
	require.True(t, ok)

	serverEnd, clientEnd := net.Pipe()
	defer func() { _ = clientEnd.Close() }()
	go func() { // 排水：mux 响应帧同步写管道
		buf := make([]byte, 4096)
		for {
			if _, err := clientEnd.Read(buf); err != nil {
				return
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go listener.serveConn(ctx, serverEnd)

	regReq, _ := proto.Marshal(&agentv1.RegisterRequest{AgentId: "agent-1", GameId: "demo", Env: "prod"})
	frame := protocol.NewMessageBody(protocol.MsgRegisterRequest, 1, regReq)
	wrapped := make([]byte, 4+len(frame))
	wrapped[0] = byte(len(frame) >> 24)
	wrapped[1] = byte(len(frame) >> 16)
	wrapped[2] = byte(len(frame) >> 8)
	wrapped[3] = byte(len(frame))
	copy(wrapped[4:], frame)
	_, werr := clientEnd.Write(wrapped)
	require.NoError(t, werr)

	// 等注册生效。
	store := listener.SessionStore()
	deadline := time.Now().Add(2 * time.Second)
	registered := false
	for time.Now().Before(deadline) {
		if _, ok := store.Get("agent-1"); ok {
			registered = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, registered, "register must create agent session")

	// 断开：serveConn 清理块 → RemoveSession + CloseAgentSessions。
	_ = clientEnd.Close()
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := hub.Get(sess.SessionID); !ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, ok = hub.Get(sess.SessionID)
	require.False(t, ok, "proxied session must be closed on agent disconnect")
	select {
	case ev := <-sub.C:
		require.Equal(t, "end", ev.GetKind())
		require.Equal(t, "agent_disconnected", ev.GetOutcome())
	default:
		t.Fatal("synthetic end event never delivered")
	}
}
