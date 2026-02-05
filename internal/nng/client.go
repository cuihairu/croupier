// Package nng provides NNG-based client implementation for Croupier control plane
package nng

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/req"
	_ "go.nanomsg.org/mangos/v3/transport/ipc"
	_ "go.nanomsg.org/mangos/v3/transport/tcp"
	"google.golang.org/protobuf/proto"
)

// Client implements an NNG-based control client (replaces gRPC ControlServiceClient)
type Client struct {
	addrs []string // Multiple addresses to try (in order)
	addr  string   // Primary address (for backward compatibility)
	sock  mangos.Socket

	// Request/response matching
	pending   map[uint32]chan *mangos.Message
	pendingMu sync.RWMutex
	nextReqID uint32

	// State
	mu      sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	// Logging
	logger Logger
}

// Logger is a minimal logging interface
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// defaultLogger is a no-op logger
type defaultLogger struct{}

func (defaultLogger) Debug(msg string, args ...any) {}
func (defaultLogger) Info(msg string, args ...any)  {}
func (defaultLogger) Warn(msg string, args ...any)  {}
func (defaultLogger) Error(msg string, args ...any) {}

// NewClient creates a new NNG control client
// addr can be a single address or comma-separated multiple addresses
// The client will try to connect to each address in order
// Examples: "localhost:19090" or "ipc://croupier-server,localhost:19090"
func NewClient(addr string) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	// Parse addresses
	var addrs []string
	if addr != "" {
		parts := strings.Split(addr, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				// Add transport prefix if not present
				if !strings.Contains(part, "://") {
					// Default to TCP
					addrs = append(addrs, "tcp://"+part)
				} else {
					addrs = append(addrs, part)
				}
			}
		}
	}

	// Default if none specified
	if len(addrs) == 0 {
		addrs = []string{"tcp://:19090"}
	}

	return &Client{
		addrs:     addrs,
		addr:      addr, // Keep for backward compatibility
		pending:   make(map[uint32]chan *mangos.Message),
		nextReqID: 1,
		ctx:       ctx,
		cancel:    cancel,
		logger:    defaultLogger{},
	}
}

// NewClientWithAddrs creates a new NNG control client with explicit addresses
func NewClientWithAddrs(addrs []string) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	// Ensure all addresses have transport prefix
	urls := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		if !strings.Contains(addr, "://") {
			urls = append(urls, "tcp://"+addr)
		} else {
			urls = append(urls, addr)
		}
	}

	// Default if none specified
	if len(urls) == 0 {
		urls = []string{"tcp://:19090"}
	}

	return &Client{
		addrs:     urls,
		pending:   make(map[uint32]chan *mangos.Message),
		nextReqID: 1,
		ctx:       ctx,
		cancel:    cancel,
		logger:    defaultLogger{},
	}
}

// SetLogger sets the logger
func (c *Client) SetLogger(logger Logger) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logger = logger
}

// Dial connects to the NNG server
// It will try each address in order until one succeeds
func (c *Client) Dial() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("client already connected")
	}

	// Create REQ socket
	sock, err := req.NewSocket()
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}

	// Configure options
	if err := sock.SetOption(mangos.OptionRecvDeadline, 5*time.Second); err != nil {
		sock.Close()
		return fmt.Errorf("failed to set recv deadline: %w", err)
	}

	// Set reasonable send/requeue options
	if err := sock.SetOption(mangos.OptionSendDeadline, 5*time.Second); err != nil {
		sock.Close()
		return fmt.Errorf("failed to set send deadline: %w", err)
	}
	if err := sock.SetOption(mangos.OptionReconnectTime, time.Second); err != nil {
		sock.Close()
		return fmt.Errorf("failed to set reconnect time: %w", err)
	}

	// Try each address in order
	var lastErr error
	for _, dialAddr := range c.addrs {
		if err := sock.Dial(dialAddr); err != nil {
			lastErr = err
			c.logger.Debug("failed to dial address, trying next", "addr", dialAddr, "error", err)
			continue
		}
		// Successfully connected
		c.sock = sock
		c.running = true

		// Start receive loop
		go c.receiveLoop()

		c.logger.Info("NNG Control client connected", "addr", dialAddr)
		return nil
	}

	// All addresses failed
	sock.Close()
	return fmt.Errorf("failed to dial any address (last error: %w)", lastErr)
}

// Close closes the client connection
func (c *Client) Close() error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = false
	c.mu.Unlock()

	c.cancel()

	if c.sock != nil {
		if err := c.sock.Close(); err != nil {
			c.logger.Error("failed to close socket", "error", err)
		}
		c.sock = nil
	}

	// Close all pending channels
	c.pendingMu.Lock()
	for _, ch := range c.pending {
		close(ch)
	}
	c.pending = make(map[uint32]chan *mangos.Message)
	c.pendingMu.Unlock()

	c.logger.Info("NNG Control client closed")
	return nil
}

