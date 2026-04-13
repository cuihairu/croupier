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

	// Auto-reconnect
	reconnectCfg *ReconnectConfig
	reconnecting atomic.Bool
	reconnWg     sync.WaitGroup

	// Callbacks
	onReconnect func() // Called when reconnection succeeds

	// Logging
	logger Logger
}

// ReconnectConfig configures automatic reconnection behavior
type ReconnectConfig struct {
	Enabled           bool          // Enable auto-reconnect
	MaxRetries        int           // Maximum number of reconnection attempts (-1 for infinite)
	InitialDelay      time.Duration // Initial delay before first retry
	MaxDelay          time.Duration // Maximum delay between retries
	Multiplier        float64       // Exponential backoff multiplier
	Jitter            float64       // Random jitter factor (0-1)
	HeartbeatInterval time.Duration // Heartbeat interval to detect connection issues
}

// DefaultReconnectConfig returns the default reconnection configuration
func DefaultReconnectConfig() *ReconnectConfig {
	return &ReconnectConfig{
		Enabled:           true,
		MaxRetries:        5,
		InitialDelay:      500 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		Multiplier:        2.0,
		Jitter:            0.1,
		HeartbeatInterval: 30 * time.Second,
	}
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

// SetOnReconnect sets a callback to be called when reconnection succeeds
func (c *Client) SetOnReconnect(callback func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onReconnect = callback
}

// SetReconnectConfig sets the reconnection configuration
func (c *Client) SetReconnectConfig(cfg *ReconnectConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reconnectCfg = cfg
}

// EnableReconnect enables automatic reconnection with default settings
func (c *Client) EnableReconnect() {
	c.SetReconnectConfig(DefaultReconnectConfig())
}

// DisableReconnect disables automatic reconnection
func (c *Client) DisableReconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.reconnectCfg != nil {
		c.reconnectCfg.Enabled = false
	}
}

// IsReconnectEnabled returns whether auto-reconnect is enabled
func (c *Client) IsReconnectEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.reconnectCfg != nil && c.reconnectCfg.Enabled
}

// Dial connects to the NNG server
// It will try each address in order until one succeeds
func (c *Client) Dial() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("client already connected")
	}

	return c.dialLocked()
}

// dialLocked performs the actual dial operation (caller must hold lock)
func (c *Client) dialLocked() error {
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

		// Start reconnection loop if enabled
		if c.reconnectCfg != nil && c.reconnectCfg.Enabled {
			c.reconnWg.Add(1)
			go c.reconnectLoop()
		}

		c.logger.Info("NNG Control client connected", "addr", dialAddr)
		return nil
	}

	// All addresses failed
	sock.Close()
	return fmt.Errorf("failed to dial any address (last error: %w)", lastErr)
}

// reconnectLoop handles automatic reconnection
func (c *Client) reconnectLoop() {
	defer c.reconnWg.Done()

	cfg := c.getReconnectConfig()
	if cfg == nil || !cfg.Enabled {
		return
	}

	ticker := time.NewTicker(cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			// Check if connection is still alive
			if c.isConnected() {
				continue
			}

			// Connection lost, attempt reconnection
			if c.reconnecting.CompareAndSwap(false, true) {
				c.logger.Warn("connection lost, attempting to reconnect")
				c.attemptReconnect()
				c.reconnecting.Store(false)
			}
		}
	}
}

// getReconnectConfig returns the current reconnection config (caller must hold lock or use atomic read)
func (c *Client) getReconnectConfig() *ReconnectConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.reconnectCfg
}

// isConnected checks if the socket is still connected
func (c *Client) isConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running && c.sock != nil
}

func (c *Client) currentSocket() mangos.Socket {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sock
}

// attemptReconnect attempts to reconnect to the server
func (c *Client) attemptReconnect() {
	cfg := c.getReconnectConfig()
	if cfg == nil {
		cfg = DefaultReconnectConfig()
	}

	attempt := 0
	for {
		// Check if we should stop trying
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Check max retries
		if cfg.MaxRetries >= 0 && attempt > cfg.MaxRetries {
			c.logger.Error("max reconnection retries exceeded", "maxRetries", cfg.MaxRetries)
			return
		}

		// Calculate delay
		delay := c.calculateReconnectDelay(attempt, cfg)
		c.logger.Debug("waiting before reconnect attempt", "attempt", attempt, "delay", delay)

		select {
		case <-c.ctx.Done():
			return
		case <-time.After(delay):
		}

		// Attempt reconnection
		c.mu.Lock()
		err := c.dialLocked()
		if err == nil {
			c.mu.Unlock()
			c.logger.Info("reconnection successful")

			// Call reconnection callback if set (e.g., to re-register)
			if c.onReconnect != nil {
				c.logger.Info("triggering post-reconnect callback")
				c.onReconnect()
			}
			return
		}
		c.mu.Unlock()

		attempt++
		c.logger.Warn("reconnection attempt failed", "attempt", attempt, "error", err)
	}
}

// calculateReconnectDelay calculates the delay before the next reconnection attempt
func (c *Client) calculateReconnectDelay(attempt int, cfg *ReconnectConfig) time.Duration {
	if attempt == 0 {
		return cfg.InitialDelay
	}

	// Exponential backoff
	delay := float64(cfg.InitialDelay) * powFloat64(cfg.Multiplier, float64(attempt))

	// Cap at max delay
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	// Add jitter
	if cfg.Jitter > 0 {
		jitterRange := delay * cfg.Jitter
		// Simple random jitter: [-jitterRange, +jitterRange]
		jitterOffset := (float64(time.Now().UnixNano()%1000000)/1000000.0*2 - 1) * jitterRange
		delay += jitterOffset
	}

	// Ensure non-negative
	if delay < 0 {
		delay = 0
	}

	return time.Duration(delay)
}

// powFloat64 calculates base^exp for float64 values
func powFloat64(base, exp float64) float64 {
	if exp == 0 {
		return 1
	}
	if base == 0 {
		return 0
	}

	result := base
	for i := 1; i < int(exp); i++ {
		result *= base
	}
	return result
}

// Close closes the client connection
func (c *Client) Close() error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = false
	sock := c.sock
	c.sock = nil
	c.mu.Unlock()

	// Cancel context to stop all goroutines
	c.cancel()

	// Wait for reconnection loop to finish
	c.reconnWg.Wait()

	if sock != nil {
		if err := sock.Close(); err != nil {
			c.logger.Error("failed to close socket", "error", err)
		}
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
	sock := c.currentSocket()
	if sock == nil {
		return nil, fmt.Errorf("client socket closed")
	}
	if err := sock.SendMsg(msg); err != nil {
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

		sock := c.currentSocket()
		if sock == nil {
			return
		}

		// Receive message
		msg, err := sock.RecvMsg()
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
