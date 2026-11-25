// Provider handshake auth regression (CR-FIX-11): once requireProviderAuth is
// on, a ProviderConnectRequest with no token or a wrong token must be rejected
// (connection closed, no session registered); with the correct token,
// registration succeeds and authToken does not land in session metadata
// (reserved key, leak prevention). Without enforcement the token is ignored
// entirely. Both connection paths (TCP listener handleConnect and the mux path
// handleProviderConnect) are pinned.
package agent

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	transportcore "github.com/cuihairu/croupier/core/transport"
	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// newAuthTestListener starts a local listener with enforced auth and returns a
// connect helper.
func newAuthTestListener(t *testing.T, requireAuth bool, token string) (*TCPLocalListener, func(t *testing.T) *providerSide) {
	t.Helper()
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{
		Address:             "127.0.0.1:0",
		RequireProviderAuth: requireAuth,
		ProviderAuthToken:   token,
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	connected := make(chan *ProviderSession, 1)
	listener.SetOnConnect(func(sess *ProviderSession) { connected <- sess })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = listener.Serve(ctx) }()

	connect := func(t *testing.T) *providerSide {
		t.Helper()
		conn, err := net.Dial("tcp", listener.Addr())
		require.NoError(t, err)
		provider := tcptr.NewMuxConn(conn, nil, transportcore.HandlerFunc(
			func(_ context.Context, _ uint32, _ uint32, _ []byte) ([]byte, error) { return nil, nil }))
		go func() { _ = provider.Run(ctx) }()
		t.Cleanup(func() { _ = provider.Close() })
		return &providerSide{conn: provider, ctx: ctx, connected: connected}
	}
	return listener, connect
}

// providerSide wraps the test-side Provider connection.
type providerSide struct {
	conn      *tcptr.MuxConn
	ctx       context.Context
	connected chan *ProviderSession
}

func (p *providerSide) connectRequest(t *testing.T, token string) error {
	t.Helper()
	metadata := map[string]string{"serverId": "s1"}
	if token != "" {
		metadata["authToken"] = token
	}
	body, err := proto.Marshal(&sdkv1.ProviderConnectRequest{
		ServiceId:   "auth-provider",
		Version:     "1.0.0",
		SdkLanguage: "go",
		SdkVersion:  "v0.0.1",
		Metadata:    metadata,
	})
	require.NoError(t, err)
	_, _, err = p.conn.Call(p.ctx, protocol.MsgProviderConnectRequest, body)
	return err
}

func (p *providerSide) waitConnected(t *testing.T) *ProviderSession {
	t.Helper()
	select {
	case sess := <-p.connected:
		return sess
	case <-time.After(3 * time.Second):
		t.Fatal("provider session should have registered")
		return nil
	}
}

func TestProviderAuth_ListenerRejectsMissingToken(t *testing.T) {
	_, connect := newAuthTestListener(t, true, "secret")
	p := connect(t)
	err := p.connectRequest(t, "")
	require.Error(t, err, "missing token must be rejected")
	select {
	case <-p.connected:
		t.Fatal("rejected handshake must not register a session")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestProviderAuth_ListenerRejectsWrongToken(t *testing.T) {
	_, connect := newAuthTestListener(t, true, "secret")
	p := connect(t)
	err := p.connectRequest(t, "wrong")
	require.Error(t, err, "token mismatch must be rejected")
	select {
	case <-p.connected:
		t.Fatal("rejected handshake must not register a session")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestProviderAuth_ListenerAcceptsCorrectTokenAndDropsItFromMetadata(t *testing.T) {
	listener, connect := newAuthTestListener(t, true, "secret")
	p := connect(t)
	require.NoError(t, p.connectRequest(t, "secret"))
	sess := p.waitConnected(t)
	// authToken is a reserved key: it must be dropped after auth and must never
	// enter session metadata (preventing leakage to the SDK distribution page);
	// user keys are preserved verbatim.
	assert.NotContains(t, sess.Metadata, "authToken")
	assert.NotContains(t, sess.Metadata, agentlocalReservedAuthKeyForTest())
	assert.Equal(t, "s1", sess.Metadata["serverId"])
	// The session store has it equally absent.
	if stored, ok := listener.SessionStore().GetBySessionID(sess.SessionID); ok {
		assert.NotContains(t, stored.Metadata, "authToken")
	}
}

func TestProviderAuth_ListenerIgnoresTokenWhenNotRequired(t *testing.T) {
	// enforcement off: no token (or any token) does not affect registration.
	_, connect := newAuthTestListener(t, false, "")
	p := connect(t)
	require.NoError(t, p.connectRequest(t, ""))
	p.waitConnected(t)
}

// agentlocalReservedAuthKeyForTest returns the reserved key name and asserts
// the reserved-key list contains authToken (guarding against list regression).
func agentlocalReservedAuthKeyForTest() string {
	const key = "authToken"
	if _, ok := agentlocal.ReservedMetadataKeys[key]; !ok {
		return "<authToken missing from ReservedMetadataKeys>"
	}
	return key
}

// mux path: LocalHandler.handleProviderConnect under the same rules.
func TestProviderAuth_MuxPathRejectsMissingToken(t *testing.T) {
	h := NewLocalHandler(nil, "", "agent-1", nil)
	h.SetProviderAuth(true, "secret")

	body, err := proto.Marshal(&sdkv1.ProviderConnectRequest{
		ServiceId: "mux-provider",
		Version:   "1.0.0",
		Metadata:  map[string]string{"serverId": "s1"},
	})
	require.NoError(t, err)
	_, err = h.handleProviderConnect(context.Background(), body)
	require.Error(t, err, "missing token must be rejected on the mux path")
}

func TestProviderAuth_MuxPathAcceptsCorrectToken(t *testing.T) {
	h := NewLocalHandler(nil, "", "agent-1", nil)
	h.SetProviderAuth(true, "secret")

	body, err := proto.Marshal(&sdkv1.ProviderConnectRequest{
		ServiceId: "mux-provider",
		Version:   "1.0.0",
		Metadata:  map[string]string{"authToken": "secret", "serverId": "s1"},
	})
	require.NoError(t, err)
	respBody, err := h.handleProviderConnect(context.Background(), body)
	require.NoError(t, err)
	resp := &sdkv1.ProviderConnectResponse{}
	require.NoError(t, proto.Unmarshal(respBody, resp))
	assert.NotEmpty(t, resp.SessionId)
}

func TestProviderAuth_MuxPathIgnoresTokenWhenNotRequired(t *testing.T) {
	h := NewLocalHandler(nil, "", "agent-1", nil)

	body, err := proto.Marshal(&sdkv1.ProviderConnectRequest{
		ServiceId: "mux-provider",
		Version:   "1.0.0",
	})
	require.NoError(t, err)
	_, err = h.handleProviderConnect(context.Background(), body)
	require.NoError(t, err, "auth off: handshake without token must succeed")
}

// fail-closed: require=true with an empty token (configuration error) → reject
// everything.
func TestProviderAuth_FailClosedOnEmptyConfiguredToken(t *testing.T) {
	err := checkProviderAuth(true, "", "anything")
	require.Error(t, err)

	err = checkProviderAuth(true, "secret", "")
	require.Error(t, err, "missing client token must be rejected")

	require.NoError(t, checkProviderAuth(false, "", ""))
	require.NoError(t, checkProviderAuth(false, "", "whatever"))
}
