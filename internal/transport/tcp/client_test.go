package tcp

import (
	"context"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/internal/transport"
	"github.com/cuihairu/croupier/pkg/protocol"
)

func TestClientServerRoundTrip(t *testing.T) {
	srv, err := NewServer(&Config{
		Address:     "127.0.0.1:0",
		Insecure:    true,
		RecvTimeout: 200 * time.Millisecond,
		SendTimeout: 200 * time.Millisecond,
	}, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		if msgID != protocol.MsgInvokeRequest {
			t.Fatalf("unexpected msgID: %x", msgID)
		}
		return append([]byte("echo:"), body...), nil
	}))
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer func() { _ = srv.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = srv.Serve(ctx)
	}()

	client, err := NewClient(&Config{
		Address:        srv.Addr(),
		Insecure:       true,
		ConnectTimeout: time.Second,
		RecvTimeout:    time.Second,
		SendTimeout:    time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer func() { _ = client.Close() }()

	respMsgID, respBody, err := client.Call(context.Background(), protocol.MsgInvokeRequest, []byte("ping"))
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if respMsgID != protocol.MsgInvokeResponse {
		t.Fatalf("Call() respMsgID = %x, want %x", respMsgID, protocol.MsgInvokeResponse)
	}
	if got, want := string(respBody), "echo:ping"; got != want {
		t.Fatalf("Call() respBody = %q, want %q", got, want)
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
	defer func() { _ = srv.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Serve(ctx) }()

	client, err := NewClient(&Config{Address: srv.Addr(), Insecure: true, ConnectTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer func() { _ = client.Close() }()

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
