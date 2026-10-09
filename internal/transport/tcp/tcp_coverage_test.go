package tcp

import (
	"context"
	"crypto/tls"
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
// MuxConn.Run branches
// ---------------------------------------------------------------------------

func TestMuxConn_Run_UnsupportedVersion(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
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
	defer func() { _ = c1.Close() }()
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
	defer func() { _ = c2.Close() }()
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
	defer func() { _ = c2.Close() }()
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
	defer func() { _ = c2.Close() }()
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
	defer func() { _ = c2.Close() }()
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
	defer func() { _ = c2.Close() }()
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
	defer func() { _ = c2.Close() }()
	mc := NewMuxConn(c1, nil, nil)
	defer func() { _ = mc.Close() }()

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
	defer func() { _ = c2.Close() }()
	mc := NewMuxConn(c1, nil, nil)
	defer func() { _ = mc.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := mc.Send(ctx, protocol.MsgTaskEvent, []byte(`{}`)); err == nil {
		t.Fatal("expected context canceled error")
	}
}

func TestMuxConn_WriteFrame_SendTimeout(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
	mc := NewMuxConn(c1, &Config{SendTimeout: 100 * time.Millisecond}, nil)
	defer func() { _ = mc.Close() }()

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
	defer func() { _ = srv.Close() }()
	go func() { _ = srv.Serve(context.Background()) }()

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

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
	defer func() { _ = srv.Close() }()
	go func() { _ = srv.Serve(context.Background()) }()

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// 3-byte body: shorter than the protocol header → parse error → close.
	if _, err := conn.Write([]byte{0, 0, 0, 3, 1, 2, 3}); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 16)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
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
	defer func() { _ = ln.Close() }()
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
