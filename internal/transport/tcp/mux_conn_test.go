package tcp

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/internal/transport"
	"github.com/cuihairu/croupier/pkg/protocol"
)

func TestProtocolError_Error(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		pe := &ProtocolError{}
		if got := pe.Error(); got != "protocol violation" {
			t.Errorf("expected 'protocol violation', got %q", got)
		}
	})

	t.Run("with error", func(t *testing.T) {
		pe := &ProtocolError{Err: fmt.Errorf("bad message")}
		if got := pe.Error(); got != "bad message" {
			t.Errorf("expected 'bad message', got %q", got)
		}
	})

	t.Run("nil receiver", func(t *testing.T) {
		var pe *ProtocolError
		if got := pe.Error(); got != "protocol violation" {
			t.Errorf("expected 'protocol violation', got %q", got)
		}
	})
}

func TestNewProtocolError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		err := NewProtocolError(nil)
		if err == nil {
			t.Fatal("expected non-nil error")
		}
		pe, ok := err.(*ProtocolError)
		if !ok {
			t.Fatalf("expected *ProtocolError, got %T", err)
		}
		if pe.Err == nil {
			t.Error("expected non-nil inner error")
		}
	})

	t.Run("with error", func(t *testing.T) {
		inner := fmt.Errorf("test")
		err := NewProtocolError(inner)
		pe, ok := err.(*ProtocolError)
		if !ok {
			t.Fatalf("expected *ProtocolError, got %T", err)
		}
		if pe.Err != inner {
			t.Error("expected inner error to match")
		}
	})
}

func TestIsProtocolError(t *testing.T) {
	if isProtocolError(nil) {
		t.Error("false positive for nil")
	}
	if isProtocolError(fmt.Errorf("normal")) {
		t.Error("false positive for normal error")
	}
	if !isProtocolError(NewProtocolError(fmt.Errorf("test"))) {
		t.Error("false negative for protocol error")
	}
}

func TestErrorAs(t *testing.T) {
	pe := &ProtocolError{Err: fmt.Errorf("test")}
	var target *ProtocolError
	if !errorAs(pe, &target) {
		t.Error("expected true for ProtocolError")
	}
	if target != pe {
		t.Error("expected target to match")
	}

	if errorAs(fmt.Errorf("other"), &target) {
		t.Error("expected false for non-ProtocolError")
	}
}

func TestNewMuxConn(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		c1, c2 := net.Pipe()
		defer c1.Close()
		defer c2.Close()

		mc := NewMuxConn(c1, nil, nil)
		if mc == nil {
			t.Fatal("expected non-nil MuxConn")
		}
		if mc.recvTimeout != 0 {
			t.Errorf("expected 0 recvTimeout, got %v", mc.recvTimeout)
		}
		if mc.sendTimeout != 0 {
			t.Errorf("expected 0 sendTimeout, got %v", mc.sendTimeout)
		}
	})

	t.Run("with config", func(t *testing.T) {
		c1, c2 := net.Pipe()
		defer c1.Close()
		defer c2.Close()

		handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
			return body, nil
		})
		mc := NewMuxConn(c1, &Config{RecvTimeout: time.Second, SendTimeout: 2 * time.Second}, handler)
		if mc.recvTimeout != time.Second {
			t.Errorf("expected 1s recvTimeout, got %v", mc.recvTimeout)
		}
		if mc.sendTimeout != 2*time.Second {
			t.Errorf("expected 2s sendTimeout, got %v", mc.sendTimeout)
		}
	})
}

