// Package transport provides TCP transport layer for Croupier SDK.
package transport

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

const (
	frameHeaderBytes = 4
	maxFrameBytes    = 32 * 1024 * 1024 // 32 MB
)

// TCPClient represents a TCP transport client.
// It uses a single TCP connection with multiplexed request/response communication.
type TCPClient struct {
	conn       net.Conn
	config     *Config
	mu         sync.RWMutex
	pending    map[uint32]chan responseTuple
	nextReqID  uint32
	closing    chan struct{}
	once       sync.Once
	readLoopWg sync.WaitGroup
	onClose    func(err error)
	inbound    InboundHandler
	writeMu    sync.Mutex
	// dead 在 receiveLoop 退出（连接死亡）后置位；晚于 failAllPending
	// 到达的 Call 依赖它立即失败，否则只能等 ctx/deadline。
	dead bool

	// inbound 队列与固定 worker 池：读循环永不执行业务逻辑（串行处理
	// 会导致头部阻塞——一个慢 handler 卡住整条连接的所有请求）。
	inbox   chan inboundTask
	inboxWg sync.WaitGroup
}

// inboundTask 是一个待处理的 Agent 入站请求。
type inboundTask struct {
	msgID uint32
	reqID uint32
	body  []byte
}

type responseTuple struct {
	msgID uint32
	body  []byte
	// err 非空表示连接已死、响应永不可达（failAllPending 注入）。
	err error
}

var errConnectionClosed = errors.New("connection closed")

// NewTCPClient creates a new TCP client with the given configuration.
func NewTCPClient(config *Config) (*TCPClient, error) {
	// Parse host and port from address if not set
	host := config.Host
	port := config.Port
	if host == "" || port == 0 {
		// Parse from Address (expected format: "host:port" or "tcp://host:port")
		addr := config.Address
		// 前缀剥离：长度必须与字面量一致（"tls+tcp://" 是 10 字节，此前
		// 按 9 字节比较导致条件永假、前缀永不被剥离、拨号必然失败）。
		if strings.HasPrefix(addr, "tcp://") {
			addr = strings.TrimPrefix(addr, "tcp://")
		} else if strings.HasPrefix(addr, "tls+tcp://") {
			addr = strings.TrimPrefix(addr, "tls+tcp://")
		}

		// Validate address is not empty after stripping protocol prefix
		if addr == "" {
			return nil, errors.New("address cannot be empty")
		}

		// Parse host:port
		var err error
		host, port, err = parseHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("parse address %s: %w", config.Address, err)
		}
	}

	// Validate host is not empty
	if host == "" {
		return nil, errors.New("host cannot be empty")
	}

	// Use JoinHostPort to properly handle IPv6 addresses
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	// Create connection with timeout
	dialer := &net.Dialer{
		Timeout: config.DialTimeout,
	}

	// Try to connect
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	client := &TCPClient{
		conn:      conn,
		config:    config,
		pending:   make(map[uint32]chan responseTuple),
		nextReqID: 1,
		closing:   make(chan struct{}),
		inbound:   config.InboundHandler,
	}

	// 入站 worker 池：读循环只投递，业务处理由固定 worker 并发执行。
	// 有界队列满时立即回错误帧（Agent 侧 failover 接管），内存不积累。
	if config.InboundHandler != nil {
		workers := config.InboundWorkers
		if workers <= 0 {
			workers = 8
		}
		qlen := config.InboundQLen
		if qlen <= 0 {
			qlen = 32
		}
		client.inbox = make(chan inboundTask, qlen)
		client.inboxWg.Add(workers)
		for i := 0; i < workers; i++ {
			go client.inboundWorker()
		}
	}

	// Start receive loop
	client.readLoopWg.Add(1)
	go client.receiveLoop()

	return client, nil
}

// inboundWorker 消费入站队列：执行 handler 并回写响应帧。
// handler 错误仍回错误响应（原同步版语义，不吞错误）。
func (c *TCPClient) inboundWorker() {
	defer c.inboxWg.Done()
	for task := range c.inbox {
		respBody, err := c.inbound(context.Background(), task.msgID, task.reqID, task.body)
		if err != nil {
			// 失败也要答 Agent：否则对端阻塞到超时且无诊断。
			if task.msgID == protocol.MsgInvokeRequest {
				errResp := &sdkv1.InvokeResponse{
					Payload: []byte(`{"error":` + strconv.Quote(err.Error()) + `}`),
				}
				if marshaled, marshalErr := proto.Marshal(errResp); marshalErr == nil {
					respBody = marshaled
				} else {
					respBody = nil
				}
			} else {
				respBody = nil
			}
		}
		c.writeResponseFrame(task.msgID, task.reqID, respBody)
	}
}

