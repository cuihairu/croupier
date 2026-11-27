package tcp

import (
	"context"
	"net"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// AUD-101: per-connection frame version negotiation on the server read loop.
// The server accepts a window; the first supported frame locks the version,
// responses carry the negotiated version, deviating frames are rejected
// (sticky), and a reconnect negotiates from scratch. Tests rehearse the
// N/N-1 mixes via the negotiationWindow hook before Version1 moves.

func newWindowServer(t *testing.T, min, max uint8) (*Server, func()) {
	t.Helper()
	handler := transportcore.HandlerFunc(func(context.Context, uint32, uint32, []byte) ([]byte, error) {
		return []byte("ok"), nil
	})
	srv, err := NewServer(&Config{Address: "127.0.0.1:0", Insecure: true}, handler)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	srv.negotiationWindow = func() (uint8, uint8) { return min, max }
	return srv, serveAndStop(srv)
}

func dialAndRoundTrip(t *testing.T, addr string, requestVersion uint8, reqID uint32) (respVersion uint8, body []byte) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	frame := protocol.NewMessageBodyWithVersion(requestVersion, protocol.MsgInvokeRequest, reqID, nil)
	if err := tcptr.WriteFrame(conn, frame); err != nil {
		t.Fatalf("write request: %v", err)
	}
	respFrame, err := tcptr.ReadFrame(conn)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	version, msgID, gotReqID, body, err := protocol.ParseMessageFromBody(respFrame)
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if msgID != protocol.MsgInvokeResponse || gotReqID != reqID {
		t.Fatalf("response header msg %s req %d", protocol.MsgIDString(msgID), gotReqID)
	}
	return version, body
}

func rejectionOf(t *testing.T, body []byte) *sdkv1.InvokeResponse {
	t.Helper()
	var resp sdkv1.InvokeResponse
	if err := proto.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal rejection: %v", err)
	}
	return &resp
}

func TestServerNegotiation_ResponseCarriesNegotiatedVersion(t *testing.T) {
	srv, stop := newWindowServer(t, 1, 2)
	defer stop()

	if v, body := dialAndRoundTrip(t, srv.Addr(), 2, 0x21); v != 2 || string(body) != "ok" {
		t.Fatalf("v2 request: response version %d body %q — must lock 2 and answer in 2", v, body)
	}
}

func TestServerNegotiation_N1PeerLocksDown(t *testing.T) {
	srv, stop := newWindowServer(t, 1, 2)
	defer stop()

	if v, _ := dialAndRoundTrip(t, srv.Addr(), 1, 0x22); v != 1 {
		t.Fatalf("v1 request got response version %d — server must answer in the peer's version", v)
	}
}

