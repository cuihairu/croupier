package tcp

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// AUD-101: mixed N/N-1 version negotiation on the shared MuxConn (agent
// and control sessions ride it). The default window is [1,1] today, so
// the N/N-1 matrix runs through an injected window — the production
// default is covered by every other test.

// muxVersionPeer drives the MuxConn peer side with explicit frame
// versions (net.Pipe is synchronous: reads must run concurrently).
type muxVersionPeer struct {
	conn net.Conn
	t    *testing.T
}

func (p *muxVersionPeer) sendVersioned(version uint8, msgID uint32, reqID uint32, body []byte) {
	payload := protocol.NewMessageBodyWithVersion(version, msgID, reqID, body)
	frame := make([]byte, 4+len(payload))
	copy(frame, []byte{0, 0, 0, byte(len(payload))})
	putUint32(frame[:4], uint32(len(payload)))
	copy(frame[4:], payload)
	if _, err := p.conn.Write(frame); err != nil {
		p.t.Fatalf("write v%d frame: %v", version, err)
	}
}

func putUint32(b []byte, v uint32) {
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
}

func (p *muxVersionPeer) readVersioned() (version uint8, msgID uint32, reqID uint32, body []byte) {
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(p.conn, hdr); err != nil {
		p.t.Fatalf("read frame header: %v", err)
	}
	n := uint32(hdr[0])<<24 | uint32(hdr[1])<<16 | uint32(hdr[2])<<8 | uint32(hdr[3])
	buf := make([]byte, n)
	if _, err := io.ReadFull(p.conn, buf); err != nil {
		p.t.Fatalf("read frame body: %v", err)
	}
	version, msgID, reqID, body, err := protocol.ParseMessageFromBody(buf)
	if err != nil {
		p.t.Fatalf("parse frame: %v", err)
	}
	return version, msgID, reqID, body
}

func versionEchoHandler() transportcore.Handler {
	return transportcore.HandlerFunc(func(_ context.Context, msgID uint32, _ uint32, body []byte) ([]byte, error) {
		if protocol.IsRequest(msgID) {
			return body, nil
		}
		return nil, nil
	})
}

func TestMuxConnNegotiation_ResponseCarriesNegotiatedVersion(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	mc := NewMuxConn(c1, nil, versionEchoHandler())
	mc.negotiation = protocol.NewNegotiationWindow(1, 2)
	go func() { _ = mc.Run(context.Background()) }()
	defer func() { _ = mc.Close() }()

	peer := &muxVersionPeer{conn: c2, t: t}
	peer.sendVersioned(2, protocol.MsgInvokeRequest, 0x21, []byte(`{"a":1}`))

	version, msgID, reqID, body := peer.readVersioned()
	if version != 2 || msgID != protocol.MsgInvokeResponse || reqID != 0x21 || !bytes.Equal(body, []byte(`{"a":1}`)) {
		t.Fatalf("v2 request: response version %d msg %s req %d body %q — must lock 2 and answer in 2", version, protocol.MsgIDString(msgID), reqID, body)
	}
}

func TestMuxConnNegotiation_N1PeerLocksDown(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	mc := NewMuxConn(c1, nil, versionEchoHandler())
	mc.negotiation = protocol.NewNegotiationWindow(1, 2)
	go func() { _ = mc.Run(context.Background()) }()
	defer func() { _ = mc.Close() }()

	peer := &muxVersionPeer{conn: c2, t: t}
	peer.sendVersioned(1, protocol.MsgInvokeRequest, 0x22, nil)

	version, _, _, _ := peer.readVersioned()
	if version != 1 {
		t.Fatalf("v1 request got response version %d — must answer in the peer's version", version)
	}
}