// writeResponseFrame 以写锁保护回写响应帧（worker 并发安全）。
func (c *TCPClient) writeResponseFrame(msgID uint32, reqID uint32, respBody []byte) {
	frameBody := protocol.NewMessageBody(protocol.GetResponseMsgID(msgID), reqID, respBody)
	frame := make([]byte, frameHeaderBytes+len(frameBody))
	binary.BigEndian.PutUint32(frame[:frameHeaderBytes], uint32(len(frameBody)))
	copy(frame[frameHeaderBytes:], frameBody)
	c.writeMu.Lock()
	_, _ = c.conn.Write(frame)
	c.writeMu.Unlock()
}

// parseHostPort parses a host:port string.
func parseHostPort(addr string) (string, int, error) {
	// Simple parsing for "host:port" format
	host := addr
	port := 19090 // default

	// Split by last colon
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			host = addr[:i]
			portStr := addr[i+1:]
			p := 0
			_, err := fmt.Sscanf(portStr, "%d", &p)
			if err != nil {
				return "", 0, fmt.Errorf("parse port: %w", err)
			}
			port = p
			break
		}
	}

	// Handle IPv6 addresses (remove brackets if present)
	if len(host) > 0 && host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}

	return host, port, nil
}

// Call sends a request and waits for the response.
func (c *TCPClient) Call(ctx context.Context, msgID uint32, reqBody []byte) (respMsgID uint32, respBody []byte, err error) {
	// 连接已死（receiveLoop 已退出）时立即失败；不检查则本调用会在
	// failAllPending 之后注册 pending，再无人唤醒。
	c.mu.Lock()
	if c.dead {
		c.mu.Unlock()
		return 0, nil, errConnectionClosed
	}
	// Allocate request ID
	reqID := c.nextReqID
	c.nextReqID++

	// Create response channel
	respCh := make(chan responseTuple, 1)
	c.pending[reqID] = respCh
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, reqID)
		c.mu.Unlock()
	}()

	// Check payload size
	payloadSize := protocol.HeaderSize + len(reqBody)
	if payloadSize > maxFrameBytes {
		return 0, nil, fmt.Errorf("payload size %d exceeds maximum frame size %d", payloadSize, maxFrameBytes)
	}

	// Create frame with protocol header
	frame := make([]byte, frameHeaderBytes+payloadSize)

	// Frame length prefix (big-endian)
	binary.BigEndian.PutUint32(frame[0:4], uint32(payloadSize))

	// Protocol header
	frame[4] = protocol.Version1
	protocol.PutMsgID(frame[5:8], msgID)
	binary.BigEndian.PutUint32(frame[8:12], reqID)

	// Request body
	copy(frame[12:], reqBody)

	// Send frame
	c.writeMu.Lock()
	_, err = c.conn.Write(frame)
	c.writeMu.Unlock()
	if err != nil {
		return 0, nil, fmt.Errorf("send: %w", err)
	}

	// Wait for response
	select {
	case resp := <-respCh:
		if resp.err != nil {
			return 0, nil, resp.err
		}
		return resp.msgID, resp.body, nil
	case <-ctx.Done():
		return 0, nil, ctx.Err()
	case <-c.closing:
		return 0, nil, errors.New("client is closing")
	}
}

