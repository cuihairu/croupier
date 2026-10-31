package cluster

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/transport/tcp"
	"github.com/cuihairu/croupier/internal/devcert"
	internaltcp "github.com/cuihairu/croupier/internal/transport/tcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadLeaf(t *testing.T, path string) *x509.Certificate {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	block, _ := pem.Decode(data)
	require.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	return cert
}

// TestVerifyInterconnectPeer_RejectsNoCertificate：mTLS 链路上无证书被拒。
func TestVerifyInterconnectPeer_RejectsNoCertificate(t *testing.T) {
	err := VerifyInterconnectPeer(tls.ConnectionState{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no peer certificate")
}

// TestVerifyInterconnectPeer_RejectsAgentIdentity：仅 ClientAuth 的 Agent
// 证书不是 server 角色，被拒（ARCH-003「Agent 身份拒绝」）。
func TestVerifyInterconnectPeer_RejectsAgentIdentity(t *testing.T) {
	dir := t.TempDir()
	caCrt, caKey, err := devcert.EnsureDevCA(dir)
	require.NoError(t, err)
	agentCrt, _, err := devcert.EnsureAgentCert(filepath.Join(dir, "agent"), caCrt, caKey, "agent-x")
	require.NoError(t, err)
	cert := loadLeaf(t, agentCrt)
	err = VerifyInterconnectPeer(tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a server role")
}

// TestVerifyInterconnectPeer_AcceptsServerIdentity：双 EKU server 角色 + 非空
// CommonName 被接受（互联对端身份）。
func TestVerifyInterconnectPeer_AcceptsServerIdentity(t *testing.T) {
	dir := t.TempDir()
	caCrt, caKey, err := devcert.EnsureDevCA(dir)
	require.NoError(t, err)
	icCrt, _, err := devcert.EnsureInterconnectCert(filepath.Join(dir, "ic"), caCrt, caKey, "server-b", []string{"127.0.0.1"})
	require.NoError(t, err)
	cert := loadLeaf(t, icCrt)
	require.NoError(t, VerifyInterconnectPeer(tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}))
}

// TestServeForwardHandler_RejectsUnhandshaked：未完成 hello 的 forward 被拒。
func TestServeForwardHandler_RejectsUnhandshaked(t *testing.T) {
	forward := ServeForwardHandler(7, func(context.Context, *ForwardedInvoke) (*ForwardedResult, error) {
		return &ForwardedResult{OK: true}, nil
	})
	body, _ := json.Marshal(&ForwardedInvoke{Forwarded: true, CallerEpoch: 7})
	resp := forward(context.Background(), body)
	var result ForwardedResult
	require.NoError(t, json.Unmarshal(resp, &result))
	assert.False(t, result.OK)
	assert.Contains(t, result.Error, "not handshaked")
}

// TestServeForwardHandler_AcceptsHandshaked：完成 hello 后合法 forward 执行，
// 保留 epoch fencing（stale epoch 仍拒绝）。
func TestServeForwardHandler_AcceptsHandshaked(t *testing.T) {
	forward := ServeForwardHandler(7, func(context.Context, *ForwardedInvoke) (*ForwardedResult, error) {
		return &ForwardedResult{OK: true, Payload: []byte(`{"ok":true}`)}, nil
	})
	session := internaltcp.NewConnectionSession()
	ctx := internaltcp.WithConnectionSession(context.Background(), session)
	session.Set(internaltcp.ConnectionHandshakeKey, true)

	body, _ := json.Marshal(&ForwardedInvoke{Forwarded: true, CallerEpoch: 7})
	resp := forward(ctx, body)
	var result ForwardedResult
	require.NoError(t, json.Unmarshal(resp, &result))
	assert.True(t, result.OK)

	body, _ = json.Marshal(&ForwardedInvoke{Forwarded: true, CallerEpoch: 999})
	resp = forward(ctx, body)
	require.NoError(t, json.Unmarshal(resp, &result))
	assert.True(t, result.NotOwner)
	assert.Contains(t, result.Error, "stale epoch")
}

// TestServeHelloHandler_MarksHandshake：合法 hello（role=server）标记握手。
func TestServeHelloHandler_MarksHandshake(t *testing.T) {
	hello := ServeHelloHandler(PeerInfo{InstanceID: "owner"}, 7)
	session := internaltcp.NewConnectionSession()
	ctx := internaltcp.WithConnectionSession(context.Background(), session)
	resp := hello(ctx, []byte(`{"role":"server"}`))
	var hr helloResponse
	require.NoError(t, json.Unmarshal(resp, &hr))
	assert.Equal(t, "owner", hr.InstanceID)
	assert.True(t, session.HandshakeDone())
}

// TestServeHelloHandler_RejectsBadRole：非 server 角色不标记握手。
func TestServeHelloHandler_RejectsBadRole(t *testing.T) {
	hello := ServeHelloHandler(PeerInfo{InstanceID: "owner"}, 7)
	session := internaltcp.NewConnectionSession()
	ctx := internaltcp.WithConnectionSession(context.Background(), session)
	resp := hello(ctx, []byte(`{"role":"agent"}`))
	var hr helloResponse
	require.NoError(t, json.Unmarshal(resp, &hr))
	assert.Empty(t, hr.InstanceID)
	assert.False(t, session.HandshakeDone())
}

func newMTLSServer(t *testing.T, caFile, certFile, keyFile string) *internaltcp.Server {
	t.Helper()
	cfg := &tcp.Config{
		Address:            "127.0.0.1:0",
		RecvTimeout:        5 * time.Second,
		SendTimeout:        5 * time.Second,
		CertFile:           certFile,
		KeyFile:            keyFile,
		CAFile:             caFile,
		VerifyConnection:   VerifyInterconnectPeer,
		InsecureSkipVerify: false,
	}
	srv, err := internaltcp.NewServer(cfg, &interconnectServeHandler{epoch: 7, rawResult: []byte(`{"ok":true}`)})
	require.NoError(t, err)
	go func() { _ = srv.Serve(context.Background()) }()
	t.Cleanup(func() { _ = srv.Close() })
	return srv
}

func dialConfig(certFile, keyFile, caFile string) *tcp.Config {
	return &tcp.Config{
		RecvTimeout:        5 * time.Second,
		SendTimeout:        5 * time.Second,
		ConnectTimeout:     3 * time.Second,
		CertFile:           certFile,
		KeyFile:            keyFile,
		CAFile:             caFile,
		VerifyConnection:   VerifyInterconnectPeer,
		InsecureSkipVerify: false,
	}
}

// TestInterconnect_MTLSFullChain 双实例 mTLS 端到端：合法 server 证书
// 握手成功、forward 成功且保留 epoch fencing；无证书/Agent 证书/错 CA
// 握手失败。
func TestInterconnect_MTLSFullChain(t *testing.T) {
	dir := t.TempDir()
	caCrt, caKey, err := devcert.EnsureDevCA(dir)
	require.NoError(t, err)
	serverCrt, serverKey, err := devcert.EnsureInterconnectCert(filepath.Join(dir, "owner"), caCrt, caKey, "owner-inst", []string{"127.0.0.1"})
	require.NoError(t, err)

	owner := PeerInfo{InstanceID: "owner-inst", Epoch: 7}
	srv := newMTLSServer(t, caCrt, serverCrt, serverKey)
	owner.AdvertiseAddr = srv.Addr()

	// 合法 server 证书拨号 → 握手成功 → forward OK。
	caller := NewMeshInterconnect(PeerInfo{InstanceID: "caller-inst"}, &staticOwnerResolver{owner: &owner}, nil)
	caller.dial = NewTCPDialer(dialConfig(serverCrt, serverKey, caCrt))
	res, err := caller.Forward(context.Background(), "agent-on-owner", &ForwardedInvoke{FunctionID: "demo.fn"})
	require.NoError(t, err)
	assert.True(t, res.OK)
	assert.Equal(t, `{"ok":true}`, string(res.Payload))

	// start_task / cancel_task：mTLS 下仍保留 epoch fencing 与 kind 路由。
	start, err := caller.Forward(context.Background(), "agent-on-owner", &ForwardedInvoke{
		Kind: ForwardKindStartTask, FunctionID: "demo.fn", Metadata: map[string]string{"taskId": "t-1"},
	})
	require.NoError(t, err)
	assert.True(t, start.OK)
	cancel, err := caller.Forward(context.Background(), "agent-on-owner", &ForwardedInvoke{
		Kind: ForwardKindCancel, TaskID: "t-1",
	})
	require.NoError(t, err)
	assert.True(t, cancel.OK)

	// 无证书拨号：TLS 握手失败（no client cert）→ Forward 错误。
	noCert := NewMeshInterconnect(PeerInfo{InstanceID: "caller-inst"}, &staticOwnerResolver{owner: &owner}, nil)
	noCert.dial = NewTCPDialer(dialConfig("", "", caCrt))
	_, err = noCert.Forward(context.Background(), "agent-on-owner", &ForwardedInvoke{FunctionID: "demo.fn"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dial owner")

	// Agent 证书（仅 ClientAuth）→ 被拒（not a server role）。
	agentDir := filepath.Join(dir, "agent")
	agentCrt, agentKey, err := devcert.EnsureAgentCert(agentDir, caCrt, caKey, "agent-x")
	require.NoError(t, err)
	agent := NewMeshInterconnect(PeerInfo{InstanceID: "caller-inst"}, &staticOwnerResolver{owner: &owner}, nil)
	agent.dial = NewTCPDialer(dialConfig(agentCrt, agentKey, caCrt))
	_, err = agent.Forward(context.Background(), "agent-on-owner", &ForwardedInvoke{FunctionID: "demo.fn"})
	require.Error(t, err)

	// 错 CA 拨号：证书不被信任 → 握手失败。
	wrongDir := t.TempDir()
	wrongCA, wrongKey, err := devcert.EnsureDevCA(wrongDir)
	require.NoError(t, err)
	wrongCrt, wrongKey2, err := devcert.EnsureInterconnectCert(filepath.Join(wrongDir, "w"), wrongCA, wrongKey, "wrong-inst", []string{"127.0.0.1"})
	require.NoError(t, err)
	wrong := NewMeshInterconnect(PeerInfo{InstanceID: "caller-inst"}, &staticOwnerResolver{owner: &owner}, nil)
	wrong.dial = NewTCPDialer(dialConfig(wrongCrt, wrongKey2, caCrt))
	_, err = wrong.Forward(context.Background(), "agent-on-owner", &ForwardedInvoke{FunctionID: "demo.fn"})
	require.Error(t, err)
}
