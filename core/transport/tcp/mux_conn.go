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
	"github.com/cuihairu/croupier/pkg/protocol"
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
	// negotiation locks the per-connection frame version (AUD-101): the
	// first supported inbound frame wins, all outbound frames carry it.
	negotiation *protocol.Negotiation

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
		negotiation: protocol.NewNegotiation(),
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
		// 统一编码（AUD-002）：busy 响应经 RPCFailureBody 生成——有失败通道的
		// RPC 携带 `Success:false`，无通道类型返回 nil（诚实边界见 rpc_error.go）。
		respBody := RPCFailureBody(msgID, "inbound queue full, retry on another instance")
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
		if !c.negotiation.Accepts(version) {
			// Reject the request with an explicit error frame and keep the
			// session alive: a peer running an unsupported wire version must
			// not silently kill the whole connection (N-1 compatibility).
			if protocol.IsRequest(msgID) {
				// 统一编码（AUD-002）：版本拒绝文档经标准编码器生成，沿用
				// InvokeResponse 载体（SDK 读取 payload 并报告范围）。
				// 拒绝帧以对端提供的版本回写（AUD-101）：对端只认自己的
				// 版本，以本端默认版本回写会被旧对端静默丢弃。
				respBody := InvokeVersionRejectedBody(int(version), protocol.SupportedVersions())
				if err := c.writeFrameVersion(reqID, protocol.GetResponseMsgID(msgID), respBody, version); err != nil {
					return err
				}
			}
			slog.Warn("rejected unsupported protocol version", "version", version, "msg_id", protocol.MsgIDString(msgID))
			continue
		}

		// AUD-101：首条受支持帧锁定本连接的协商版本；此后偏离已锁版本的
		// 帧一律按不支持处理（粘滞）——会话中途换版本会破坏 reqID 配对
		// 与"响应必须用请求的版本"语义。
		if _, err := c.negotiation.Offer(version); err != nil {
			if protocol.IsRequest(msgID) {
				respBody := InvokeVersionRejectedBody(int(version), protocol.SupportedVersions())
				if err := c.writeFrameVersion(reqID, protocol.GetResponseMsgID(msgID), respBody, version); err != nil {
					return err
				}
			}
			slog.Warn("rejected conflicting frame version", "version", version,
				"negotiated", c.negotiation.Version(), "msg_id", protocol.MsgIDString(msgID))
			continue
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
		// 统一编码（AUD-002）：错误文本经标准编码器，响应携带该 RPC 的
		// 失败通道（无通道类型返回 nil，诚实边界见 rpc_error.go）。
		respBody = RPCFailureBody(msgID, err.Error())
	}

	return c.writeFrame(reqID, protocol.GetResponseMsgID(msgID), respBody)
}

func (c *MuxConn) writeFrame(reqID uint32, msgID uint32, body []byte) error {
	return c.writeFrameVersion(reqID, msgID, body, c.negotiation.Version())
}

// writeFrameVersion stamps an explicit frame version (AUD-101): normal
// traffic carries the negotiated version; pre-lock rejection replies carry
// the peer's own offered version so an older peer can still read them.
func (c *MuxConn) writeFrameVersion(reqID uint32, msgID uint32, body []byte, version uint8) error {
	payload := protocol.NewMessageBodyWithVersion(version, msgID, reqID, body)

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