// SetOnClose sets a callback that is invoked when the connection is lost.
// The callback receives the error that caused the connection to close.
func (c *TCPClient) SetOnClose(fn func(err error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onClose = fn
}

// receiveLoop receives frames from the connection and routes them to pending requests.
func (c *TCPClient) receiveLoop() {
	defer c.readLoopWg.Done()
	// 连接死亡时所有在途 Call 的响应永远不会到达；必须显式失败，
	// 否则等待方阻塞到 ctx deadline（context.Background() 的调用方
	// 如 RegisterWithAgent 会永久挂起）。
	defer c.failAllPending()
	defer c.notifyClose()

	frameHeader := make([]byte, frameHeaderBytes)

	for {
		select {
		case <-c.closing:
			return
		default:
		}

		// Read frame header
		_, err := io.ReadFull(c.conn, frameHeader)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				// Connection error — will be notified via onClose
			}
			return
		}

		// Parse frame size
		frameSize := binary.BigEndian.Uint32(frameHeader)
		if frameSize == 0 {
			continue
		}
		if frameSize > maxFrameBytes {
			return
		}

		// Read frame payload
		payload := make([]byte, frameSize)
		_, err = io.ReadFull(c.conn, payload)
		if err != nil {
			return
		}

		// Parse protocol header from payload
		if len(payload) < protocol.HeaderSize {
			continue
		}

		version := payload[0]
		if version != protocol.Version1 {
			continue
		}

		msgID := protocol.GetMsgID(payload[1:4])
		reqID := binary.BigEndian.Uint32(payload[4:8])
		body := payload[8:]

		if !protocol.IsResponse(msgID) {
			c.handleInboundRequest(msgID, reqID, body)
			continue
		}

		// Route to pending request
		c.mu.RLock()
		ch, ok := c.pending[reqID]
		c.mu.RUnlock()

		if ok {
			select {
			case ch <- responseTuple{msgID: msgID, body: body}:
				// Delivered to waiting goroutine
			case <-c.closing:
				return
			}
		}
	}
}

// handleInboundRequest 把入站请求投递到 worker 池（读循环永不执行
// 业务逻辑——串行处理会让一个慢 handler 卡住整条连接的所有请求）。
// 队列满时立即回 busy 错误帧，让 Agent 侧 failover 接管而不是排队
// 积压内存。
func (c *TCPClient) handleInboundRequest(msgID uint32, reqID uint32, body []byte) {
	if c.inbound == nil || !protocol.IsRequest(msgID) {
		return
	}
	task := inboundTask{msgID: msgID, reqID: reqID, body: body}
	select {
	case c.inbox <- task:
	default:
		// 队列满：回 busy 错误响应（handler 错误同款格式），Agent 侧
		// failover 换实例重试。
		var respBody []byte
		if msgID == protocol.MsgInvokeRequest {
			errResp := &sdkv1.InvokeResponse{
				Payload: []byte(`{"error":"inbound queue full, retry on another instance"}`),
			}
			if marshaled, marshalErr := proto.Marshal(errResp); marshalErr == nil {
				respBody = marshaled
			}
		}
		c.writeResponseFrame(msgID, reqID, respBody)
	}
}

// failAllPending wakes every in-flight Call with a connection-dead error.
func (c *TCPClient) failAllPending() {
	c.mu.Lock()
	c.dead = true
	pending := c.pending
	c.pending = make(map[uint32]chan responseTuple)
	c.mu.Unlock()
	for _, ch := range pending {
		select {
		case ch <- responseTuple{msgID: 0, body: nil, err: errConnectionClosed}:
		default:
		}
	}
}

// Close closes the client connection.
func (c *TCPClient) Close() error {
	var closeErr error
	c.once.Do(func() {
		close(c.closing)
		closeErr = c.conn.Close()
		c.readLoopWg.Wait()
		// 收编入站 worker：关闭队列（可能已有缓冲任务，worker 会快速
		// 失败——连接已死写不出去），等待全部退出避免协程泄漏。
		if c.inbox != nil {
			close(c.inbox)
			c.inboxWg.Wait()
		}
	})
	return closeErr
}

// IsClosed returns true if the client has been closed.
func (c *TCPClient) IsClosed() bool {
	select {
	case <-c.closing:
		return true
	default:
		return false
	}
}

// notifyClose calls the onClose callback if set and the client was not
// intentionally closed. It is called as a deferred function in receiveLoop.
//
// The callback runs in its own goroutine because the caller (receiveLoop)
// is still alive when this runs — invoking Close() synchronously from the
// callback would deadlock on readLoopWg.Wait() since readLoopWg.Done()
// hasn't executed yet.
func (c *TCPClient) notifyClose() {
	// Skip notification if client was intentionally closed
	if c.IsClosed() {
		return
	}
	c.mu.RLock()
	fn := c.onClose
	c.mu.RUnlock()
	if fn != nil {
		go fn(errors.New("connection lost"))
	}
}
