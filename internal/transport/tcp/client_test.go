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
	defer srv.Close()

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
	defer client.Close()

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
