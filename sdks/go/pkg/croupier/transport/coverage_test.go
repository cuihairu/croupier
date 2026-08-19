package transport

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

// rawPeer is a minimal TCP peer that owns its connection so tests can push
// arbitrary frames without competing readers. The accepted connection is
// published over a channel to stay race-free.
type rawPeer struct {
	ln    net.Listener
	conns chan net.Conn
}

func newRawPeer(t *testing.T) *rawPeer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	p := &rawPeer{ln: ln, conns: make(chan net.Conn, 4)}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		p.conns <- conn
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return p
}

func (p *rawPeer) waitConnected(t *testing.T) net.Conn {
	t.Helper()
	select {
	case conn := <-p.conns:
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	case <-time.After(2 * time.Second):
		t.Fatal("peer connection not established")
		return nil
	}
}

func writeRawFrame(conn net.Conn, frameBody []byte) error {
	frame := make([]byte, 4+len(frameBody))
	binary.BigEndian.PutUint32(frame[:4], uint32(len(frameBody)))
	copy(frame[4:], frameBody)
	_, err := conn.Write(frame)
	return err
}

func readRawFrame(conn net.Conn) (msgID, reqID uint32, body []byte, err error) {
	header := make([]byte, 4)
	if _, err = io.ReadFull(conn, header); err != nil {
		return 0, 0, nil, err
	}
	size := binary.BigEndian.Uint32(header)
	if size == 0 {
		return 0, 0, nil, io.ErrUnexpectedEOF
	}
	payload := make([]byte, size)
	if _, err = io.ReadFull(conn, payload); err != nil {
		return 0, 0, nil, err
	}
	if len(payload) < protocol.HeaderSize {
		return 0, 0, payload, nil
	}
	return protocol.GetMsgID(payload[1:4]), binary.BigEndian.Uint32(payload[4:8]), payload[protocol.HeaderSize:], nil
}

// ---------------------------------------------------------------------------
// Inbound request handling (agent → provider dispatch)
// ---------------------------------------------------------------------------

