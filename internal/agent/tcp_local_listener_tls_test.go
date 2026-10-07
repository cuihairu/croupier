package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/devcert"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// 批次 E（能力矩阵 #5 本地网关 TLS 接线）：`tls:` 配置段经
// tcptr.BuildServerTLSConfig 构建服务端 *tls.Config，注入
// TCPLocalListenerConfig.TLSConfig。TLS 位于 TCP 之上、分帧之下——
// 握手成功的连接走与明文完全一致的 ProviderConnect/MuxConn 流程。

// startTLSListener 生成 CA/服务器证书并启动 TLS 监听（caFile 为空=单向 TLS，
// 非空=mTLS），返回监听器、CA 证书池与清理函数。
func startTLSListener(t *testing.T, caFile string) (*TCPLocalListener, *x509.CertPool, func()) {
	t.Helper()
	caDir := t.TempDir()
	certDir := t.TempDir()
	caCrt, caKey, err := devcert.EnsureDevCA(caDir)
	require.NoError(t, err)
	serverCrt, serverKey, err := devcert.EnsureServerCert(certDir, caCrt, caKey, []string{"127.0.0.1", "localhost"})
	require.NoError(t, err)

	tlsCfg, err := tcptr.BuildServerTLSConfig(serverCrt, serverKey, caFile, false)
	require.NoError(t, err)

	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{
		Address:   "127.0.0.1:0",
		TLSConfig: tlsCfg,
	}, nil, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = listener.Serve(ctx)
	}()

	pool := x509.NewCertPool()
	caPEM, err := os.ReadFile(caCrt)
	require.NoError(t, err)
	require.True(t, pool.AppendCertsFromPEM(caPEM))

	cleanup := func() {
		cancel()
		require.NoError(t, listener.Close())
		<-done
	}
	return listener, pool, cleanup
}

// tlsDial 以 TLS 拨入本地网关（clientCert 为空=不出示客户端证书）。
func tlsDial(t *testing.T, addr string, rootCAs *x509.CertPool, clientCrt, clientKey string) *tls.Conn {
	t.Helper()
	cfg := &tls.Config{
		RootCAs:    rootCAs,
		ServerName: "127.0.0.1",
		MinVersion: tls.VersionTLS12,
	}
	if clientCrt != "" {
		cert, err := tls.LoadX509KeyPair(clientCrt, clientKey)
		require.NoError(t, err)
		cfg.Certificates = []tls.Certificate{cert}
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", addr, cfg)
	require.NoError(t, err)
	return conn
}

// tlsProviderSession 走既有 Provider 握手把 TLS 连接升级为已注册会话。
func tlsProviderSession(t *testing.T, listener *TCPLocalListener, rootCAs *x509.CertPool, clientCrt, clientKey, serviceID string) {
	t.Helper()
	tlsConn := tlsDial(t, listener.Addr(), rootCAs, clientCrt, clientKey)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = tlsConn.Close() })

	provider := tcptr.NewMuxConn(tlsConn, nil, nil)
	go func() { _ = provider.Run(ctx) }()

	connectBody, err := proto.Marshal(&sdkv1.ProviderConnectRequest{
		ServiceId: serviceID,
		Version:   "1.0.0",
		Functions: []*sdkv1.ProviderFunctionDescriptor{{Id: "mail.send", Version: "1.0.0"}},
	})
	require.NoError(t, err)
	_, _, err = provider.Call(ctx, protocol.MsgProviderConnectRequest, connectBody)
	require.NoError(t, err)

	_, ok := listener.SessionStore().GetByServiceID(serviceID)
	require.True(t, ok, "TLS session must register like a plaintext one")
}

func TestTCPLocalListener_TLSHandshakeSessionFlow(t *testing.T) {
	listener, pool, cleanup := startTLSListener(t, "")
	defer cleanup()

	tlsConn := tlsDial(t, listener.Addr(), pool, "", "")
	state := tlsConn.ConnectionState()
	require.GreaterOrEqual(t, state.Version, uint16(tls.VersionTLS12))
	_ = tlsConn.Close()

	tlsProviderSession(t, listener, pool, "", "", "tls-provider-one-way")
}

func TestTCPLocalListener_TLSRejectsPlaintextAndSurvives(t *testing.T) {
	listener, pool, cleanup := startTLSListener(t, "")
	defer cleanup()

	// 明文探测：服务端握手失败 → 连接被丢弃，探针侧读到 EOF/重置。
	plain, err := net.Dial("tcp", listener.Addr())
	require.NoError(t, err)
	_, err = plain.Write([]byte("GET / HTTP/1.0\r\n\r\n"))
	require.NoError(t, err)
	_ = plain.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = plain.Read(make([]byte, 16))
	require.Error(t, err, "server must close non-TLS connections")
	_ = plain.Close()

	// Accept 循环不受单连接握手失败影响：TLS 客户端照常完成会话注册。
	tlsProviderSession(t, listener, pool, "", "", "tls-provider-after-plaintext")
}

func TestTCPLocalListener_MutualTLSRequiresClientCert(t *testing.T) {
	caDir := t.TempDir()
	clientDir := t.TempDir()
	caCrt, caKey, err := devcert.EnsureDevCA(caDir)
	require.NoError(t, err)
	clientCrt, clientKey, err := devcert.EnsureAgentCert(clientDir, caCrt, caKey, "sdk-provider")
	require.NoError(t, err)

	listener, pool, cleanup := startTLSListener(t, caCrt)
	defer cleanup()

	// 无客户端证书：握手失败（或 TLS1.3 下首次读写暴露告警），不得产生会话。
	conn, dialErr := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", listener.Addr(), &tls.Config{
		RootCAs: pool, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12,
	})
	if dialErr == nil {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, readErr := conn.Read(make([]byte, 16))
		require.Error(t, readErr, "server must reject mTLS clients without a certificate")
		_ = conn.Close()
	}
	require.Empty(t, listener.SessionStore().List(), "no session may exist from a rejected handshake")

	// 持 CA 签发的客户端证书：正常注册。
	tlsProviderSession(t, listener, pool, clientCrt, clientKey, "tls-provider-mtls")
}
