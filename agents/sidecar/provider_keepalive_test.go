package sidecar

import (
	"context"
	"net"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// newKeepaliveTestSession 用 net.Pipe 建一条真实 MuxConn 会话入池（provider
// 端 handler 可控），语义与生产同构：Call/Close/IsClosed 全走 core transport。
func newKeepaliveTestSession(t *testing.T, serviceID string, handler func(ctx context.Context, msgID uint32, body []byte) ([]byte, error)) (*ProviderSessionStore, *ProviderSession) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	agentSide, peerSide := net.Pipe()
	peer := tcptr.NewMuxConn(peerSide, nil, transportcore.HandlerFunc(
		func(c context.Context, msgID uint32, _ uint32, body []byte) ([]byte, error) {
			return handler(c, msgID, body)
		}))
	go func() { _ = peer.Run(ctx) }()
	t.Cleanup(func() { _ = peer.Close() })

	conn := tcptr.NewMuxConn(agentSide, nil, nil)
	t.Cleanup(func() { _ = conn.Close() })

	store := NewProviderSessionStore()
	sess := NewProviderSession(conn)
	sess.SessionID = serviceID + "-sess"
	sess.ServiceID = serviceID
	require.NoError(t, store.Add(sess))
	return store, sess
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
	store, session := newKeepaliveTestSession(t, "healthy-provider",
		func(_ context.Context, msgID uint32, _ []byte) ([]byte, error) {
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
	// provider 端对心跳永不响应（模拟 SDK 事件循环卡死；conn Run 退出时放行）
	store, session := newKeepaliveTestSession(t, "dead-provider",
		func(ctx context.Context, _ uint32, _ []byte) ([]byte, error) {
			<-ctx.Done()
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
	store, session := newKeepaliveTestSession(t, "closing-provider",
		func(context.Context, uint32, []byte) ([]byte, error) { return nil, nil })

	// 先关掉连接，再跑探测：走 conn.IsClosed() 分支直接摘除
	require.NoError(t, session.Close())

	k := NewProviderKeepalive(store, time.Hour, nil)
	k.probeOnce(context.Background())

	_, ok := store.GetBySessionID(session.SessionID)
	assert.False(t, ok, "closed provider must be removed")
}

// proto3 string 字段含无效 UTF-8 时 Marshal 必败。探测帧的 SessionID 来自
// ProviderSessionStore（不经 proto 反序列化闸门直填），可以构造出编码必败
// 的会话——probeOnce 必须跳过它而不是误摘除。
func TestProviderKeepaliveProbeOnce_MarshalFailureSkipsWithoutRemoval(t *testing.T) {
	store := NewProviderSessionStore()

	agentSide, peerSide := net.Pipe()
	defer func() { _ = peerSide.Close() }()
	conn := tcptr.NewMuxConn(agentSide, nil, nil)
	defer func() { _ = conn.Close() }()

	sess := NewProviderSession(conn)
	sess.SessionID = "\xff\xfe" // 无效 UTF-8：心跳探测帧编码必败
	sess.ServiceID = "svc-bad-utf8"
	require.NoError(t, store.Add(sess))

	k := NewProviderKeepalive(store, time.Hour, nil)
	k.probeOnce(context.Background())

	_, ok := store.GetBySessionID("\xff\xfe")
	assert.True(t, ok, "marshal-failed session must be skipped, not removed")
}