func TestServerNegotiation_ConflictingVersionRejectedSticky(t *testing.T) {
	srv, stop := newWindowServer(t, 1, 2)
	defer stop()

	conn, err := net.DialTimeout("tcp", srv.Addr(), time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Lock the connection at 2.
	lock := protocol.NewMessageBodyWithVersion(2, protocol.MsgInvokeRequest, 0x31, nil)
	if err := tcptr.WriteFrame(conn, lock); err != nil {
		t.Fatalf("write lock request: %v", err)
	}
	resp, err := tcptr.ReadFrame(conn)
	if err != nil {
		t.Fatalf("read lock response: %v", err)
	}
	if v, _, _, _, _ := protocol.ParseMessageFromBody(resp); v != 2 {
		t.Fatalf("lock response version %d, want 2", v)
	}

	// A deviating in-window frame is rejected with the offered version stamp,
	// and the session stays locked at 2.
	deviant := protocol.NewMessageBodyWithVersion(1, protocol.MsgInvokeRequest, 0x32, nil)
	if err := tcptr.WriteFrame(conn, deviant); err != nil {
		t.Fatalf("write deviant request: %v", err)
	}
	rejFrame, err := tcptr.ReadFrame(conn)
	if err != nil {
		t.Fatalf("read deviant rejection: %v", err)
	}
	v, msgID, reqID, body, err := protocol.ParseMessageFromBody(rejFrame)
	if err != nil {
		t.Fatalf("parse deviant rejection: %v", err)
	}
	if v != 1 || msgID != protocol.MsgInvokeResponse || reqID != 0x32 {
		t.Fatalf("deviant rejection header version %d msg %s req %d", v, protocol.MsgIDString(msgID), reqID)
	}
	if rejectionOf(t, body).GetPayload() == nil {
		t.Fatalf("deviant rejection must carry an error payload")
	}

	// Still locked at 2: a compliant frame works again.
	again := protocol.NewMessageBodyWithVersion(2, protocol.MsgInvokeRequest, 0x33, nil)
	if err := tcptr.WriteFrame(conn, again); err != nil {
		t.Fatalf("write post-conflict request: %v", err)
	}
	resp2, err := tcptr.ReadFrame(conn)
	if err != nil {
		t.Fatalf("read post-conflict response: %v", err)
	}
	if v2, _, _, body2, _ := protocol.ParseMessageFromBody(resp2); v2 != 2 || string(body2) != "ok" {
		t.Fatalf("post-conflict response version %d body %q — session must stay usable at 2", v2, body2)
	}
}

func TestServerNegotiation_NoIntersectionRejectsThenAcceptsSupported(t *testing.T) {
	// Default window [1,1]: a v2-only peer has no intersection — its request
	// is rejected with its own version stamp, and a later v1 frame still
	// locks and works (session kept, N-1 compat).
	srv, stop := newWindowServer(t, protocol.MinSupportedVersion, protocol.Version1)
	defer stop()

	conn, err := net.DialTimeout("tcp", srv.Addr(), time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	unsupported := protocol.NewMessageBodyWithVersion(protocol.Version1+1, protocol.MsgInvokeRequest, 0x41, nil)
	if err := tcptr.WriteFrame(conn, unsupported); err != nil {
		t.Fatalf("write unsupported request: %v", err)
	}
	rejFrame, err := tcptr.ReadFrame(conn)
	if err != nil {
		t.Fatalf("read unsupported rejection: %v", err)
	}
	v, _, _, body, _ := protocol.ParseMessageFromBody(rejFrame)
	if v != protocol.Version1+1 {
		t.Fatalf("rejection version %d — must mirror the peer's offer so old peers can read it", v)
	}
	var resp sdkv1.InvokeResponse
	if err := proto.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal rejection: %v", err)
	}

	// Same connection: first supported frame locks 1 and traffic flows.
	supported := protocol.NewMessageBodyWithVersion(protocol.Version1, protocol.MsgInvokeRequest, 0x42, nil)
	if err := tcptr.WriteFrame(conn, supported); err != nil {
		t.Fatalf("write supported request: %v", err)
	}
	okFrame, err := tcptr.ReadFrame(conn)
	if err != nil {
		t.Fatalf("read ok response: %v", err)
	}
	if v2, _, _, body2, _ := protocol.ParseMessageFromBody(okFrame); v2 != protocol.Version1 || string(body2) != "ok" {
		t.Fatalf("post-rejection response version %d body %q", v2, body2)
	}
}

func TestServerNegotiation_ReconnectNegotiatesFresh(t *testing.T) {
	srv, stop := newWindowServer(t, 1, 2)
	defer stop()

	// Connection 1 locks at 1 (an N-1 peer).
	if v, _ := dialAndRoundTrip(t, srv.Addr(), 1, 0x51); v != 1 {
		t.Fatalf("first connection version %d, want 1", v)
	}
	// Connection 2 is a fresh negotiation and locks at 2 (an N peer).
	if v, _ := dialAndRoundTrip(t, srv.Addr(), 2, 0x52); v != 2 {
		t.Fatalf("reconnect version %d, want 2", v)
	}
}
