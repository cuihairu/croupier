package agent

import (
	"context"
	"net"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	"github.com/cuihairu/croupier/internal/platform/agentlocal"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// ARCH-016: callProvider must reject new work routed to a draining session
// (停止新路由), instead of falling back to the same provider's address.
func TestCallProviderRejectsDrainingSessionV9(t *testing.T) {
	sessStore, session := newKeepaliveTestProvider(t, "svc-drain", func(msgID uint32, body []byte) ([]byte, error) {
		if msgID != protocol.MsgInvokeRequest {
			return nil, nil
		}
		return proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`{"r":"ok"}`)})
	})
	require.NotNil(t, session)

	store := agentlocal.NewLocalStore()
	store.Register("p-drain", "svc-drain", "127.0.0.1:1", "1.0.0",
		[]*sdkv1.ProviderFunctionDescriptor{{Id: "f.ping", Version: "1.0.0"}}, nil)

	h := NewLocalHandler(store, "/tmp", "agent-1", nil)
	h.SetProviderSessionStore(sessStore)

	req := &sdkv1.InvokeRequest{
		FunctionId: "f.ping",
		Payload:    []byte(`{}`),
		Metadata:   map[string]string{"serviceId": "svc-drain"},
	}
	data, err := proto.Marshal(req)
	require.NoError(t, err)

	// Before drain: routes over the session.
	out, err := h.handleInvoke(context.Background(), data)
	require.NoError(t, err)
	resp := &sdkv1.InvokeResponse{}
	require.NoError(t, proto.Unmarshal(out, resp))
	assert.Equal(t, `{"r":"ok"}`, string(resp.GetPayload()))

	// After drain: new calls are rejected, not routed to the going-away provider.
	session.StartDraining()
	_, err = h.handleInvoke(context.Background(), data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "draining")
}

// ARCH-016: handleDrain must stop new routing and wait for in-flight calls
// before ACKing (等待在途 / 执行结束条件). The ACK arrives only after the
// in-flight invoke completes.
func TestHandleDrainWaitsForInflightCallV9(t *testing.T) {
	invokeArrived := make(chan struct{})
	release := make(chan struct{})

	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{Address: "127.0.0.1:0"}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = listener.Serve(ctx) }()

	conn, err := net.Dial("tcp", listener.Addr())
	require.NoError(t, err)
	provider := tcptr.NewMuxConn(conn, nil, transportcore.HandlerFunc(
		func(_ context.Context, msgID uint32, _ uint32, _ []byte) ([]byte, error) {
			if msgID != protocol.MsgInvokeRequest {
				return nil, nil
			}
			close(invokeArrived)
			<-release
			return proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`{"r":"done"}`)})
		}))
	go func() { _ = provider.Run(ctx) }()
	t.Cleanup(func() { _ = provider.Close() })

	connectBody, err := proto.Marshal(&sdkv1.ProviderConnectRequest{
		ServiceId: "svc-drain-wait",
		Version:   "1.0.0",
		Functions: []*sdkv1.ProviderFunctionDescriptor{{Id: "f.ping", Version: "1.0.0"}},
	})
	require.NoError(t, err)
	_, _, err = provider.Call(ctx, protocol.MsgProviderConnectRequest, connectBody)
	require.NoError(t, err)
	sess, ok := listener.SessionStore().GetByServiceID("svc-drain-wait")
	require.True(t, ok)

	store := agentlocal.NewLocalStore()
	store.Register("p-w", "svc-drain-wait", "127.0.0.1:1", "1.0.0",
		[]*sdkv1.ProviderFunctionDescriptor{{Id: "f.ping", Version: "1.0.0"}}, nil)
	h := NewLocalHandler(store, "/tmp", "agent-1", nil)
	h.SetProviderSessionStore(listener.SessionStore())

	// Start an in-flight invoke (blocks in the provider until release).
	invokeData, err := proto.Marshal(&sdkv1.InvokeRequest{
		FunctionId: "f.ping",
		Payload:    []byte(`{}`),
		Metadata:   map[string]string{"serviceId": "svc-drain-wait"},
	})
	require.NoError(t, err)
	invokeDone := make(chan error, 1)
	go func() {
		_, err := h.handleInvoke(context.Background(), invokeData)
		invokeDone <- err
	}()
	<-invokeArrived
	require.Equal(t, 1, sess.InflightCount(), "invoke is in flight")

	// Provider requests a drain; the ACK must be withheld until in-flight ends.
	drainBody, err := proto.Marshal(&sdkv1.ProviderDrainRequest{
		SessionId:    sess.SessionID,
		Reason:       "deploy",
		RetryAfterMs: 5000,
	})
	require.NoError(t, err)
	ack := make(chan uint32, 1)
	go func() {
		msgID, _, err := provider.Call(ctx, protocol.MsgProviderDrainRequest, drainBody)
		if err != nil {
			ack <- 0
			return
		}
		ack <- msgID
	}()

	// Wait for the agent to mark the session draining (new routing stopped).
	require.Eventually(t, sess.IsDraining, 2*time.Second, 10*time.Millisecond)

	// No ACK while the call is still in flight.
	select {
	case id := <-ack:
		t.Fatalf("drain ACK arrived before in-flight call finished (msgID=%d)", id)
	case <-time.After(150 * time.Millisecond):
	}

	// Release the invoke: both the invoke and the drain complete.
	close(release)
	require.NoError(t, <-invokeDone)
	select {
	case id := <-ack:
		assert.Equal(t, uint32(protocol.MsgProviderDrainResponse), id, "drain ACK after in-flight call finished")
	case <-time.After(2 * time.Second):
		t.Fatal("drain ACK did not arrive after in-flight call finished")
	}
}

