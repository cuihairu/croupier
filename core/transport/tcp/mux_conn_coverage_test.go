package tcp

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/pkg/protocol"
)

func TestMuxConnDispatch_ControlLaneCtxDoneMigration(t *testing.T) {
	mc := &MuxConn{
		ctrlInbox: make(chan muxTask),
		bizInbox:  make(chan muxTask),
		closed:    make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := mc.dispatchInbound(ctx, protocol.MsgHeartbeatRequest, 1, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestMuxConnDispatch_ControlLaneClosedMigration(t *testing.T) {
	mc := &MuxConn{
		ctrlInbox: make(chan muxTask),
		bizInbox:  make(chan muxTask),
		closed:    make(chan struct{}),
	}
	close(mc.closed)
	if err := mc.dispatchInbound(context.Background(), protocol.MsgHeartbeatRequest, 1, nil); err != nil {
		t.Fatalf("expected nil on closed conn, got %v", err)
	}
}

func TestMuxConnDispatch_BusyWriteErrorMigration(t *testing.T) {
	conn, _ := net.Pipe()
	if err := conn.Close(); err != nil {
		t.Fatalf("close conn: %v", err)
	}
	mc := &MuxConn{
		ctrlInbox: make(chan muxTask),
		bizInbox:  make(chan muxTask, 1),
		closed:    make(chan struct{}),
		conn:      conn,
	}
	mc.bizInbox <- muxTask{}
	if err := mc.dispatchInbound(context.Background(), protocol.MsgInvokeRequest, 1, []byte(`{}`)); err == nil {
		t.Fatal("expected busy-frame write error on closed conn")
	}
}

func TestMuxConnRun_EventHandlerProtocolErrorMigration(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
	mc := NewMuxConn(c1, nil, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return nil, NewProtocolError(errors.New("event violation"))
	}))
	defer func() { _ = mc.Close() }()

	errCh := make(chan error, 1)
	go func() { errCh <- mc.Run(context.Background()) }()
	if err := WriteFrame(c2, protocol.NewMessageBody(protocol.MsgTaskEvent, 0, []byte(`{}`))); err != nil {
		t.Fatalf("write event: %v", err)
	}
	select {
	case err := <-errCh:
		if err == nil || !isProtocolError(err) {
			t.Fatalf("expected protocol error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not terminate on event protocol error")
	}
}

type writeFailConn struct{ net.Conn }

func (writeFailConn) Write([]byte) (int, error) { return 0, errors.New("write boom") }

func TestMuxConnRun_DispatchNonTimeoutErrorMigration(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
	release := make(chan struct{})
	defer close(release)
	mc := NewMuxConn(writeFailConn{c1}, &Config{DispatchWorkers: 1, BusinessQLen: 1}, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		<-release
		return body, nil
	}))
	defer func() { _ = mc.Close() }()

	errCh := make(chan error, 1)
	go func() { errCh <- mc.Run(context.Background()) }()
	peer := &muxTestPeer{conn: c2, t: t}
	peer.sendRequest(protocol.MsgInvokeRequest, 1, []byte(`{}`))
	time.Sleep(50 * time.Millisecond)
	peer.sendRequest(protocol.MsgInvokeRequest, 2, []byte(`{}`))
	peer.sendRequest(protocol.MsgInvokeRequest, 3, []byte(`{}`))
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected dispatch write error to terminate Run")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not terminate")
	}
}

func TestMuxConnRun_DispatchTimeoutErrorMigration(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
	release := make(chan struct{})
	defer close(release)
	mc := NewMuxConn(c1, &Config{DispatchWorkers: 1, BusinessQLen: 1, ControlQLen: 1}, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		<-release
		return body, nil
	}))
	defer func() { _ = mc.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- mc.Run(ctx) }()
	peer := &muxTestPeer{conn: c2, t: t}
	peer.sendRequest(protocol.MsgHeartbeatRequest, 1, nil)
	time.Sleep(50 * time.Millisecond)
	peer.sendRequest(protocol.MsgHeartbeatRequest, 2, nil)
	peer.sendRequest(protocol.MsgHeartbeatRequest, 3, nil)
	select {
	case err := <-errCh:
		if err == nil || !isTimeout(err) {
			t.Fatalf("expected timeout error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not terminate on control-lane ctx timeout")
	}
}