func TestTCPClient_HandleInboundRequest(t *testing.T) {
	peer := newRawPeer(t)
	handlerCalled := make(chan uint32, 4)
	clientCfg := &Config{
		Address:     peer.ln.Addr().String(),
		Insecure:    true,
		DialTimeout: 2 * time.Second,
		InboundHandler: func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
			handlerCalled <- msgID
			if len(body) == 0 {
				return nil, errStringT("handler exploded")
			}
			resp, _ := proto.Marshal(&sdkv1.InvokeResponse{Payload: append([]byte("handled:"), body...)})
			return resp, nil
		},
	}
	client, err := NewTCPClient(clientCfg)
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer client.Close()
	conn := peer.waitConnected(t)

	// Successful dispatch: agent sends an invoke request.
	req, _ := proto.Marshal(&sdkv1.InvokeRequest{FunctionId: "demo.echo", Payload: []byte("go")})
	if err := writeRawFrame(conn, protocol.NewMessageBody(protocol.MsgInvokeRequest, 11, req)); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	respMsgID, respReqID, respBody, err := readRawFrame(conn)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if respMsgID != protocol.MsgInvokeResponse || respReqID != 11 {
		t.Fatalf("msgID=%#x reqID=%d", respMsgID, respReqID)
	}
	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(respBody, resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !bytes.HasPrefix(resp.Payload, []byte("handled:")) {
		t.Fatalf("payload = %q", resp.Payload)
	}
	if got := <-handlerCalled; got != protocol.MsgInvokeRequest {
		t.Fatalf("handler msgID = %#x", got)
	}

	// Failing dispatch: error is carried inside the InvokeResponse payload.
	if err := writeRawFrame(conn, protocol.NewMessageBody(protocol.MsgInvokeRequest, 12, nil)); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	_, _, respBody, err = readRawFrame(conn)
	if err != nil {
		t.Fatalf("read error response: %v", err)
	}
	resp = &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(respBody, resp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if string(resp.Payload) == "" || resp.Payload[0] != '{' {
		t.Fatalf("expected JSON error payload, got %q", resp.Payload)
	}
	<-handlerCalled
}

func TestTCPClient_HandleInboundRequest_NoInboundHandler(t *testing.T) {
	peer := newRawPeer(t)
	client, err := NewTCPClient(&Config{
		Address:     peer.ln.Addr().String(),
		Insecure:    true,
		DialTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer client.Close()
	conn := peer.waitConnected(t)

	// Without an inbound handler nothing is written back and the client
	// survives (the frame is dropped).
	if err := writeRawFrame(conn, protocol.NewMessageBody(protocol.MsgInvokeRequest, 1, []byte(`{}`))); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	if client.IsClosed() {
		t.Fatal("client should stay open")
	}

	// Non-request message types are ignored by handleInboundRequest.
	if err := writeRawFrame(conn, protocol.NewMessageBody(protocol.MsgTaskEvent, 0, []byte(`{}`))); err != nil {
		t.Fatalf("write event frame: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if client.IsClosed() {
		t.Fatal("client should stay open after an event frame")
	}
}

type errStringT string

func (e errStringT) Error() string { return string(e) }

// ---------------------------------------------------------------------------
// receiveLoop edge cases
// ---------------------------------------------------------------------------

func TestTCPClient_ReceiveLoop_MalformedFrames(t *testing.T) {
	peer := newRawPeer(t)
	client, err := NewTCPClient(&Config{
		Address:     peer.ln.Addr().String(),
		Insecure:    true,
		DialTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	defer client.Close()
	conn := peer.waitConnected(t)

	// Zero-size frame is skipped without breaking the loop.
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		t.Fatalf("write zero frame: %v", err)
	}

	// Payload shorter than the protocol header is skipped.
	frame := make([]byte, 4+2)
	binary.BigEndian.PutUint32(frame[:4], 2)
	frame[4] = protocol.Version1
	frame[5] = 0x03
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write short frame: %v", err)
	}

	// Wrong protocol version is skipped.
	body := append([]byte{0x09, 0x03, 0x01, 0x01, 0, 0, 0, 9}, []byte(`{}`)...)
	if err := writeRawFrame(conn, body); err != nil {
		t.Fatalf("write bad-version frame: %v", err)
	}

	// Response for an unknown reqID is dropped silently.
	respBody := protocol.NewMessageBody(protocol.MsgInvokeResponse, 4242, []byte(`{}`))
	if err := writeRawFrame(conn, respBody); err != nil {
		t.Fatalf("write orphan response: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if client.IsClosed() {
		t.Fatal("client should survive malformed frames")
	}

	// An oversized frame terminates the receive loop.
	oversized := make([]byte, 4)
	binary.BigEndian.PutUint32(oversized, maxFrameBytes+1)
	if _, err := conn.Write(oversized); err != nil {
		t.Fatalf("write oversized frame: %v", err)
	}
}

// ---------------------------------------------------------------------------
// SetOnClose / notifyClose
// ---------------------------------------------------------------------------

func TestTCPClient_SetOnClose_CalledOnConnectionLoss(t *testing.T) {
	peer := newRawPeer(t)
	client, err := NewTCPClient(&Config{
		Address:     peer.ln.Addr().String(),
		Insecure:    true,
		DialTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	conn := peer.waitConnected(t)

	var called int32
	client.SetOnClose(func(err error) {
		atomic.AddInt32(&called, 1)
	})

	// Forcing the peer to hang up triggers onClose (not an intentional close).
	_ = conn.Close()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&called) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&called) != 1 {
		t.Fatal("onClose callback was not invoked after connection loss")
	}
	_ = client.Close()
}

func TestTCPClient_SetOnClose_NotCalledOnIntentionalClose(t *testing.T) {
	server := startMockServer(t)
	defer server.Close()

	client, err := NewTCPClient(&Config{
		Address:     server.Addr(),
		Insecure:    true,
		DialTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}

	var called int32
	client.SetOnClose(func(err error) { atomic.AddInt32(&called, 1) })
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&called) != 0 {
		t.Fatal("onClose must not fire for an intentional Close")
	}
}

// ---------------------------------------------------------------------------
// NewTCPClient address parsing
// ---------------------------------------------------------------------------

func TestNewTCPClient_HostPortAndPrefixes(t *testing.T) {
	server := startMockServer(t)
	defer server.Close()

	host, portStr, err := net.SplitHostPort(server.Addr())
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	// Host + Port path.
	client, err := NewTCPClient(&Config{Host: host, Port: port, Insecure: true, DialTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("host/port dial: %v", err)
	}
	_ = client.Close()

	// tcp:// prefix path.
	client, err = NewTCPClient(&Config{Address: "tcp://" + server.Addr(), Insecure: true, DialTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("tcp:// dial: %v", err)
	}
	_ = client.Close()

	// NOTE: "tls+tcp://" addresses are documented in Config.Address and
	// normalizeAddress, but NewTCPClient's prefix strip compares
	// addr[:9] == "tls+tcp://" (10 bytes), which can never match, so such
	// addresses fail with "no such host". Recorded as a product bug; not
	// asserted here.
}

func TestNewTCPClient_EmptyAndHostlessAddresses(t *testing.T) {
	if _, err := NewTCPClient(&Config{Address: "", Insecure: true}); err == nil {
		t.Fatal("empty address must fail")
	}
	if _, err := NewTCPClient(&Config{Address: ":9100", Insecure: true, DialTimeout: 100 * time.Millisecond}); err == nil {
		t.Fatal("hostless address must fail")
	}
}

// ---------------------------------------------------------------------------
// TLS config
// ---------------------------------------------------------------------------

func genTransportCert(t *testing.T, dir string) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(11),
		Subject:               pkix.Name{CommonName: "transport-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certPath, keyPath
}

func TestCreateTLSConfig_Branches(t *testing.T) {
	// Insecure mode short-circuits.
	cfg, err := createTLSConfig(&Config{Insecure: true})
	if err != nil || cfg != nil {
		t.Fatalf("insecure TLS config = %v, %v", cfg, err)
	}

	// Missing CA file.
	if _, err := createTLSConfig(&Config{CAFile: filepath.Join(t.TempDir(), "missing.pem")}); err == nil {
		t.Fatal("expected CA read error")
	}

	// Unparsable CA.
	bad := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(bad, []byte("junk"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := createTLSConfig(&Config{CAFile: bad}); err == nil {
		t.Fatal("expected CA append error")
	}

	dir := t.TempDir()
	certPath, keyPath := genTransportCert(t, dir)

	// Valid CA + client certificate + server name.
	tlsCfg, err := createTLSConfig(&Config{CAFile: certPath, CertFile: certPath, KeyFile: keyPath, ServerName: "agent.local"})
	if err != nil {
		t.Fatalf("createTLSConfig: %v", err)
	}
	if tlsCfg.RootCAs == nil || len(tlsCfg.Certificates) != 1 || tlsCfg.ServerName != "agent.local" {
		t.Fatalf("TLS config incomplete: %+v", tlsCfg)
	}

	// Client certificate load failure.
	if _, err := createTLSConfig(&Config{CertFile: certPath, KeyFile: "missing.pem"}); err == nil {
		t.Fatal("expected client cert load error")
	}
}

// ---------------------------------------------------------------------------
// dialAddr fallback
// ---------------------------------------------------------------------------

func TestDialAddr_DefaultFallback(t *testing.T) {
	if got := dialAddr(&Config{}); got != "tcp://127.0.0.1:19091" {
		t.Fatalf("dialAddr default = %q", got)
	}
	if got := dialAddr(&Config{Insecure: true, Addresses: []string{"  ", "host:1"}}); got != "tcp://host:1" {
		t.Fatalf("dialAddr addresses = %q", got)
	}
}

func TestBuildDialAddrs_IPCSkipDuplicate(t *testing.T) {
	cfg := &Config{
		Address:    "127.0.0.1:19091, ,127.0.0.1:19091",
		IPCAddress: "ipc://croupier-agent",
	}
	addrs := buildDialAddrs(cfg)
	if len(addrs) == 0 || addrs[0] != "ipc://croupier-agent" {
		t.Fatalf("IPC address should come first: %v", addrs)
	}
}