func TestMuxConnNegotiation_ConflictingVersionRejected(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	mc := NewMuxConn(c1, nil, versionEchoHandler())
	mc.negotiation = protocol.NewNegotiationWindow(1, 2)
	go func() { _ = mc.Run(context.Background()) }()
	defer func() { _ = mc.Close() }()

	peer := &muxVersionPeer{conn: c2, t: t}
	peer.sendVersioned(2, protocol.MsgInvokeRequest, 0x31, nil)
	if version, _, _, _ := peer.readVersioned(); version != 2 {
		t.Fatalf("lock response version %d, want 2", version)
	}

	// Deviating in-window frame: rejected with the offered-version stamp.
	peer.sendVersioned(1, protocol.MsgInvokeRequest, 0x32, nil)
	version, msgID, reqID, body := peer.readVersioned()
	if version != 1 || msgID != protocol.MsgInvokeResponse || reqID != 0x32 {
		t.Fatalf("conflict rejection header version %d msg %s req %d", version, protocol.MsgIDString(msgID), reqID)
	}
	var rejection sdkv1.InvokeResponse
	if err := proto.Unmarshal(body, &rejection); err != nil {
		t.Fatalf("unmarshal rejection: %v", err)
	}
	if len(rejection.GetPayload()) == 0 {
		t.Fatalf("conflict rejection must carry an error payload")
	}

	// Session still locked at 2.
	peer.sendVersioned(2, protocol.MsgInvokeRequest, 0x33, nil)
	if version, _, _, _ := peer.readVersioned(); version != 2 {
		t.Fatalf("post-conflict response version %d — session must stay usable at 2", version)
	}
}

func TestMuxConnNegotiation_NoIntersectionRejectsThenLocksOnSupported(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	mc := NewMuxConn(c1, nil, versionEchoHandler()) // default window [1,1]
	go func() { _ = mc.Run(context.Background()) }()
	defer func() { _ = mc.Close() }()

	peer := &muxVersionPeer{conn: c2, t: t}
	// N-only peer on [1,1]: no intersection → rejection stamped with the
	// peer's own version so the old peer can read it.
	peer.sendVersioned(protocol.Version1+1, protocol.MsgInvokeRequest, 0x41, nil)
	version, msgID, reqID, body := peer.readVersioned()
	if version != protocol.Version1+1 || msgID != protocol.MsgInvokeResponse || reqID != 0x41 {
		t.Fatalf("rejection header version %d msg %s req %d", version, protocol.MsgIDString(msgID), reqID)
	}
	var rejection sdkv1.InvokeResponse
	if err := proto.Unmarshal(body, &rejection); err != nil {
		t.Fatalf("unmarshal rejection: %v", err)
	}
	if len(rejection.GetPayload()) == 0 {
		t.Fatalf("rejection must carry an error payload")
	}

	// Same connection locks when a supported frame arrives (N-1 compat).
	peer.sendVersioned(protocol.Version1, protocol.MsgInvokeRequest, 0x42, nil)
	if version, _, _, _ := peer.readVersioned(); version != protocol.Version1 {
		t.Fatalf("post-rejection response version %d — first supported frame locks", version)
	}
}

func TestMuxConnNegotiation_InitiatorStampsUnlockedVersion(t *testing.T) {
	// The initiator (agent/SDK dialer) pre-lock outbound frames carry the
	// local maximum — today Version1. At the next bump a widened window
	// automatically stamps the higher offer.
	defaultMux := NewMuxConn(nil, nil, nil)
	if got := defaultMux.negotiation.Version(); got != protocol.Version1 {
		t.Fatalf("unlocked default mux stamps version %d, want Version1", got)
	}
}

func TestMuxConnNegotiation_ReconnectNegotiatesFresh(t *testing.T) {
	var locked atomic.Int32
	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	mc := NewMuxConn(c1, nil, versionEchoHandler())
	mc.negotiation = protocol.NewNegotiationWindow(1, 2)
	go func() { _ = mc.Run(context.Background()) }()

	peer := &muxVersionPeer{conn: c2, t: t}
	peer.sendVersioned(2, protocol.MsgInvokeRequest, 0x51, nil)
	if version, _, _, _ := peer.readVersioned(); version != 2 {
		t.Fatalf("first connection locked at %d, want 2", version)
	}
	locked.Store(1)

	_ = mc.Close()
	// A new connection on the same peer pair negotiates from scratch:
	c1b, c2b := net.Pipe()
	defer func() { _ = c1b.Close() }()
	defer func() { _ = c2b.Close() }()

	mc2 := NewMuxConn(c1b, nil, versionEchoHandler())
	mc2.negotiation = protocol.NewNegotiationWindow(1, 2)
	go func() { _ = mc2.Run(context.Background()) }()
	defer func() { _ = mc2.Close() }()

	peer2 := &muxVersionPeer{conn: c2b, t: t}
	peer2.sendVersioned(1, protocol.MsgInvokeRequest, 0x52, nil)
	if version, _, _, _ := peer2.readVersioned(); version != 1 {
		t.Fatalf("reconnect locked at %d, want fresh negotiation to 1", version)
	}
	if locked.Load() != 1 {
		t.Fatalf("precondition: first connection must have locked")
	}
}

var _ = time.Second