func TestMuxConnIsClosed_NilReceiverMigration(t *testing.T) {
	var mc *MuxConn
	if !mc.IsClosed() {
		t.Fatal("nil MuxConn should report closed")
	}
}

func TestMuxConnWaitReturnsAfterRunMigration(t *testing.T) {
	c1, c2 := net.Pipe()
	mc := NewMuxConn(c1, &Config{RecvTimeout: 100 * time.Millisecond}, nil)
	_ = c2.Close()
	_ = mc.Run(context.Background())
	done := make(chan struct{})
	go func() {
		mc.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return after Run terminated")
	}
}

func TestMuxConnCloseFailsOccupiedPendingChannelMigration(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
	mc := NewMuxConn(c1, nil, nil)
	respCh := make(chan muxResponse, 1)
	mc.pendingMu.Lock()
	mc.pending[4242] = respCh
	mc.pendingMu.Unlock()
	mc.fulfillPending(4242, muxResponse{msgID: protocol.MsgInvokeResponse, body: []byte("fulfilled")})
	if err := mc.Close(); err != nil {
		t.Fatalf("unexpected Close error: %v", err)
	}
	select {
	case resp := <-respCh:
		if resp.msgID != protocol.MsgInvokeResponse || string(resp.body) != "fulfilled" {
			t.Fatalf("unexpected buffered response: msgID=%#x body=%q", resp.msgID, resp.body)
		}
	default:
		t.Fatal("expected the fulfilled response to remain buffered")
	}
}

func TestMuxConnHandleInboundRequestMigration(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
	mc := NewMuxConn(c1, nil, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return []byte("ok"), nil
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
	if msgID != protocol.MsgHeartbeatResponse || reqID != 3 || string(body) != "ok" {
		t.Fatalf("msgID=%x reqID=%d body=%q", msgID, reqID, body)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("handleInboundRequest: %v", err)
	}
}

func TestMuxConnLaneWorkerClosesOnWriteFailureMigration(t *testing.T) {
	c1, c2 := net.Pipe()
	_ = c2.Close()
	mc := NewMuxConn(c1, nil, transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return []byte("ok"), nil
	}))
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

func TestMuxConnCall_WriteFailureMigration(t *testing.T) {
	conn, _ := net.Pipe()
	if err := conn.Close(); err != nil {
		t.Fatalf("close conn: %v", err)
	}
	mc := NewMuxConn(conn, nil, nil)
	if _, _, err := mc.Call(context.Background(), protocol.MsgInvokeRequest, []byte(`{}`)); err == nil {
		t.Fatal("expected write failure")
	}
}

func TestMuxConnCall_ContextCanceledWhileWaitingMigration(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
	mc := NewMuxConn(c1, nil, nil)
	defer func() { _ = mc.Close() }()
	go func() {
		_, _ = ReadFrame(c2)
		<-time.After(2 * time.Second)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, _, err := mc.Call(ctx, protocol.MsgInvokeRequest, []byte(`{}`)); err == nil {
		t.Fatal("expected context deadline error")
	}
}

func TestMuxConnSend_ContextCanceledMigration(t *testing.T) {
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

func TestMuxConnWriteFrame_SendTimeoutMigration(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()
	mc := NewMuxConn(c1, &Config{SendTimeout: 100 * time.Millisecond}, nil)
	defer func() { _ = mc.Close() }()
	if err := mc.writeFrame(1, protocol.MsgTaskEvent, []byte(`{}`)); err == nil {
		t.Fatal("expected write timeout")
	}
}
