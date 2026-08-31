package tcp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// ---------------------------------------------------------------------------
// framing
// ---------------------------------------------------------------------------

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWriteFrame_WriteErrors(t *testing.T) {
	wantErr := &net.AddrError{Err: "boom", Addr: "x"}
	if err := writeFrame(failingWriter{err: wantErr}, []byte("payload")); err == nil {
		t.Fatal("expected header write error")
	}

	// Header write succeeds, payload write fails.
	w := &halfFailingWriter{err: wantErr}
	if err := writeFrame(w, []byte("payload")); err == nil {
		t.Fatal("expected payload write error")
	}
}

type halfFailingWriter struct{ err error }

func (w *halfFailingWriter) Write(p []byte) (int, error) {
	if len(p) == frameHeaderBytes {
		return len(p), nil
	}
	return 0, w.err
}

func TestReadFrame_EmptyFrame(t *testing.T) {
	header := make([]byte, frameHeaderBytes)
	payload, err := readFrame(io.Reader(bytesReader(header)))
	if err != nil {
		t.Fatalf("readFrame empty: %v", err)
	}
	if len(payload) != 0 {
		t.Fatalf("payload = %v, want empty", payload)
	}
}

type byteSliceReader struct {
	data []byte
	pos  int
}

func bytesReader(b []byte) *byteSliceReader { return &byteSliceReader{data: b} }

