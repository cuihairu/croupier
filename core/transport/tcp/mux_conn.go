package tcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// ProtocolError marks a request as a protocol violation and forces the connection to close.
type ProtocolError struct {
	Err error
}

func (e *ProtocolError) Error() string {
	if e == nil || e.Err == nil {
		return "protocol violation"
	}
	return e.Err.Error()
}

// NewProtocolError wraps an error as a protocol violation.
func NewProtocolError(err error) error {
	if err == nil {
		err = fmt.Errorf("protocol violation")
	}
	return &ProtocolError{Err: err}
}

func isProtocolError(err error) bool {
	var protocolErr *ProtocolError
	return errors.As(err, &protocolErr)
}

type muxResponse struct {
	msgID uint32
	body  []byte
	err   error
}

// MuxConn multiplexes bidirectional request/response messages on a single TCP connection.
type MuxConn struct {
	conn        net.Conn
	handler     transportcore.Handler
	recvTimeout time.Duration
	sendTimeout time.Duration

	ctrlInbox chan muxTask
	bizInbox  chan muxTask
	workersWg sync.WaitGroup

	writeMu   sync.Mutex
	pendingMu sync.Mutex
	pending   map[uint32]chan muxResponse
	nextReqID atomic.Uint32

	closeOnce sync.Once
	closed    chan struct{}
}

type muxTask struct {
	msgID uint32
	reqID uint32
	body  []byte
}

// NewMuxConn creates a new bidirectional framed connection.
func NewMuxConn(conn net.Conn, config *Config, handler transportcore.Handler) *MuxConn {
	if config == nil {
		config = &Config{}
	}

	workers := config.DispatchWorkers
	if workers <= 0 {
		workers = max(runtime.NumCPU(), 2)
	}
	bizQLen := config.BusinessQLen
	if bizQLen <= 0 {
		bizQLen = workers * 4
	}
	ctrlQLen := config.ControlQLen
	if ctrlQLen <= 0 {
		ctrlQLen = 64
	}

	m := &MuxConn{
		conn:        conn,
		handler:     handler,
		recvTimeout: config.RecvTimeout,
		sendTimeout: config.SendTimeout,
		ctrlInbox:   make(chan muxTask, ctrlQLen),
		bizInbox:    make(chan muxTask, bizQLen),
		pending:     make(map[uint32]chan muxResponse),
		closed:      make(chan struct{}),
	}
	m.nextReqID.Store(1)

	m.workersWg.Add(1)
	go m.laneWorker(m.ctrlInbox)
	for i := 0; i < workers; i++ {
		m.workersWg.Add(1)
		go m.laneWorker(m.bizInbox)
	}

	return m
}

func (c *MuxConn) laneWorker(inbox chan muxTask) {
	defer c.workersWg.Done()
	for {
		select {
		case <-c.closed:
			return
		case task := <-inbox:
			if err := c.handleInboundRequest(context.Background(), task.msgID, task.reqID, task.body); err != nil {
				_ = c.Close()
				return
			}
		}
	}
}

func (c *MuxConn) dispatchInbound(ctx context.Context, msgID uint32, reqID uint32, body []byte) error {
	task := muxTask{msgID: msgID, reqID: reqID, body: body}

	if protocol.IsControlRequest(msgID) {
		select {
		case c.ctrlInbox <- task:
			return nil
		case <-c.closed:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	select {
	case c.bizInbox <- task:
	default:
		var respBody []byte
		if msgID == protocol.MsgInvokeRequest {
			resp := &sdkv1.InvokeResponse{
				Payload: []byte(`{"error":"inbound queue full, retry on another instance"}`),
			}
			if marshaled, err := proto.Marshal(resp); err == nil {
				respBody = marshaled
			}
		}
		if err := c.writeFrame(reqID, protocol.GetResponseMsgID(msgID), respBody); err != nil {
			return err
		}
	}
	return nil
}

// RemoteAddr returns the remote peer address string.
func (c *MuxConn) RemoteAddr() string {
	if c == nil || c.conn == nil || c.conn.RemoteAddr() == nil {
		return ""
	}
	return c.conn.RemoteAddr().String()
}

// LocalAddr returns the local bound address string.
func (c *MuxConn) LocalAddr() string {
	if c == nil || c.conn == nil || c.conn.LocalAddr() == nil {
		return ""
	}
	return c.conn.LocalAddr().String()
}

// Run starts the read loop and blocks until the connection closes or ctx is done.
func (c *MuxConn) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	defer func() { _ = c.Close() }()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			if c.conn != nil {
				_ = c.conn.SetReadDeadline(time.Now())
			}
		case <-c.closed:
		case <-done:
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.closed:
			return nil
		default:
		}

		if c.recvTimeout > 0 {
			_ = c.conn.SetReadDeadline(time.Now().Add(c.recvTimeout))
		}

		frame, err := readFrame(c.conn)
		if err != nil {
			if isTimeout(err) {
				continue
			}
			return err
		}

		version, msgID, reqID, body, err := protocol.ParseMessageFromBody(frame)
		if err != nil {
			return err
		}
		if version != protocol.Version1 {
			return NewProtocolError(fmt.Errorf("unsupported protocol version: %d", version))
		}

		if protocol.IsResponse(msgID) {
			c.fulfillPending(reqID, muxResponse{msgID: msgID, body: body})
			continue
		}

		if protocol.IsEvent(msgID) {
			if c.handler == nil {
				continue
			}
			if _, err := c.handler.Handle(ctx, msgID, 0, body); err != nil {
				if isProtocolError(err) {
					return err
				}
			}
			continue
		}

		if c.handler == nil {
			return NewProtocolError(fmt.Errorf("no request handler configured for %s", protocol.MsgIDString(msgID)))
		}

		if err := c.dispatchInbound(ctx, msgID, reqID, body); err != nil {
			return err
		}
	}
}

