package agent

import (
	"context"
	"net"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// providerCallDeadline: metadata timeout_ms clamping and default semantics.
func TestProviderCallDeadlineSemantics(t *testing.T) {
	h := NewLocalHandler(nil, "/tmp", "agent-1", nil)

	// default 15s (aligned with the Server dispatch layer, replacing the old
	// hardcoded 10s)
	assert.Equal(t, 15*time.Second, h.providerCallDeadline(nil))
	assert.Equal(t, 15*time.Second, h.providerCallDeadline(map[string]string{"timeoutMs": "garbage"}))

	// the request declaration is the authoritative budget: declared values
	// take effect (never tightened by the default)
	assert.Equal(t, 3*time.Second, h.providerCallDeadline(map[string]string{"timeoutMs": "3000"}))
	// below 1s is raised to 1s; above 60s is capped at 60s (synchronous channel
	// hard ceiling)
	assert.Equal(t, time.Second, h.providerCallDeadline(map[string]string{"timeoutMs": "50"}))
	assert.Equal(t, 60*time.Second, h.providerCallDeadline(map[string]string{"timeoutMs": "999999"}))

	// the configured default only applies to requests with no declaration; a
	// declared value may exceed the configured default
	h.SetProviderCallTimeout(2 * time.Second)
	assert.Equal(t, 2*time.Second, h.providerCallDeadline(nil))
	assert.Equal(t, 1500*time.Millisecond, h.providerCallDeadline(map[string]string{"timeoutMs": "1500"}))
	assert.Equal(t, 30*time.Second, h.providerCallDeadline(map[string]string{"timeoutMs": "30000"}))

	// invalid config falls back to the default; over the cap is truncated
	h.SetProviderCallTimeout(-1)
	assert.Equal(t, 15*time.Second, h.providerCallDeadline(nil))
	h.SetProviderCallTimeout(5 * time.Minute)
	assert.Equal(t, 60*time.Second, h.providerCallDeadline(nil))
}

// End to end: a slow provider that does not finish within the budget declared
// in metadata → the caller gets a timeout error.
func TestHandleInvokeRespectsMetadataTimeout(t *testing.T) {
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
			if msgID == protocol.MsgInvokeRequest {
				req := &sdkv1.InvokeRequest{}
				require.NoError(t, proto.Unmarshal(body, req))
				// slower than the declared 1200ms budget
				time.Sleep(2500 * time.Millisecond)
				return proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`{}`)})
			}
			return nil, nil
		}))
	go func() { _ = provider.Run(ctx) }()
	t.Cleanup(func() { _ = provider.Close() })

	connectBody, err := proto.Marshal(&sdkv1.ProviderConnectRequest{
		ServiceId: "slow-provider",
		Version:   "1.0.0",
		Functions: []*sdkv1.ProviderFunctionDescriptor{{Id: "slow.fn", Version: "1.0.0"}},
	})
	require.NoError(t, err)
	_, _, err = provider.Call(ctx, protocol.MsgProviderConnectRequest, connectBody)
	require.NoError(t, err)

	session, ok := listener.SessionStore().GetByServiceID("slow-provider")
	require.True(t, ok)
	require.NotNil(t, session.Conn())

	// production wiring shape: the session connect callback registers functions
	// into the local store (same as app.go SetOnConnect) so pickInstance can
	// route to the function.
	store := agentlocal.NewLocalStore()
	listener.SetOnConnect(func(sess *ProviderSession) {
		store.Register(sess.SessionID, sess.ServiceID, sess.Conn().RemoteAddr(), sess.Version, sess.Functions, nil)
	})
	// re-handshake triggers onConnect (the registration above happened before
	// SetOnConnect)
	conn2, err := net.Dial("tcp", listener.Addr())
	require.NoError(t, err)
	provider2 := tcptr.NewMuxConn(conn2, nil, transportcore.HandlerFunc(
		func(_ context.Context, msgID uint32, _ uint32, _ []byte) ([]byte, error) {
			if msgID == protocol.MsgInvokeRequest {
				time.Sleep(2500 * time.Millisecond)
				return proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`{}`)})
			}
			return nil, nil
		}))
	go func() { _ = provider2.Run(ctx) }()
	t.Cleanup(func() { _ = provider2.Close() })
	_, _, err = provider2.Call(ctx, protocol.MsgProviderConnectRequest, connectBody)
	require.NoError(t, err)

	h := NewLocalHandler(store, "/tmp", "agent-1", nil)
	h.SetProviderSessionStore(listener.SessionStore())

	// 1) a declared 1200ms budget with the provider sleeping 2.5s → must time
	// out within the budget
	invokeBody, err := proto.Marshal(&sdkv1.InvokeRequest{
		FunctionId: "slow.fn",
		Payload:    []byte(`{}`),
		Metadata:   map[string]string{"timeoutMs": "1200"},
	})
	require.NoError(t, err)

	start := time.Now()
	_, err = h.Handle(ctx, protocol.MsgInvokeRequest, 1, invokeBody)
	require.Error(t, err, "调用必须超时")
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 2300*time.Millisecond, "应在声明的 ~1.2s 预算内失败")
	assert.GreaterOrEqual(t, elapsed, 1100*time.Millisecond, "不应早于预算误伤")

	// 2) no declared budget → the default 15s applies, the provider's 2.5s
	// completes successfully
	invokeBody2, err := proto.Marshal(&sdkv1.InvokeRequest{
		FunctionId: "slow.fn",
		Payload:    []byte(`{}`),
	})
	require.NoError(t, err)
	resp, err := h.Handle(ctx, protocol.MsgInvokeRequest, 2, invokeBody2)
	require.NoError(t, err, "默认预算下慢 provider 应成功")
	assert.NotEmpty(t, resp)
}
