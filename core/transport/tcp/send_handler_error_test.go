package tcp

// Send 非事件拒绝、handleInboundRequest 处理器错误回包、ServerTLSConfig
// 坏证书分支：这些错误路径此前未被任何用例触达。复用既有 net.Pipe
// fixture 形态（同包 client_test / mux_conn_coverage_test）。

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	transportcore "github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/pkg/protocol"
)

func TestClientSend_RejectsNonEvent(t *testing.T) {
	// Send 在触达 conn 之前就拒绝非事件帧，conn 可以为 nil。
	c := &Client{config: &Config{}, closing: make(chan struct{})}
	defer close(c.closing)
	err := c.Send(context.Background(), protocol.MsgInvokeRequest, []byte(`{}`))
	if err == nil {
		t.Fatal("expected non-event Send to be rejected")
	}
	if err.Error() != "message InvokeRequest is not an event" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMuxConnHandleInboundHandlerError_NonInvoke(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
	// 非 protocol error：走日志 + 空响应体分支（MsgInvokeRequest 之外）。
	mc := NewMuxConn(c1, nil, transportcore.HandlerFunc(func(_ context.Context, msgID uint32, _ uint32, _ []byte) ([]byte, error) {
		return nil, errors.New("boom")
	}))
	errCh := make(chan error, 1)
	go func() {
		errCh <- mc.handleInboundRequest(context.Background(), protocol.MsgHeartbeatRequest, 3, []byte(`{}`))
	}()
	frame, err := ReadFrame(c2)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	_, msgID, reqID, body, err := protocol.ParseMessageFromBody(frame)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if msgID != protocol.MsgHeartbeatResponse || reqID != 3 || len(body) != 0 {
		t.Fatalf("msgID=%x reqID=%d body=%q, want empty body response", msgID, reqID, body)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("handleInboundRequest: %v", err)
	}
}

func TestMuxConnHandleInboundHandlerError_Invoke(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
	mc := NewMuxConn(c1, nil, transportcore.HandlerFunc(func(_ context.Context, msgID uint32, _ uint32, _ []byte) ([]byte, error) {
		return nil, errors.New("boom")
	}))
	errCh := make(chan error, 1)
	go func() {
		errCh <- mc.handleInboundRequest(context.Background(), protocol.MsgInvokeRequest, 7, []byte(`{}`))
	}()
	frame, err := ReadFrame(c2)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	_, msgID, reqID, body, err := protocol.ParseMessageFromBody(frame)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if msgID != protocol.MsgInvokeResponse || reqID != 7 {
		t.Fatalf("msgID=%x reqID=%d, want InvokeResponse echo", msgID, reqID)
	}
	if len(body) == 0 {
		t.Fatal("expected error payload in InvokeResponse")
	}
	if err := <-errCh; err != nil {
		t.Fatalf("handleInboundRequest: %v", err)
	}
}

func TestServerTLSConfig_InvalidKeyPair(t *testing.T) {
	_, err := ServerTLSConfig(&Config{CertFile: "/nonexistent/cert.pem", KeyFile: "/nonexistent/key.pem"})
	if err == nil {
		t.Fatal("expected load certificate error")
	}
	if !strings.Contains(err.Error(), "load server certificate") {
		t.Fatalf("unexpected error: %v", err)
	}
}
