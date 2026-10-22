package tcp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/pkg/protocol"
)

// roundTripManualServer 自包含帧应答服务（core 侧不依赖 internal Server 实现）。
func roundTripManualServer(t *testing.T, handle func([]byte) []byte) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		for {
			frame, err := ReadFrame(conn)
			if err != nil {
				return
			}
			_, _, reqID, body, err := protocol.ParseMessageFromBody(frame)
			if err != nil {
				return
			}
			resp := protocol.NewMessageBody(protocol.MsgInvokeResponse, reqID, handle(body))
			if err := WriteFrame(conn, resp); err != nil {
				return
			}
		}
	}()
	return ln.Addr().String()
}

func TestClientCallRoundTripCore(t *testing.T) {
	addr := roundTripManualServer(t, func(body []byte) []byte { return append([]byte("echo:"), body...) })
	c, err := NewClient(&Config{Address: addr, Insecure: true, ConnectTimeout: time.Second, RecvTimeout: time.Second, SendTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer func() { _ = c.Close() }()
	_, respBody, err := c.Call(context.Background(), protocol.MsgInvokeRequest, []byte("ping"))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if string(respBody) != "echo:ping" {
		t.Fatalf("respBody = %q", respBody)
	}
}

// ==== 以下随上收自 internal/transport/tcp（client/framing 行为用例原样随迁）====

// v9DeadlineErrConn 所有 Set*Deadline 均失败，用于触发 Client.Call 的
// deadline 设置错误分支。
type v9DeadlineErrConn struct{ net.Conn }

func (v9DeadlineErrConn) SetDeadline(time.Time) error      { return errors.New("deadline boom") }
func (v9DeadlineErrConn) SetWriteDeadline(time.Time) error { return errors.New("wdeadline boom") }
func (v9DeadlineErrConn) SetReadDeadline(time.Time) error  { return errors.New("rdeadline boom") }

func TestClientCall_SetDeadlineErrorV9(t *testing.T) {
	client := &Client{config: &Config{}, conn: v9DeadlineErrConn{}, closing: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _, err := client.Call(ctx, protocol.MsgInvokeRequest, []byte("x"))
	if err == nil || !containsStr(err.Error(), "set deadline") {
		t.Fatalf("expected set deadline error, got %v", err)
	}
}

func TestClientCall_WriteDeadlineErrorV9(t *testing.T) {
	client := &Client{
		config:  &Config{SendTimeout: time.Second},
		conn:    v9DeadlineErrConn{},
		closing: make(chan struct{}),
	}
	_, _, err := client.Call(context.Background(), protocol.MsgInvokeRequest, []byte("x"))
	if err == nil || !containsStr(err.Error(), "set write deadline") {
		t.Fatalf("expected write deadline error, got %v", err)
	}
}

func TestClientCall_ReadDeadlineErrorV9(t *testing.T) {
	client := &Client{
		config:  &Config{RecvTimeout: time.Second},
		conn:    v9DeadlineErrConn{},
		closing: make(chan struct{}),
	}
	_, _, err := client.Call(context.Background(), protocol.MsgInvokeRequest, []byte("x"))
	if err == nil || !containsStr(err.Error(), "set read deadline") {
		t.Fatalf("expected read deadline error, got %v", err)
	}
}

// TestClientCall_ReadFrameErrorV9 覆盖 client.go:111-113：对端读走请求后
// 立即断开，readFrame 失败。
func TestClientCall_ReadFrameErrorV9(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		_, _ = readFrame(conn)
		_ = conn.Close()
	}()

	client, err := NewClient(&Config{Address: ln.Addr().String(), Insecure: true, ConnectTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer func() { _ = client.Close() }()

	_, _, err = client.Call(context.Background(), protocol.MsgInvokeRequest, []byte("x"))
	if err == nil {
		t.Fatal("expected read frame error after peer close")
	}
}

// TestReadFrame_TruncatedPayloadV9 覆盖 framing.go:46-48：header 声明的
// 长度超过实际数据。
func TestReadFrame_TruncatedPayloadV9(t *testing.T) {
	header := make([]byte, FrameHeaderBytes)
	binary.BigEndian.PutUint32(header, 8)
	data := append(header, []byte("ab")...) // 只有 2 字节，声明 8 字节
	_, err := readFrame(io.Reader(bytesReader(data)))
	if err == nil {
		t.Fatal("expected unexpected EOF for truncated payload")
	}
}

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
	if len(p) == FrameHeaderBytes {
		return len(p), nil
	}
	return 0, w.err
}

func TestReadFrame_EmptyFrame(t *testing.T) {
	header := make([]byte, FrameHeaderBytes)
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

func TestClient_Call_Closing(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
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
		defer func() { _ = conn.Close() }()
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
	defer func() { _ = client.Close() }()

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
	defer func() { _ = client.Close() }()

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
	defer func() { _ = client.Close() }()

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

func containsStr(s, sub string) bool { return strings.Contains(s, sub) }

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
