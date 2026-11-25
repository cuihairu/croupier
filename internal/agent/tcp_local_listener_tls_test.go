package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"testing"
	"time"

	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	"github.com/cuihairu/croupier/internal/devcert"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// Batch E (capability matrix #5, local gateway TLS wiring): the `tls:` config
// section builds a server-side *tls.Config via tcptr.BuildServerTLSConfig and
// injects it into TCPLocalListenerConfig.TLSConfig. TLS sits above TCP and
// below framing — a connection whose handshake succeeds runs exactly the same
// ProviderConnect/MuxConn flow as plaintext.

// startTLSListener generates CA/server certificates and starts a TLS listener
// (caFile empty = one-way TLS, non-empty = mTLS), returning the listener, the
// CA cert pool, and a cleanup function.
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

// tlsDial dials in to the local gateway over TLS (empty clientCrt = no client
// certificate presented).
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

// tlsProviderSession upgrades a TLS connection into a registered session
// through the existing Provider handshake.
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

	// Plaintext probe: the server handshake fails → the connection is dropped,
	// and the probe side reads EOF/reset.
	plain, err := net.Dial("tcp", listener.Addr())
	require.NoError(t, err)
	_, err = plain.Write([]byte("GET / HTTP/1.0\r\n\r\n"))
	require.NoError(t, err)
	_ = plain.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = plain.Read(make([]byte, 16))
	require.Error(t, err, "server must close non-TLS connections")
	_ = plain.Close()

	// The Accept loop is unaffected by a single connection's handshake failure:
	// a TLS client still completes session registration.
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

	// No client certificate: the handshake fails (or, under TLS 1.3, the first
	// read/write surfaces the alert); no session may be produced.
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

	// Holding a CA-signed client certificate: registers normally.
	tlsProviderSession(t, listener, pool, clientCrt, clientKey, "tls-provider-mtls")
}