func TestMuxConn_RemoteAddr_LocalAddr(t *testing.T) {
	t.Run("nil conn", func(t *testing.T) {
		mc := &MuxConn{}
		if got := mc.RemoteAddr(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
		if got := mc.LocalAddr(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("nil MuxConn", func(t *testing.T) {
		var mc *MuxConn
		if got := mc.RemoteAddr(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
		if got := mc.LocalAddr(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("with pipe", func(t *testing.T) {
		c1, c2 := net.Pipe()
		defer c1.Close()
		defer c2.Close()

		mc := NewMuxConn(c1, nil, nil)
		// Pipe connections may return empty addresses, that's OK
		_ = mc.RemoteAddr()
		_ = mc.LocalAddr()
	})
}

func TestMuxConn_Close(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()

	mc := NewMuxConn(c1, nil, nil)
	if mc.IsClosed() {
		t.Fatal("expected mux conn to be open initially")
	}

	if err := mc.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !mc.IsClosed() {
		t.Fatal("expected mux conn to report closed after Close")
	}

	// Multiple close should not panic
	mc.Close()
	mc.Close()
}

func TestMuxConn_Close_FailsPending(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()

	mc := NewMuxConn(c1, nil, nil)
	mc.pending[1] = make(chan muxResponse, 1)

	mc.Close()

	// Pending channel should receive an error
	select {
	case resp := <-mc.pending[1]:
		if resp.err == nil {
			t.Error("expected error in pending response")
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for pending fail")
	}
}

func TestMuxConn_Send_Closed(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()

	mc := NewMuxConn(c1, nil, nil)
	mc.Close()

	err := mc.Send(context.Background(), protocol.MsgTaskEvent, []byte("test"))
	if err == nil {
		t.Error("expected error sending on closed connection")
	}
}

func TestMuxConn_Send_NilContext(t *testing.T) {
	c1, c2 := net.Pipe()

	mc := NewMuxConn(c1, nil, nil)

	// Drain reads on remote end in background
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := c2.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	done := make(chan error, 1)
	go func() {
		done <- mc.Send(nil, protocol.MsgTaskEvent, []byte("test"))
	}()

	select {
	case err := <-done:
		// Send completed (may succeed or fail depending on timing)
		_ = err
	case <-time.After(2 * time.Second):
		t.Error("Send with nil context did not complete")
	}
	mc.Close()
	c2.Close()
}

func TestMuxConn_Send_NonEvent(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	mc := NewMuxConn(c1, nil, nil)
	defer mc.Close()

	err := mc.Send(context.Background(), protocol.MsgInvokeRequest, []byte("test"))
	if err == nil {
		t.Error("expected error for non-event message")
	}
}

func TestMuxConn_Call_Closed(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()

	mc := NewMuxConn(c1, nil, nil)
	mc.Close()

	_, _, err := mc.Call(context.Background(), protocol.MsgInvokeRequest, []byte("test"))
	if err == nil {
		t.Error("expected error calling on closed connection")
	}
}

func TestMuxConn_Call_NilContext(t *testing.T) {
	c1, c2 := net.Pipe()

	mc := NewMuxConn(c1, nil, nil)

	// Drain reads on remote end and optionally respond
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := c2.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		_, _, _ = mc.Call(nil, protocol.MsgInvokeRequest, []byte("test"))
		close(done)
	}()

	// Give it a moment then close to unblock
	time.AfterFunc(200*time.Millisecond, func() {
		mc.Close()
	})

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Error("Call with nil context did not complete")
	}
	c2.Close()
}

func TestMuxConn_Run_ContextCancellation(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()

	mc := NewMuxConn(c1, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- mc.Run(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Logf("Run returned: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Run did not return after context cancellation")
	}
}

func TestMuxConn_Run_NilContext(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()

	mc := NewMuxConn(c1, nil, nil)

	done := make(chan error, 1)
	go func() {
		done <- mc.Run(nil)
	}()

	// Close the connection to unblock Run
	mc.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("Run did not return after close")
	}
}

func TestMuxConn_FulfillPending(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	mc := NewMuxConn(c1, nil, nil)

	ch := make(chan muxResponse, 1)
	mc.pending[42] = ch

	mc.fulfillPending(42, muxResponse{msgID: protocol.MsgInvokeResponse, body: []byte("ok")})

	select {
	case resp := <-ch:
		if resp.msgID != protocol.MsgInvokeResponse {
			t.Errorf("expected MsgInvokeResponse, got %x", resp.msgID)
		}
		if string(resp.body) != "ok" {
			t.Errorf("expected 'ok', got %q", resp.body)
		}
	default:
		t.Error("expected response in channel")
	}
}

func TestMuxConn_FulfillPending_UnknownReqID(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	mc := NewMuxConn(c1, nil, nil)
	// Should not panic for unknown reqID
	mc.fulfillPending(999, muxResponse{})
}

func TestMuxConn_FulfillPending_FullChannel(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	mc := NewMuxConn(c1, nil, nil)

	// Already full channel - should not block
	ch := make(chan muxResponse, 1)
	ch <- muxResponse{}
	mc.pending[1] = ch

	// Should not block
	mc.fulfillPending(1, muxResponse{})
}

func TestMuxConn_FailPending(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	mc := NewMuxConn(c1, nil, nil)

	ch1 := make(chan muxResponse, 1)
	ch2 := make(chan muxResponse, 1)
	mc.pending[1] = ch1
	mc.pending[2] = ch2

	mc.failPending(fmt.Errorf("test error"))

	for _, ch := range []chan muxResponse{ch1, ch2} {
		select {
		case resp := <-ch:
			if resp.err == nil {
				t.Error("expected error")
			}
		default:
			t.Error("expected response in channel")
		}
	}
}

func TestIsTimeout(t *testing.T) {
	if isTimeout(nil) {
		t.Error("false positive for nil")
	}
	if isTimeout(fmt.Errorf("normal")) {
		t.Error("false positive for non-timeout error")
	}
}

func TestMuxConn_Run_ReadError(t *testing.T) {
	c1, c2 := net.Pipe()

	mc := NewMuxConn(c1, nil, nil)

	// Close the remote end to cause a read error
	c2.Close()

	err := mc.Run(context.Background())
	if err == nil {
		t.Error("expected error when remote closes")
	}
}

func TestMuxConn_Run_InvalidFrame(t *testing.T) {
	c1, c2 := net.Pipe()

	mc := NewMuxConn(c1, nil, nil)

	// Write invalid protocol data
	go func() {
		// Write a valid frame but with bad protocol content
		writeFrame(c2, []byte{0xFF, 0xFF, 0xFF, 0xFF})
		c2.Close()
	}()

	err := mc.Run(context.Background())
	if err == nil {
		t.Error("expected error for invalid frame")
	}
}

func TestMuxConn_Run_ProcessesInboundRequestWhileAwaitingCallbackResponse(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var server *MuxConn
	server = NewMuxConn(serverConn, nil, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, _ uint32, body []byte) ([]byte, error) {
		if msgID != protocol.MsgInvokeRequest {
			t.Fatalf("server received %s, want InvokeRequest", protocol.MsgIDString(msgID))
		}

		responseMsgID, responseBody, err := server.Call(ctx, protocol.MsgStartTaskRequest, append([]byte("callback:"), body...))
		if err != nil {
			return nil, err
		}
		if responseMsgID != protocol.MsgStartTaskResponse {
			return nil, fmt.Errorf("callback response message = %s, want %s", protocol.MsgIDString(responseMsgID), protocol.MsgIDString(protocol.MsgStartTaskResponse))
		}
		return responseBody, nil
	}))

	client := NewMuxConn(clientConn, nil, transportcore.HandlerFunc(func(_ context.Context, msgID uint32, _ uint32, body []byte) ([]byte, error) {
		if msgID != protocol.MsgStartTaskRequest {
			t.Fatalf("client received %s, want StartTaskRequest", protocol.MsgIDString(msgID))
		}
		return append([]byte("handled:"), body...), nil
	}))
	defer server.Close()
	defer client.Close()

	serverDone := make(chan error, 1)
	clientDone := make(chan error, 1)
	go func() { serverDone <- server.Run(ctx) }()
	go func() { clientDone <- client.Run(ctx) }()

	responseMsgID, responseBody, err := client.Call(ctx, protocol.MsgInvokeRequest, []byte("request"))
	if err != nil {
		t.Fatalf("invoke over bidirectional connection: %v", err)
	}
	if responseMsgID != protocol.MsgInvokeResponse {
		t.Fatalf("response message = %s, want %s", protocol.MsgIDString(responseMsgID), protocol.MsgIDString(protocol.MsgInvokeResponse))
	}
	if got, want := string(responseBody), "handled:callback:request"; got != want {
		t.Fatalf("response body = %q, want %q", got, want)
	}

	_ = server.Close()
	_ = client.Close()
	<-serverDone
	<-clientDone
}
