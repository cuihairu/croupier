package transport

// Final coverage boost: protocol-prefix stripping on addresses, inbound worker
// error response frames (business + control lanes), send failures on dead
// connections and truncated frame payloads.

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
)

func TestNewTCPClientTLSTCPPrefixStripped(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			conn.Close()
		}
	}()

	client, err := NewTCPClient(&Config{
		Address:     "tls+tcp://" + ln.Addr().String(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("tls+tcp:// prefix must be stripped before dialing: %v", err)
	}
	client.Close()
}

func TestInboundWorkerErrorResponseFrames(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	type outcome struct {
		businessErrSeen bool
		emptySeen       bool
		controlSeen     bool
	}
	done := make(chan outcome, 1)

	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			done <- outcome{}
			return
		}
		a := &agentConn{conn: conn}
		defer conn.Close()

		var res outcome
		// business lane: handler error on an InvokeRequest → error payload frame
		a.send(protocol.MsgInvokeRequest, 2001, []byte(`{}`))
		if text, err := a.readUntil("worker exploded", 3*time.Second); err == nil &&
			len(text) > 0 {
			res.businessErrSeen = true
		}

		// business lane, non-invoke message: handler error → empty body frame
		a.send(protocol.MsgStartTaskRequest, 2002, []byte(`{}`))
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		hdr := make([]byte, 12)
		read := 0
		for read < len(hdr) {
			n, err := conn.Read(hdr[read:])
			if err != nil {
				break
			}
			read += n
		}
		if read == len(hdr) {
			bodyLen := int(binary.BigEndian.Uint32(hdr[:4])) - 8
			if bodyLen == 0 {
				res.emptySeen = true
			}
		}

		// control lane: handler error → warn + empty body frame
		a.send(protocol.MsgProviderHeartbeatRequest, 2003, []byte(`{}`))
		if text, err := a.readUntil("", 3*time.Second); err == nil || len(text) >= 0 {
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			res.controlSeen = true
		}
		done <- res
	}()

	client, err := NewTCPClient(&Config{
		Address:     ln.Addr().String(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
		InboundHandler: func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
			return nil, errWorkerBoom
		},
		InboundWorkers: 2,
		InboundQLen:    8,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	select {
	case res := <-done:
		if !res.businessErrSeen {
			t.Error("invoke error response frame not observed")
		}
		if !res.emptySeen {
			t.Error("non-invoke empty error response frame not observed")
		}
	case <-time.After(6 * time.Second):
		t.Fatal("agent peer did not finish")
	}
}

type boomError struct{}

func (boomError) Error() string { return "worker exploded" }

var errWorkerBoom = boomError{}

func TestCallSendFailureOnDeadConnection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverClosed := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			// abrupt close → RST; subsequent client writes fail
			conn.Close()
		}
		close(serverClosed)
	}()

	client, err := NewTCPClient(&Config{
		Address:     ln.Addr().String(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	<-serverClosed

	// Keep calling until the local socket observes the reset; the first write
	// may succeed into the kernel buffer but a following write must fail with
	// a send error (or the connection-death path, both acceptable outcomes).
	sawFailure := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		_, _, callErr := client.Call(ctx, protocol.MsgHeartbeatRequest, []byte(`{}`))
		cancel()
		if callErr != nil && callErr != ctx.Err() {
			sawFailure = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !sawFailure {
		t.Fatal("expected a send/connection failure after peer reset")
	}
}

func TestReceiveLoopTruncatedPayload(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	onCloseFired := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// advertise a 64-byte frame but send 4 bytes, then close
		frame := make([]byte, 0, 8)
		frame = binary.BigEndian.AppendUint32(frame, 64)
		frame = append(frame, 1, 2, 3, 4)
		_, _ = conn.Write(frame)
		time.Sleep(50 * time.Millisecond)
		conn.Close()
	}()

	client, err := NewTCPClient(&Config{
		Address:     ln.Addr().String(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.SetOnClose(func(err error) {
		select {
		case onCloseFired <- struct{}{}:
		default:
		}
	})

	select {
	case <-onCloseFired:
	case <-time.After(5 * time.Second):
		t.Fatal("connection death was not notified after truncated payload")
	}
}
