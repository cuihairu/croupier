// Provider 握手鉴权回归（CR-FIX-11）：requireProviderAuth 开启后，
// 无 token/错 token 的 ProviderConnectRequest 必须被拒（连接关闭、不注册
// 会话）；token 正确时注册成功且 authToken 不落会话元数据（保留键防泄漏）。
// 未开启强制时 token 完全忽略。两条连接路径（TCP 监听 handleConnect 与
// mux 路径 handleProviderConnect）都钉死。
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

// newAuthTestListener 起一个开了强制鉴权的本地监听，返回连接工具。
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

// providerSide 封装测试 Provider 侧连接。
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
	// authToken 是保留键：鉴权后必须丢弃，绝不入会话元数据（防泄漏到
	// SDK 分发页）；用户键原样保留。
	assert.NotContains(t, sess.Metadata, "authToken")
	assert.NotContains(t, sess.Metadata, agentlocalReservedAuthKeyForTest())
	assert.Equal(t, "s1", sess.Metadata["serverId"])
	// 会话存储里同样没有。
	if stored, ok := listener.SessionStore().GetBySessionID(sess.SessionID); ok {
		assert.NotContains(t, stored.Metadata, "authToken")
	}
}

func TestProviderAuth_ListenerIgnoresTokenWhenNotRequired(t *testing.T) {
	// 未开启强制：无 token（以及任意 token）不影响注册。
	_, connect := newAuthTestListener(t, false, "")
	p := connect(t)
	require.NoError(t, p.connectRequest(t, ""))
	p.waitConnected(t)
}

// agentlocalReservedAuthKeyForTest 返回保留键名，断言保留键清单含
// authToken（防清单回退）。
func agentlocalReservedAuthKeyForTest() string {
	const key = "authToken"
	if _, ok := agentlocal.ReservedMetadataKeys[key]; !ok {
		return "<authToken missing from ReservedMetadataKeys>"
	}
	return key
}

// mux 路径：LocalHandler.handleProviderConnect 同规则。
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

// fail-closed：require=true 且 token 为空（配置错误）→ 全部拒绝。
func TestProviderAuth_FailClosedOnEmptyConfiguredToken(t *testing.T) {
	err := checkProviderAuth(true, "", "anything")
	require.Error(t, err)

	err = checkProviderAuth(true, "secret", "")
	require.Error(t, err, "missing client token must be rejected")

	require.NoError(t, checkProviderAuth(false, "", ""))
	require.NoError(t, checkProviderAuth(false, "", "whatever"))
}
