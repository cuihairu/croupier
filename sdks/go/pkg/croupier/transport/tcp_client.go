// Package transport provides TCP transport layer for Croupier SDK.
package transport

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
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
}

type responseTuple struct {
	msgID uint32
	body  []byte
}

// NewTCPClient creates a new TCP client with the given configuration.
func NewTCPClient(config *Config) (*TCPClient, error) {
	// Parse host and port from address if not set
	host := config.Host
	port := config.Port
	if host == "" || port == 0 {
		// Parse from Address (expected format: "host:port" or "tcp://host:port")
		addr := config.Address
		if len(addr) > 6 && addr[:6] == "tcp://" {
			addr = addr[6:]
		} else if len(addr) > 9 && addr[:9] == "tls+tcp://" {
			addr = addr[9:]
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
	}

	// Start receive loop
	client.readLoopWg.Add(1)
	go client.receiveLoop()

	return client, nil
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
	// Allocate request ID
	c.mu.Lock()
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
	_, err = c.conn.Write(frame)
	if err != nil {
		return 0, nil, fmt.Errorf("send: %w", err)
	}

	// Wait for response
	select {
	case resp := <-respCh:
		return resp.msgID, resp.body, nil
	case <-ctx.Done():
		return 0, nil, ctx.Err()
	case <-c.closing:
		return 0, nil, errors.New("client is closing")
	}
}

// receiveLoop receives frames from the connection and routes them to pending requests.
func (c *TCPClient) receiveLoop() {
	defer c.readLoopWg.Done()

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
				// Connection error
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
		// Inbound requests (invoke/task from agent) would be handled here
	}
}

// Close closes the client connection.
func (c *TCPClient) Close() error {
	var closeErr error
	c.once.Do(func() {
		close(c.closing)
		closeErr = c.conn.Close()
		c.readLoopWg.Wait()
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
