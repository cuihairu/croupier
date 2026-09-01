package agent

import (
	"context"
	"net"
	"testing"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// newKeepaliveTestProvider 建立一条真实 TCP 会话（provider 端 handler 可控），
// 返回 (store, session)。provider 端通过 agent 侧握手入池，语义与生产一致。
func newKeepaliveTestProvider(t *testing.T, serviceID string, handler func(msgID uint32, body []byte) ([]byte, error)) (*ProviderSessionStore, *ProviderSession) {
	t.Helper()
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{Address: "127.0.0.1:0"}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = listener.Serve(ctx) }()

	conn, err := net.Dial("tcp", listener.Addr())
	require.NoError(t, err)
	provider := tcptr.NewMuxConn(conn, nil, transportcore.HandlerFunc(
		func(_ context.Context, msgID uint32, _ uint32, body []byte) ([]byte, error) {
			return handler(msgID, body)
		}))
	go func() { _ = provider.Run(ctx) }()
	t.Cleanup(func() { _ = provider.Close() })

	connectBody, err := proto.Marshal(&sdkv1.ProviderConnectRequest{
		ServiceId: serviceID,
		Version:   "1.0.0",
		Functions: []*sdkv1.ProviderFunctionDescriptor{{Id: "f.ping", Version: "1.0.0"}},
	})
	require.NoError(t, err)
	_, _, err = provider.Call(ctx, protocol.MsgProviderConnectRequest, connectBody)
	require.NoError(t, err)

	session, ok := listener.SessionStore().GetByServiceID(serviceID)
	require.True(t, ok)
	return listener.SessionStore(), session
}

func TestNewProviderKeepaliveDefaults(t *testing.T) {
	store := NewProviderSessionStore()
	k := NewProviderKeepalive(store, 0, nil)
	require.NotNil(t, k)
	assert.Equal(t, 5*time.Second, k.interval)
	assert.NotNil(t, k.logger)

	k2 := NewProviderKeepalive(store, 100*time.Millisecond, nil)
	assert.Equal(t, 100*time.Millisecond, k2.interval)
}

func TestProviderKeepaliveRunStopsOnContextCancel(t *testing.T) {
	store := NewProviderSessionStore()
	k := NewProviderKeepalive(store, 10*time.Millisecond, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		k.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}

func TestProviderKeepaliveHealthyProviderStays(t *testing.T) {
	store, session := newKeepaliveTestProvider(t, "healthy-provider",
		func(msgID uint32, _ []byte) ([]byte, error) {
			if msgID == protocol.MsgProviderHeartbeatRequest {
				return proto.Marshal(&sdkv1.ProviderHeartbeatResponse{})
			}
			return nil, nil
		})

	k := NewProviderKeepalive(store, 30*time.Millisecond, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go k.Run(ctx)

	// 跑过多个探测周期后健康 provider 应仍在池中
	require.Eventually(t, func() bool { return true }, 150*time.Millisecond, 50*time.Millisecond)
	_, ok := store.GetBySessionID(session.SessionID)
	assert.True(t, ok, "healthy provider must stay in the pool")
}

func TestProviderKeepaliveDeadProviderRemoved(t *testing.T) {
	// provider 端对心跳永不响应（模拟 SDK 事件循环卡死）
	store, session := newKeepaliveTestProvider(t, "dead-provider",
		func(msgID uint32, _ []byte) ([]byte, error) {
			if msgID == protocol.MsgProviderHeartbeatRequest {
				select {} // 永不返回，靠调用方超时
			}
			return nil, nil
		})

	k := NewProviderKeepalive(store, 50*time.Millisecond, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go k.Run(ctx)

	require.Eventually(t, func() bool {
		_, ok := store.GetBySessionID(session.SessionID)
		return !ok
	}, 3*time.Second, 50*time.Millisecond, "dead provider must be removed from the pool")
	assert.True(t, session.Conn().IsClosed(), "dead provider conn must be closed")
}

func TestProviderKeepaliveClosedConnRemovedWithoutProbe(t *testing.T) {
	store, session := newKeepaliveTestProvider(t, "closing-provider",
		func(uint32, []byte) ([]byte, error) { return nil, nil })

	// 先关掉连接，再跑探测：走 conn.IsClosed() 分支直接摘除
	require.NoError(t, session.Close())

	k := NewProviderKeepalive(store, time.Hour, nil)
	k.probeOnce(context.Background())

	_, ok := store.GetBySessionID(session.SessionID)
	assert.False(t, ok, "closed provider must be removed")
}

func TestLocalHandlerSetProviderSessionStoreAndExpectedGameEnv(t *testing.T) {
	store := agentlocal.NewLocalStore()
	h := NewLocalHandler(store, "/tmp", "agent-1", nil)

	sessStore := NewProviderSessionStore()
	h.SetProviderSessionStore(sessStore)
	h.mu.RLock()
	assert.Same(t, sessStore, h.providerSessions)
	h.mu.RUnlock()

	h.SetExpectedGameEnv("game-a", "prod")
	h.mu.RLock()
	assert.Equal(t, "game-a", h.expectedGameID)
	assert.Equal(t, "prod", h.expectedEnv)
	h.mu.RUnlock()
}