func (r *byteSliceReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

func TestNewClient_NilConfig(t *testing.T) {
	_, err := NewClient(nil)
	if err == nil {
		t.Fatal("expected dial error with nil config")
	}
}

func TestNewClient_DialFailure(t *testing.T) {
	_, err := NewClient(&Config{Address: "127.0.0.1:1", Insecure: true, ConnectTimeout: time.Second})
	if err == nil {
		t.Fatal("expected dial failure for closed port")
	}
}

func newEchoTLSServer(t *testing.T) *Server {
	t.Helper()
	srv, err := NewServer(&Config{
		Address:     "127.0.0.1:0",
		Insecure:    true,
		RecvTimeout: time.Second,
		SendTimeout: time.Second,
	}, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return append([]byte("echo:"), body...), nil
	}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

func TestClient_Call_WithDeadline(t *testing.T) {
	srv := newEchoTLSServer(t)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Serve(ctx) }()

	client, err := NewClient(&Config{Address: srv.Addr(), Insecure: true, ConnectTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	callCtx, callCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer callCancel()
	_, body, err := client.Call(callCtx, protocol.MsgInvokeRequest, []byte("deadline"))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if got := string(body); got != "echo:deadline" {
		t.Fatalf("body = %q", got)
	}
}

func TestClient_Call_Closing(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	client := &Client{config: &Config{}, conn: c1, closing: make(chan struct{})}
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !client.IsClosed() {
		t.Fatal("client should report closed")
	}
	_, _, err := client.Call(context.Background(), protocol.MsgInvokeRequest, nil)
	if err == nil || err.Error() != "client is closing" {
		t.Fatalf("expected closing error, got %v", err)
	}
}

func TestClient_Call_WriteFailure(t *testing.T) {
	c1, _ := net.Pipe()
	requireNoError(t, c1.Close())
	client := &Client{config: &Config{}, conn: c1, closing: make(chan struct{})}
	_, _, err := client.Call(context.Background(), protocol.MsgInvokeRequest, []byte("x"))
	if err == nil {
		t.Fatal("expected write failure after conn closed")
	}
}

// rawFrameServer reads one frame and replies with a crafted frame.
func rawFrameServer(t *testing.T, respond func(reqFrame []byte) []byte) (addr string, stop func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		frame, err := readFrame(conn)
		if err != nil {
			return
		}
		if err := writeFrame(conn, respond(frame)); err != nil {
			return
		}
		// Wait for the client to finish before returning.
		time.Sleep(200 * time.Millisecond)
	}()
	return ln.Addr().String(), func() {
		_ = ln.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}
}

func TestClient_Call_RequestIDMismatch(t *testing.T) {
	addr, stop := rawFrameServer(t, func(reqFrame []byte) []byte {
		_, msgID, reqID, body, _ := protocol.ParseMessageFromBody(reqFrame)
		return protocol.NewMessageBody(msgID+1, reqID+1, body)
	})
	defer stop()

	client, err := NewClient(&Config{Address: addr, Insecure: true, ConnectTimeout: time.Second, RecvTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	_, _, err = client.Call(context.Background(), protocol.MsgInvokeRequest, []byte("x"))
	if err == nil {
		t.Fatal("expected request ID mismatch error")
	}
}

func TestClient_Call_ParseResponseFailure(t *testing.T) {
	addr, stop := rawFrameServer(t, func(reqFrame []byte) []byte {
		return []byte{0x01, 0x02} // shorter than the protocol header
	})
	defer stop()

	client, err := NewClient(&Config{Address: addr, Insecure: true, ConnectTimeout: time.Second, RecvTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	_, _, err = client.Call(context.Background(), protocol.MsgInvokeRequest, []byte("x"))
	if err == nil {
		t.Fatal("expected parse response error")
	}
}

func TestClient_Call_ReadFailure(t *testing.T) {
	addr, stop := rawFrameServer(t, func(reqFrame []byte) []byte {
		return nil // server never writes a valid frame
	})
	defer stop()

	client, err := NewClient(&Config{Address: addr, Insecure: true, ConnectTimeout: time.Second, RecvTimeout: 300 * time.Millisecond})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	_, _, err = client.Call(context.Background(), protocol.MsgInvokeRequest, []byte("x"))
	if err == nil {
		t.Fatal("expected read failure")
	}
}

// ---------------------------------------------------------------------------
// Dial / TLS configs
// ---------------------------------------------------------------------------

func TestDial_TLSConfigErrors(t *testing.T) {
	cfg := &Config{Address: "127.0.0.1:0", CAFile: filepath.Join(t.TempDir(), "missing-ca.pem")}
	if _, err := Dial(cfg); err == nil {
		t.Fatal("expected read CA file error")
	}

	invalidCA := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(invalidCA, []byte("not a pem"), 0o600); err != nil {
		t.Fatalf("write ca: %v", err)
	}
	if _, err := Dial(&Config{Address: "127.0.0.1:0", CAFile: invalidCA}); err == nil {
		t.Fatal("expected append CA certificate error")
	}

	if _, err := Dial(&Config{
		Address:  "127.0.0.1:0",
		CertFile: filepath.Join(t.TempDir(), "missing-cert.pem"),
		KeyFile:  filepath.Join(t.TempDir(), "missing-key.pem"),
	}); err == nil {
		t.Fatal("expected load client certificate error")
	}
}

// genSelfSignedCert writes a self-signed certificate and key to dir and
// returns their paths.
func genSelfSignedCert(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "croupier-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certPath, keyPath
}

func TestCreateClientTLSConfig_Full(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := genSelfSignedCert(t, dir)

	cfg, err := createClientTLSConfig(&Config{
		CAFile:     certPath,
		CertFile:   certPath,
		KeyFile:    keyPath,
		ServerName: "croupier-test",
	})
	if err != nil {
		t.Fatalf("createClientTLSConfig: %v", err)
	}
	if cfg.ServerName != "croupier-test" {
		t.Fatalf("ServerName = %q", cfg.ServerName)
	}
	if cfg.RootCAs == nil {
		t.Fatal("RootCAs should be set from CAFile")
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("Certificates = %d, want 1", len(cfg.Certificates))
	}
}

// ---------------------------------------------------------------------------
// MuxConn.Run branches
// ---------------------------------------------------------------------------

func TestMuxConn_Run_UnsupportedVersion(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	mc := NewMuxConn(c1, nil, nil)

	go func() {
		// version byte 2, valid msgID/request framing after it.
		frame := append([]byte{0x02, 0x03, 0x01, 0x01, 0, 0, 0, 1}, []byte("{}")...)
		_ = writeFrame(c2, frame)
	}()

	err := mc.Run(context.Background())
	if err == nil {
		t.Fatal("expected protocol error for unsupported version")
	}
	if !isProtocolError(err) {
		t.Fatalf("expected ProtocolError, got %v", err)
	}
}

func TestMuxConn_Run_EventWithoutHandler(t *testing.T) {
	c1, c2 := net.Pipe()
	mc := NewMuxConn(c1, nil, nil)

	errCh := make(chan error, 1)
	go func() { errCh <- mc.Run(context.Background()) }()

	frame := protocol.NewMessageBody(protocol.MsgTaskEvent, 0, []byte(`{}`))
	if err := writeFrame(c2, frame); err != nil {
		t.Fatalf("write event: %v", err)
	}
	// No handler: the loop continues; close the peer to terminate Run.
	_ = c2.Close()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected read error after peer close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not terminate")
	}
}

func TestMuxConn_Run_EventWithHandler(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	eventCh := make(chan uint32, 1)
	mc := NewMuxConn(c1, nil, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		select {
		case eventCh <- msgID:
		default:
		}
		return nil, nil
	}))

	errCh := make(chan error, 1)
	go func() { errCh <- mc.Run(context.Background()) }()

	frame := protocol.NewMessageBody(protocol.MsgMetricEvent, 0, []byte(`{}`))
	if err := writeFrame(c2, frame); err != nil {
		t.Fatalf("write event: %v", err)
	}
	select {
	case got := <-eventCh:
		if got != protocol.MsgMetricEvent {
			t.Fatalf("handler msgID = %x", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler not invoked for event")
	}
	_ = c2.Close()
	<-errCh
}

func TestMuxConn_Run_RequestWithoutHandler(t *testing.T) {
	c1, c2 := net.Pipe()
	mc := NewMuxConn(c1, nil, nil)

	errCh := make(chan error, 1)
	go func() { errCh <- mc.Run(context.Background()) }()

	frame := protocol.NewMessageBody(protocol.MsgInvokeRequest, 1, []byte(`{}`))
	if err := writeFrame(c2, frame); err != nil {
		t.Fatalf("write request: %v", err)
	}
	select {
	case err := <-errCh:
		if err == nil || !isProtocolError(err) {
			t.Fatalf("expected protocol error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not terminate")
	}
	_ = c2.Close()
}

func TestMuxConn_Run_RecvTimeoutLoop(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	mc := NewMuxConn(c1, &Config{RecvTimeout: 50 * time.Millisecond}, nil)

	errCh := make(chan error, 1)
	go func() { errCh <- mc.Run(context.Background()) }()

	// With recvTimeout set, idle reads keep timing out and looping. Cancel
	// via Close after a short window.
	time.Sleep(150 * time.Millisecond)
	_ = mc.Close()
	select {
	case err := <-errCh:
		// Depending on where Close() lands, Run either observes the closed
		// channel (nil) or the in-flight read failing with a closed-pipe error.
		if err != nil && !strings.Contains(err.Error(), "closed pipe") {
			t.Fatalf("Run after Close = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not terminate after Close")
	}
}

// ---------------------------------------------------------------------------
// handleInboundRequest branches
// ---------------------------------------------------------------------------

func TestMuxConn_HandleInboundRequest_HandlerErrorInvoke(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	mc := NewMuxConn(c1, nil, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return nil, context.DeadlineExceeded
	}))

	errCh := make(chan error, 1)
	go func() {
		errCh <- mc.handleInboundRequest(context.Background(), protocol.MsgInvokeRequest, 7, []byte(`{}`))
	}()

	frame, err := readFrame(c2)
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	_, msgID, reqID, body, err := protocol.ParseMessageFromBody(frame)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if msgID != protocol.MsgInvokeResponse || reqID != 7 {
		t.Fatalf("msgID=%x reqID=%d", msgID, reqID)
	}
	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(body, resp); err != nil {
		t.Fatalf("unmarshal InvokeResponse: %v", err)
	}
	if got := string(resp.Payload); got != `{"error":"context deadline exceeded"}` {
		t.Fatalf("payload = %q", got)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("handleInboundRequest: %v", err)
	}
}

func TestMuxConn_HandleInboundRequest_HandlerErrorOther(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	mc := NewMuxConn(c1, nil, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return nil, context.Canceled
	}))

	errCh := make(chan error, 1)
	go func() {
		errCh <- mc.handleInboundRequest(context.Background(), protocol.MsgGetSystemInfoRequest, 9, []byte(`{}`))
	}()

	frame, err := readFrame(c2)
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	_, _, _, body, err := protocol.ParseMessageFromBody(frame)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("expected empty body for non-invoke error, got %q", body)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("handleInboundRequest: %v", err)
	}
}

func TestMuxConn_HandleInboundRequest_ProtocolError(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	mc := NewMuxConn(c1, nil, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return nil, NewProtocolError(context.Canceled)
	}))

	err := mc.handleInboundRequest(context.Background(), protocol.MsgInvokeRequest, 1, []byte(`{}`))
	if err == nil || !isProtocolError(err) {
		t.Fatalf("expected protocol error, got %v", err)
	}
}