// Connected returns true if the client is connected
func (c *Client) Connected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running && c.sock != nil
}

// Register sends a RegisterRequest to the server
func (c *Client) Register(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal RegisterRequest: %w", err)
	}

	respData, err := c.call(ctx, protocol.MsgRegisterRequest, data)
	if err != nil {
		return nil, err
	}

	resp := &agentv1.RegisterResponse{}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return nil, fmt.Errorf("unmarshal RegisterResponse: %w", err)
	}

	return resp, nil
}

// Heartbeat sends a HeartbeatRequest to the server
func (c *Client) Heartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal HeartbeatRequest: %w", err)
	}

	respData, err := c.call(ctx, protocol.MsgHeartbeatRequest, data)
	if err != nil {
		return nil, err
	}

	resp := &agentv1.HeartbeatResponse{}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return nil, fmt.Errorf("unmarshal HeartbeatResponse: %w", err)
	}

	return resp, nil
}

// RegisterCapabilities sends a RegisterCapabilitiesRequest to the server
func (c *Client) RegisterCapabilities(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal RegisterCapabilitiesRequest: %w", err)
	}

	respData, err := c.call(ctx, protocol.MsgRegisterCapabilitiesReq, data)
	if err != nil {
		return nil, err
	}

	resp := &agentv1.RegisterCapabilitiesResponse{}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return nil, fmt.Errorf("unmarshal RegisterCapabilitiesResponse: %w", err)
	}

	return resp, nil
}

// call sends a request and waits for the response
func (c *Client) call(ctx context.Context, msgID uint32, data []byte) ([]byte, error) {
	if !c.Connected() {
		return nil, fmt.Errorf("client not connected")
	}

	// Allocate RequestID
	reqID := atomic.AddUint32(&c.nextReqID, 1)

	// Create message body with protocol header
	body := protocol.NewMessageBody(msgID, reqID, data)

	// Create response channel
	respCh := make(chan *mangos.Message, 1)
	c.pendingMu.Lock()
	c.pending[reqID] = respCh
	c.pendingMu.Unlock()

	// Clean up pending on exit
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, reqID)
		c.pendingMu.Unlock()
		close(respCh)
	}()

	// Send request
	msg := mangos.NewMessage(0)
	msg.Body = body
	if err := c.sock.SendMsg(msg); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	// Wait for response
	select {
	case respMsg := <-respCh:
		if respMsg == nil {
			return nil, fmt.Errorf("received nil response")
		}
		defer respMsg.Free()

		// Parse response from body (header + data)
		_, respMsgID, _, respData, err := protocol.ParseMessageFromBody(respMsg.Body)
		if err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}

		expectedMsgID := protocol.GetResponseMsgID(msgID)
		if respMsgID != expectedMsgID {
			return nil, fmt.Errorf("unexpected response type: got 0x%06X, want 0x%06X", respMsgID, expectedMsgID)
		}

		return respData, nil

	case <-ctx.Done():
		return nil, ctx.Err()

	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout waiting for response")
	}
}

// receiveLoop handles incoming responses
func (c *Client) receiveLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		if c.sock == nil {
			return
		}

		// Receive message
		msg, err := c.sock.RecvMsg()
		if err != nil {
			// Check if context is cancelled
			select {
			case <-c.ctx.Done():
				return
			default:
			}
			// Timeout is expected due to RecvDeadline
			continue
		}

		// Parse protocol header from body prefix
		_, msgID, reqID, _, err := protocol.ParseMessageFromBody(msg.Body)
		if err != nil {
			c.logger.Error("failed to parse message", "error", err)
			msg.Free()
			continue
		}

		// Check if this is a response (even msgID)
		if !protocol.IsResponse(msgID) {
			c.logger.Warn("received non-response message", "msgID", protocol.MsgIDString(msgID))
			msg.Free()
			continue
		}

		// Find pending request
		c.pendingMu.RLock()
		ch, ok := c.pending[reqID]
		c.pendingMu.RUnlock()

		if ok {
			// Send to pending channel (non-blocking)
			select {
			case ch <- msg:
				// Message ownership transferred to receiver
			default:
				c.logger.Warn("pending channel full, dropping response", "reqID", reqID)
				msg.Free()
			}
		} else {
			c.logger.Debug("no pending request for response", "reqID", reqID)
			msg.Free()
		}
	}
}

// Call sends a generic request and waits for response
// This is used by Dispatcher to call Agent services
func (c *Client) Call(ctx context.Context, msgID uint32, data []byte) ([]byte, error) {
	return c.call(ctx, msgID, data)
}

// IsRunning returns true if the client is connected and running
func (c *Client) IsRunning() bool {
	return c.Connected()
}
