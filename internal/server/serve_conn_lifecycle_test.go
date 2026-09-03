package server

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// 端到端 serveConn 生命周期（net.Pipe）：注册建会话 → 心跳全路径 →
// agent_id 失配拒绝 → 断开清理（RemoveSession + clusterHooks）。
// 覆盖 serveConn 清理块 / validateHeartbeatAgentID / handleHeartbeatRequest。
func TestServeConn_LifecycleOverPipe(t *testing.T) {
	config := &TCPListenerConfig{Address: "127.0.0.1:0", Insecure: true}
	listener, err := NewTCPListener(config, NewAgentSessionStore(), nil, nil)
	require.NoError(t, err)

	svc := newTestControlService()
	listener.SetHandler(svc)

	hooks := &lifecycleHook{}
	listener.SetClusterHooks(hooks)

	serverEnd, clientEnd := net.Pipe()
	defer clientEnd.Close()
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

	writeFrame := func(msgID uint32, reqID uint32, body []byte) {
		frame := protocol.NewMessageBody(msgID, reqID, body)
		wrapped := make([]byte, 4+len(frame))
		wrapped[0] = byte(len(frame) >> 24)
		wrapped[1] = byte(len(frame) >> 16)
		wrapped[2] = byte(len(frame) >> 8)
		wrapped[3] = byte(len(frame))
		copy(wrapped[4:], frame)
		_, werr := clientEnd.Write(wrapped)
		require.NoError(t, werr)
	}

	// 1. 注册：建会话（agent-1）
	regReq, _ := proto.Marshal(&agentv1.RegisterRequest{AgentId: "agent-1", GameId: "game-1", Env: "dev"})
	writeFrame(protocol.MsgRegisterRequest, 1, regReq)

	// 等注册生效：会话出现在 store
	store := listener.SessionStore()
	deadline := time.Now().Add(2 * time.Second)
	var registered bool
	for time.Now().Before(deadline) {
		if _, ok := store.Get("agent-1"); ok {
			registered = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, registered, "register must create agent session")

	// 2. 心跳（agent_id 匹配，带 owner 自报）→ 全路径处理
	hb, _ := proto.Marshal(&agentv1.HeartbeatRequest{AgentId: "agent-1", OwnerInstanceId: "inst-9"})
	writeFrame(protocol.MsgHeartbeatRequest, 2, hb)
	time.Sleep(100 * time.Millisecond)

	// 3. 心跳 agent_id 失配 → validateHeartbeatAgentID 拒绝
	hbBad, _ := proto.Marshal(&agentv1.HeartbeatRequest{AgentId: "agent-2"})
	writeFrame(protocol.MsgHeartbeatRequest, 3, hbBad)
	time.Sleep(100 * time.Millisecond)

	_, still := store.Get("agent-1")
	assert.True(t, still, "mismatched heartbeat must not evict session")

	// 4. 断开 → serveConn 清理：RemoveSession + OnAgentDisconnected
	clientEnd.Close()
	deadline = time.Now().Add(2 * time.Second)
	var removed bool
	for time.Now().Before(deadline) {
		if _, ok := store.Get("agent-1"); !ok {
			removed = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.True(t, removed, "disconnect must remove session")
	assert.True(t, hooks.disconnected.Load(), "clusterHooks.OnAgentDisconnected must fire")
}

// 供生命周期测试使用的 hook（带断言状态）。
type lifecycleHook struct {
	disconnected atomic.Bool
}

func (h *lifecycleHook) OnAgentRegistered(context.Context, string, string, string) {}
func (h *lifecycleHook) OnAgentHeartbeat(context.Context, string)                  {}
func (h *lifecycleHook) OnAgentDisconnected(context.Context, string) {
	h.disconnected.Store(true)
}

// serveConn 补充分支：坏帧容错（unmarshal 失败不中断）、无效注册告警、
// 新连接替换旧会话后旧连接断开走"已被替换"跳过分支。
func TestServeConn_EdgeBranches(t *testing.T) {
	config := &TCPListenerConfig{Address: "127.0.0.1:0", Insecure: true}
	listener, err := NewTCPListener(config, NewAgentSessionStore(), nil, nil)
	require.NoError(t, err)
	listener.SetHandler(newTestControlService())

	// 子场景 A：空 AgentId 注册失败分支（独立连接，避免污染后续注册状态机）
	{
		se, ce := net.Pipe()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		go listener.serveConn(ctx, se)
		frame := protocol.NewMessageBody(protocol.MsgRegisterRequest, 3,
			mustProto(t, &agentv1.RegisterRequest{AgentId: "", GameId: "g"}))
		wrapped := make([]byte, 4+len(frame))
		wrapped[0] = byte(len(frame) >> 24)
		wrapped[1] = byte(len(frame) >> 16)
		wrapped[2] = byte(len(frame) >> 8)
		wrapped[3] = byte(len(frame))
		_, _ = ce.Write(wrapped)
		time.Sleep(100 * time.Millisecond)
		ce.Close()
		se.Close()
	}

	serverEnd, clientEnd := net.Pipe()
	defer clientEnd.Close()
	// 排水：mux 的响应帧写管道是同步的，无人读会卡住响应写
	go func() {
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

	writeFrame := func(msgID uint32, reqID uint32, body []byte) {
		frame := protocol.NewMessageBody(msgID, reqID, body)
		wrapped := make([]byte, 4+len(frame))
		wrapped[0] = byte(len(frame) >> 24)
		wrapped[1] = byte(len(frame) >> 16)
		wrapped[2] = byte(len(frame) >> 8)
		wrapped[3] = byte(len(frame))
		copy(wrapped[4:], frame)
		_, werr := clientEnd.Write(wrapped)
		require.NoError(t, werr)
	}
	waitSession := func(id string, want bool) {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if _, ok := listener.SessionStore().Get(id); ok == want {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	// 1. 非法注册（AgentId 为空）：proto 合法但校验失败 → 告警分支（353）
	writeFrame(protocol.MsgRegisterRequest, 3,
		mustProto(t, &agentv1.RegisterRequest{AgentId: "", GameId: "g"}))
	time.Sleep(80 * time.Millisecond)

	// 2. 正常注册 agent-1（session-old）
	writeFrame(protocol.MsgRegisterRequest, 4,
		mustProto(t, &agentv1.RegisterRequest{AgentId: "agent-1", GameId: "game-1", Env: "dev"}))
	waitSession("agent-1", true)
	_, regCheck := listener.SessionStore().Get("agent-1")
	require.True(t, regCheck, "edge: register must create session")

	// 3. 已注册后：坏 protobuf 体的 re-register / 心跳 → 各自的 unmarshal
	//    失败分支（255 / 291），连接不断
	writeFrame(protocol.MsgRegisterRequest, 5, []byte{0xff, 0xff, 0xff})
	writeFrame(protocol.MsgHeartbeatRequest, 6, []byte{0xff, 0xff, 0xff})
	time.Sleep(80 * time.Millisecond)
	_, still2 := listener.SessionStore().Get("agent-1")
	require.True(t, still2, "bad frames must not kill session")

	// 3b. 合法 protobuf 但函数非法（空函数 ID）→ handleRegisterRequest
	//     失败告警分支（353），会话保持
	writeFrame(protocol.MsgRegisterRequest, 7, mustProto(t, &agentv1.RegisterRequest{
		AgentId: "agent-1", GameId: "game-1", Env: "dev",
		Functions: []*agentv1.FunctionDescriptor{{Id: ""}},
	}))
	time.Sleep(80 * time.Millisecond)
	_, still3 := listener.SessionStore().Get("agent-1")
	require.True(t, still3, "failed re-register must keep session")

	// 4. 模拟新连接替换：同 agent 注入新 session → 旧连接断开后应走"已被替换"分支
	listener.SessionStore().Upsert(&AgentSession{
		AgentID: "agent-1", SessionID: "session-new",
	})

	clientEnd.Close()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := listener.SessionStore().Get("agent-1"); !ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	// session-new 仍在（旧连接清理不得误删新会话）
	_, still := listener.SessionStore().Get("agent-1")
	assert.True(t, still, "old connection cleanup must not evict newer session")
}

// tcpListenAddress nil 兜底。
func TestTcpListenAddress_NilConfig(t *testing.T) {
	addr := tcpListenAddress(nil)
	assert.NotEmpty(t, addr)
}

func mustProto(t *testing.T, m proto.Message) []byte {
	t.Helper()
	b, err := proto.Marshal(m)
	require.NoError(t, err)
	return b
}