func TestMuxConn_HandleInboundRequest_Success(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	mc := NewMuxConn(c1, nil, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return []byte("ok"), nil
	}))

	errCh := make(chan error, 1)
	go func() {
		errCh <- mc.handleInboundRequest(context.Background(), protocol.MsgHeartbeatRequest, 3, []byte(`{}`))
	}()

	frame, err := readFrame(c2)
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	_, msgID, reqID, body, err := protocol.ParseMessageFromBody(frame)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if msgID != protocol.MsgHeartbeatResponse || reqID != 3 || string(body) != "ok" {
		t.Fatalf("msgID=%x reqID=%d body=%q", msgID, reqID, body)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("handleInboundRequest: %v", err)
	}
}

func TestMuxConn_HandleInboundRequestAsync_ClosesConnOnError(t *testing.T) {
	c1, c2 := net.Pipe()
	_ = c2.Close()
	mc := NewMuxConn(c1, nil, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return []byte("ok"), nil
	}))

	// dispatchInbound 投递到业务车道；worker 处理时写失败（对端已关）
	// 必须关闭连接（laneWorker 语义，替代原 handleInboundRequestAsync）。
	if err := mc.dispatchInbound(context.Background(), protocol.MsgInvokeRequest, 1, []byte(`{}`)); err != nil {
		t.Fatalf("dispatchInbound: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !mc.IsClosed() {
		if time.Now().After(deadline) {
			t.Fatal("connection should be closed after write failure")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// ---------------------------------------------------------------------------
// MuxConn Call/Send/writeFrame branches
// ---------------------------------------------------------------------------

func TestMuxConn_Call_WriteFailure(t *testing.T) {
	c1, _ := net.Pipe()
	// Close the raw conn without going through Close() so the closed channel
	// is not triggered and the write path is exercised.
	requireNoError(t, c1.Close())
	mc := NewMuxConn(c1, nil, nil)

	_, _, err := mc.Call(context.Background(), protocol.MsgInvokeRequest, []byte(`{}`))
	if err == nil {
		t.Fatal("expected write failure")
	}
}

func TestMuxConn_Call_ContextCanceledWhileWaiting(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	mc := NewMuxConn(c1, nil, nil)
	defer mc.Close()

	// Consume the request frame but never respond.
	go func() {
		_, _ = readFrame(c2)
		<-time.After(2 * time.Second)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _, err := mc.Call(ctx, protocol.MsgInvokeRequest, []byte(`{}`))
	if err == nil {
		t.Fatal("expected context deadline error")
	}
}

func TestMuxConn_Send_ContextCanceled(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	mc := NewMuxConn(c1, nil, nil)
	defer mc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := mc.Send(ctx, protocol.MsgTaskEvent, []byte(`{}`)); err == nil {
		t.Fatal("expected context canceled error")
	}
}

func TestMuxConn_WriteFrame_SendTimeout(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	mc := NewMuxConn(c1, &Config{SendTimeout: 100 * time.Millisecond}, nil)
	defer mc.Close()

	// net.Pipe honors deadlines; with nobody reading the peer, the write
	// must time out.
	if err := mc.writeFrame(1, protocol.MsgTaskEvent, []byte(`{}`)); err == nil {
		t.Fatal("expected write timeout")
	}
}

// ---------------------------------------------------------------------------
// Server.serveConn handler-error and frame branches
// ---------------------------------------------------------------------------

func TestServer_ServeConn_HandlerErrors(t *testing.T) {
	srv, err := NewServer(&Config{
		Address:     "127.0.0.1:0",
		Insecure:    true,
		RecvTimeout: time.Second,
		SendTimeout: time.Second,
	}, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return nil, context.DeadlineExceeded
	}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer srv.Close()
	go func() { _ = srv.Serve(context.Background()) }()

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// InvokeRequest errors produce an InvokeResponse with an error payload.
	req := protocol.NewMessageBody(protocol.MsgInvokeRequest, 1, []byte(`{}`))
	if err := writeFrame(conn, req); err != nil {
		t.Fatalf("write: %v", err)
	}
	frame, err := readFrame(conn)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	_, msgID, _, body, err := protocol.ParseMessageFromBody(frame)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if msgID != protocol.MsgInvokeResponse {
		t.Fatalf("msgID = %x", msgID)
	}
	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(body, resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Payload) == 0 {
		t.Fatal("expected error payload")
	}

	// Non-invoke errors yield an empty response body.
	req2 := protocol.NewMessageBody(protocol.MsgGetSystemInfoRequest, 2, []byte(`{}`))
	if err := writeFrame(conn, req2); err != nil {
		t.Fatalf("write: %v", err)
	}
	frame2, err := readFrame(conn)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	_, _, _, body2, err := protocol.ParseMessageFromBody(frame2)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(body2) != 0 {
		t.Fatalf("expected empty body, got %q", body2)
	}
}

func TestServer_ServeConn_InvalidFrameClosesConn(t *testing.T) {
	srv, err := NewServer(&Config{
		Address:  "127.0.0.1:0",
		Insecure: true,
	}, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return nil, nil
	}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer srv.Close()
	go func() { _ = srv.Serve(context.Background()) }()

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// 3-byte body: shorter than the protocol header → parse error → close.
	if _, err := conn.Write([]byte{0, 0, 0, 3, 1, 2, 3}); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 16)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("expected connection close after invalid frame")
	}
}

// ---------------------------------------------------------------------------
// Server TLS listener
// ---------------------------------------------------------------------------

func TestListen_TLSConfigErrors(t *testing.T) {
	if _, err := listen(&Config{Address: "127.0.0.1:0", CertFile: "missing.pem", KeyFile: "missing.pem"}); err == nil {
		t.Fatal("expected server cert load error")
	}

	invalidCA := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(invalidCA, []byte("nope"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := listen(&Config{Address: "127.0.0.1:0", CAFile: invalidCA}); err == nil {
		t.Fatal("expected CA append error")
	}
}

func TestListen_WithCertificates(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := genSelfSignedCert(t, dir)

	ln, err := listen(&Config{Address: "127.0.0.1:0", CertFile: certPath, KeyFile: keyPath, InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("listen tls: %v", err)
	}
	defer ln.Close()
	if ln.Addr() == nil {
		t.Fatal("listener addr should not be nil")
	}
}

func TestCreateServerTLSConfig_Defaults(t *testing.T) {
	cfg, err := createServerTLSConfig(&Config{})
	if err != nil {
		t.Fatalf("createServerTLSConfig: %v", err)
	}
	if cfg.ClientAuth != tls.NoClientCert {
		t.Fatalf("ClientAuth = %v", cfg.ClientAuth)
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