// ARCH-016: handleDrain times out when in-flight calls do not finish within
// the provider's RetryAfterMs window (超时退出语义) — it ACKs anyway instead
// of blocking the session handler forever.
func TestHandleDrainTimesOutOnStuckCallV9(t *testing.T) {
	invokeArrived := make(chan struct{})

	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{Address: "127.0.0.1:0"}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = listener.Serve(ctx) }()

	conn, err := net.Dial("tcp", listener.Addr())
	require.NoError(t, err)
	provider := tcptr.NewMuxConn(conn, nil, transportcore.HandlerFunc(
		func(_ context.Context, msgID uint32, _ uint32, _ []byte) ([]byte, error) {
			if msgID == protocol.MsgInvokeRequest {
				close(invokeArrived)
				<-ctx.Done() // never returns; the call is stuck
			}
			return nil, nil
		}))
	go func() { _ = provider.Run(ctx) }()
	t.Cleanup(func() { _ = provider.Close() })

	connectBody, err := proto.Marshal(&sdkv1.ProviderConnectRequest{
		ServiceId: "svc-drain-stuck",
		Version:   "1.0.0",
		Functions: []*sdkv1.ProviderFunctionDescriptor{{Id: "f.ping", Version: "1.0.0"}},
	})
	require.NoError(t, err)
	_, _, err = provider.Call(ctx, protocol.MsgProviderConnectRequest, connectBody)
	require.NoError(t, err)
	sess, ok := listener.SessionStore().GetByServiceID("svc-drain-stuck")
	require.True(t, ok)

	store := agentlocal.NewLocalStore()
	store.Register("p-s", "svc-drain-stuck", "127.0.0.1:1", "1.0.0",
		[]*sdkv1.ProviderFunctionDescriptor{{Id: "f.ping", Version: "1.0.0"}}, nil)
	h := NewLocalHandler(store, "/tmp", "agent-1", nil)
	h.SetProviderSessionStore(listener.SessionStore())

	invokeData, err := proto.Marshal(&sdkv1.InvokeRequest{
		FunctionId: "f.ping",
		Payload:    []byte(`{}`),
		Metadata:   map[string]string{"serviceId": "svc-drain-stuck"},
	})
	require.NoError(t, err)
	go func() { _, _ = h.handleInvoke(context.Background(), invokeData) }()
	<-invokeArrived
	require.Equal(t, 1, sess.InflightCount())

	// Drain with a 50ms window; the stuck call does not finish, so the ACK
	// must arrive after the timeout (bounded, not blocked forever).
	drainBody, err := proto.Marshal(&sdkv1.ProviderDrainRequest{
		SessionId:    sess.SessionID,
		Reason:       "deploy",
		RetryAfterMs: 50,
	})
	require.NoError(t, err)
	start := time.Now()
	msgID, _, err := provider.Call(ctx, protocol.MsgProviderDrainRequest, drainBody)
	require.NoError(t, err)
	elapsed := time.Since(start)
	assert.Equal(t, uint32(protocol.MsgProviderDrainResponse), msgID)
	assert.Less(t, elapsed, 5*time.Second, "drain must time out rather than block forever")
	assert.True(t, sess.IsDraining())
}