// Send writes a one-way event frame on the connection.
func (c *MuxConn) Send(ctx context.Context, msgID uint32, body []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.closed:
		return fmt.Errorf("connection closed")
	default:
	}

	if !protocol.IsEvent(msgID) {
		return fmt.Errorf("message %s is not an event", protocol.MsgIDString(msgID))
	}
	return c.writeFrame(0, msgID, body)
}

var callPrecheckDone func()

// Call sends a request and waits for the matching response on the same connection.
func (c *MuxConn) Call(ctx context.Context, msgID uint32, reqBody []byte) (respMsgID uint32, respBody []byte, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-c.closed:
		return 0, nil, fmt.Errorf("connection closed")
	default:
	}
	if callPrecheckDone != nil {
		callPrecheckDone()
	}

	reqID := c.nextReqID.Add(1)
	respCh := make(chan muxResponse, 1)

	c.pendingMu.Lock()
	c.pending[reqID] = respCh
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, reqID)
		c.pendingMu.Unlock()
	}()

	if err := c.writeFrame(reqID, msgID, reqBody); err != nil {
		return 0, nil, err
	}

	select {
	case <-ctx.Done():
		return 0, nil, ctx.Err()
	case <-c.closed:
		select {
		case resp := <-respCh:
			return resp.msgID, resp.body, resp.err
		default:
		}
		return 0, nil, fmt.Errorf("connection closed")
	case resp := <-respCh:
		return resp.msgID, resp.body, resp.err
	}
}

// Close closes the underlying connection and fails any pending requests.
func (c *MuxConn) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		close(c.closed)
		if c.conn != nil {
			closeErr = c.conn.Close()
		}
		c.failPending(fmt.Errorf("connection closed"))
	})
	return closeErr
}

// Wait blocks until all lane workers have exited.
func (c *MuxConn) Wait() {
	c.workersWg.Wait()
}

// IsClosed reports whether the mux connection has been closed.
func (c *MuxConn) IsClosed() bool {
	if c == nil || c.closed == nil {
		return true
	}
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

func (c *MuxConn) handleInboundRequest(ctx context.Context, msgID uint32, reqID uint32, body []byte) error {
	respBody, err := c.handler.Handle(ctx, msgID, reqID, body)
	if err != nil {
		if isProtocolError(err) {
			return err
		}
		slog.Error("rpc handler error", "msg_id", protocol.MsgIDString(msgID), "error", err)
		if msgID == protocol.MsgInvokeRequest {
			resp := &sdkv1.InvokeResponse{Payload: []byte(`{"error":"` + err.Error() + `"}`)}
			respBody, _ = proto.Marshal(resp)
		} else {
			respBody = nil
		}
	}

	return c.writeFrame(reqID, protocol.GetResponseMsgID(msgID), respBody)
}

func (c *MuxConn) writeFrame(reqID uint32, msgID uint32, body []byte) error {
	payload := protocol.NewMessageBody(msgID, reqID, body)

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.sendTimeout > 0 {
		_ = c.conn.SetWriteDeadline(time.Now().Add(c.sendTimeout))
		defer func() {
			_ = c.conn.SetWriteDeadline(time.Time{})
		}()
	}

	if err := writeFrame(c.conn, payload); err != nil {
		return fmt.Errorf("write frame: %w", err)
	}
	return nil
}

func (c *MuxConn) fulfillPending(reqID uint32, resp muxResponse) {
	c.pendingMu.Lock()
	ch, ok := c.pending[reqID]
	c.pendingMu.Unlock()
	if !ok {
		return
	}

	select {
	case ch <- resp:
	default:
	}
}

func (c *MuxConn) failPending(err error) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	for _, ch := range c.pending {
		select {
		case ch <- muxResponse{err: err}:
		default:
		}
	}
}

func isTimeout(err error) bool {
	type timeout interface {
		Timeout() bool
	}
	if err == nil {
		return false
	}
	if te, ok := err.(timeout); ok {
		return te.Timeout()
	}
	return false
}
