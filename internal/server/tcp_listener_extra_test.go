package server

// tcp_listener.go 补测：AgentIdentityFromContext 的 nil ctx / 无身份 ctx /
// 身份往返、心跳全路径处理失败的告警降级（upstream HandleHeartbeat 错误不
// 打断心跳应答）、断连收割失败的告警降级（清理不得阻断会话移除）。
//
// 注：coverage_gapfill_j_test.go 的 TestServeConn_HeartbeatRequestErrorWarn
// 的注册帧缺 Env，注册即被拒，心跳到不了全路径——本文件用完整三元组注册
// 真正触达 :570-572。

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentIdentityFromContextNilAndRoundTrip(t *testing.T) {
	// nil ctx → nil（非控制面入口）
	require.Nil(t, AgentIdentityFromContext(nil))
	// 无身份 ctx → nil
	require.Nil(t, AgentIdentityFromContext(context.Background()))

	// 身份往返
	identity := &AgentIdentity{AgentID: "agent-1", GameID: "demo", Env: "prod"}
	got := AgentIdentityFromContext(withAgentIdentity(context.Background(), identity))
	require.NotNil(t, got)
	require.Equal(t, "agent-1", got.AgentID)
	require.Equal(t, "demo", got.GameID)
	require.Equal(t, "prod", got.Env)
}

// newPipeListener 起一条 net.Pipe 上的 serveConn，返回写帧、注册等待与
// 主动断开三个闭包。
func newPipeListener(t *testing.T, listener *TCPListener) (
	writeFrame func(uint32, uint32, []byte),
	waitRegistered func(string) bool,
	disconnect func(),
) {
	t.Helper()
	serverEnd, clientEnd := net.Pipe()
	t.Cleanup(func() { _ = clientEnd.Close() })
	go func() { // 排水：mux 响应帧同步写管道
		buf := make([]byte, 4096)
		for {
			if _, err := clientEnd.Read(buf); err != nil {
				return
			}
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go listener.serveConn(ctx, serverEnd)

	writeFrame = func(msgID, reqID uint32, body []byte) {
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
	waitRegistered = func(agentID string) bool {
		store := listener.SessionStore()
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if _, ok := store.Get(agentID); ok {
				return true
			}
			time.Sleep(10 * time.Millisecond)
		}
		return false
	}
	disconnect = func() { _ = clientEnd.Close() }
	return writeFrame, waitRegistered, disconnect
}

func TestServeConn_HeartbeatFullPathErrorDegrades(t *testing.T) {
	config := &TCPListenerConfig{Address: "127.0.0.1:0", Insecure: true}
	listener, err := NewTCPListener(config, NewAgentSessionStore(), nil, nil)
	require.NoError(t, err)

	// upstream 心跳恒失败 → 全路径失败仅告警，心跳仍应答（:570-572）
	svc := newTestControlService()
	svc.SetUpstreamHandler(&mockHandler{heartbeatErr: errors.New("upstream down")})
	listener.SetHandler(svc)

	writeFrame, waitRegistered, _ := newPipeListener(t, listener)

	regReq := mustProto(t, &agentv1.RegisterRequest{
		AgentId: "agent-hb", GameId: "demo", Env: "prod",
	})
	writeFrame(protocol.MsgRegisterRequest, 1, regReq)
	require.True(t, waitRegistered("agent-hb"), "complete register must create session")

	hb := mustProto(t, &agentv1.HeartbeatRequest{AgentId: "agent-hb"})

	writeFrame(protocol.MsgHeartbeatRequest, 2, hb)
	time.Sleep(120 * time.Millisecond)

	_, still := listener.SessionStore().Get("agent-hb")
	assert.True(t, still, "heartbeat full-path failure must not evict session")
}

func TestServeConn_DisconnectReaperErrorDegrades(t *testing.T) {
	config := &TCPListenerConfig{Address: "127.0.0.1:0", Insecure: true}
	listener, err := NewTCPListener(config, NewAgentSessionStore(), nil, nil)
	require.NoError(t, err)

	// 断连收割恒失败 → 仅告警，不得阻断会话移除（:263-267）
	var reaped atomic.Int32
	listener.SetAgentDisconnectReaper(func(context.Context, string) error {
		reaped.Add(1)
		return errors.New("task db down")
	})

	writeFrame, waitRegistered, disconnect := newPipeListener(t, listener)
	regReq := mustProto(t, &agentv1.RegisterRequest{
		AgentId: "agent-reap", GameId: "demo", Env: "prod",
	})
	writeFrame(protocol.MsgRegisterRequest, 1, regReq)
	require.True(t, waitRegistered("agent-reap"))

	// 主动断开 → 会话移除 + 收割回调触发（收割失败仅告警）
	disconnect()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if reaped.Load() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.Positive(t, reaped.Load(), "disconnect reaper must fire on session removal")
	_, still := listener.SessionStore().Get("agent-reap")
	assert.False(t, still, "reaper failure must not block session removal")
}
