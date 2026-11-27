package transport

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

// AUD-101: mixed N/N-1 version negotiation on the SDK transport. The
// shipped window is [1,1]; the N/N-1 matrix runs through an injected
// per-connection window (the same rule the server rehearses with) before
// Version1 moves.

// versionAgent reads one SDK request frame and answers with a
// InvokeResponse stamped with the given version. It reports the observed
// request version on the channel.
func versionAgent(t *testing.T, replyVersion uint8, observed chan<- uint8) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		hdr := make([]byte, 4)
		if _, err := io.ReadFull(conn, hdr); err != nil {
			return
		}
		n := binary.BigEndian.Uint32(hdr)
		payload := make([]byte, n)
		if _, err := io.ReadFull(conn, payload); err != nil {
			return
		}
		version, _, reqID, _, err := protocol.ParseMessageFromBody(payload)
		if err != nil {
			return
		}
		observed <- version

		resp, err := proto.Marshal(&sdkv1.InvokeResponse{})
		if err != nil {
			return
		}
		body := protocol.NewMessageBodyWithVersion(replyVersion, protocol.MsgInvokeResponse, reqID, resp)
		frame := make([]byte, 4+len(body))
		binary.BigEndian.PutUint32(frame, uint32(len(body)))
		copy(frame[4:], body)
		_, _ = conn.Write(frame)
	}()
	return listener
}

func dialVersionClient(t *testing.T, addr string) *TCPClient {
	t.Helper()
	client, err := NewTCPClient(&Config{Address: addr, DialTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewTCPClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestTCPClientNegotiation_DefaultStampsV1AndLocksOnV1Reply(t *testing.T) {
	observed := make(chan uint8, 4)
	listener := versionAgent(t, protocol.Version1, observed)
	defer func() { _ = listener.Close() }()

	client := dialVersionClient(t, listener.Addr().String())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := client.Call(ctx, protocol.MsgInvokeRequest, []byte(`{}`)); err != nil {
		t.Fatalf("call: %v", err)
	}
	select {
	case v := <-observed:
		if v != protocol.Version1 {
			t.Fatalf("unlocked default client stamped version %d, want Version1", v)
		}
	default:
		t.Fatalf("agent observed no request version")
	}
	if client.negotiation.Version() != protocol.Version1 {
		t.Fatalf("negotiated version %d, want locked at Version1", client.negotiation.Version())
	}
}

func TestTCPClientNegotiation_WidenedWindowLocksUpAndKeepsStamps(t *testing.T) {
	observed := make(chan uint8, 4)
	listener := versionAgent(t, 2, observed)
	defer func() { _ = listener.Close() }()

	client := dialVersionClient(t, listener.Addr().String())
	client.negotiation = protocol.NewNegotiationWindow(1, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := client.Call(ctx, protocol.MsgInvokeRequest, []byte(`{}`)); err != nil {
		t.Fatalf("first call: %v", err)
	}
	select {
	case v := <-observed:
		if v != 2 {
			t.Fatalf("unlocked widened client stamped version %d, want its local max 2", v)
		}
	default:
		t.Fatalf("agent observed no request version")
	}
	if client.negotiation.Version() != 2 {
		t.Fatalf("negotiated version %d, want locked at 2 after a v2 reply", client.negotiation.Version())
	}
}

func TestTCPClientNegotiation_NoIntersectionFailsTheCall(t *testing.T) {
	observed := make(chan uint8, 4)
	listener := versionAgent(t, protocol.Version1+1, observed) // N-only agent
	defer func() { _ = listener.Close() }()

	client := dialVersionClient(t, listener.Addr().String())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := client.Call(ctx, protocol.MsgInvokeRequest, []byte(`{}`))
	if err == nil {
		t.Fatalf("call must fail fast on an unsupported reply version instead of waiting for ctx")
	}
}

func TestTCPClientNegotiation_ConflictingReplyWakesCaller(t *testing.T) {
	observed := make(chan uint8, 4)
	listener := versionAgent(t, 1, observed) // deviates from the locked 2
	defer func() { _ = listener.Close() }()

	client := dialVersionClient(t, listener.Addr().String())
	client.negotiation = protocol.NewNegotiationWindow(1, 2)
	if _, err := client.negotiation.Offer(2); err != nil {
		t.Fatalf("pre-lock offer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := client.Call(ctx, protocol.MsgInvokeRequest, []byte(`{}`))
	if err == nil {
		t.Fatalf("call must fail with a version conflict instead of waiting for ctx")
	}
	if got := fmt.Sprint(err); got == "" {
		t.Fatalf("error must carry a message")
	}
}

func TestTCPClientNegotiation_ReconnectNegotiatesFresh(t *testing.T) {
	// Connection 1: widened client locks at 2.
	observed := make(chan uint8, 4)
	listener := versionAgent(t, 2, observed)
	defer func() { _ = listener.Close() }()

	client := dialVersionClient(t, listener.Addr().String())
	client.negotiation = protocol.NewNegotiationWindow(1, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := client.Call(ctx, protocol.MsgInvokeRequest, []byte(`{}`)); err != nil {
		t.Fatalf("first connection call: %v", err)
	}
	_ = client.Close()

	// A fresh client (fresh TCP connection) negotiates from scratch and
	// stamps the local maximum again — no inherited lock.
	observed2 := make(chan uint8, 4)
	listener2 := versionAgent(t, 2, observed2)
	defer func() { _ = listener2.Close() }()

	client2 := dialVersionClient(t, listener2.Addr().String())
	client2.negotiation = protocol.NewNegotiationWindow(1, 2)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	if _, _, err := client2.Call(ctx2, protocol.MsgInvokeRequest, []byte(`{}`)); err != nil {
		t.Fatalf("reconnect call: %v", err)
	}
	select {
	case v := <-observed2:
		if v != 2 {
			t.Fatalf("reconnect stamped version %d, want a fresh offer of 2", v)
		}
	default:
		t.Fatalf("agent observed no request version on reconnect")
	}
}
