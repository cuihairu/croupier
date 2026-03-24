// Package tcp provides a plain TCP transport for the Croupier wire protocol.
package tcp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	transportcore "github.com/cuihairu/croupier/internal/transport"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// Config holds plain TCP transport configuration.
type Config struct {
	Address string

	Insecure           bool
	CAFile             string
	CertFile           string
	KeyFile            string
	ServerName         string
	InsecureSkipVerify bool

	ConnectTimeout time.Duration
	RecvTimeout    time.Duration
	SendTimeout    time.Duration
}

// Client implements a plain TCP request-response transport.
type Client struct {
	config  *Config
	conn    net.Conn
	closing chan struct{}
	once    sync.Once
	mu      sync.Mutex
}

var _ transportcore.Client = (*Client)(nil)

// NewClient creates a new TCP transport client and dials the remote endpoint.
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		config = &Config{}
	}

	conn, err := dial(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		config:  config,
		conn:    conn,
		closing: make(chan struct{}),
	}, nil
}

// Call sends a request frame and waits for a response frame.
func (c *Client) Call(ctx context.Context, msgID uint32, reqBody []byte) (respMsgID uint32, respBody []byte, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-c.closing:
		return 0, nil, fmt.Errorf("client is closing")
	default:
	}

	if deadline, ok := ctx.Deadline(); ok {
		if err := c.conn.SetDeadline(deadline); err != nil {
			return 0, nil, fmt.Errorf("set deadline: %w", err)
		}
	} else {
		if c.config.SendTimeout > 0 {
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.config.SendTimeout)); err != nil {
				return 0, nil, fmt.Errorf("set write deadline: %w", err)
			}
		}
		if c.config.RecvTimeout > 0 {
			if err := c.conn.SetReadDeadline(time.Now().Add(c.config.RecvTimeout)); err != nil {
				return 0, nil, fmt.Errorf("set read deadline: %w", err)
			}
		}
	}
	defer func() {
		_ = c.conn.SetDeadline(time.Time{})
	}()

	reqID := uint32(time.Now().UnixNano())
	payload := protocol.NewMessageBody(msgID, reqID, reqBody)
	if err := writeFrame(c.conn, payload); err != nil {
		return 0, nil, fmt.Errorf("write frame: %w", err)
	}

	frame, err := readFrame(c.conn)
	if err != nil {
		return 0, nil, fmt.Errorf("read frame: %w", err)
	}

	_, respMsgID, respReqID, respBody, err := protocol.ParseMessageFromBody(frame)
	if err != nil {
		return 0, nil, fmt.Errorf("parse response: %w", err)
	}
	if respReqID != reqID {
		return 0, nil, fmt.Errorf("request ID mismatch: expected %d, got %d", reqID, respReqID)
	}

	return respMsgID, respBody, nil
}

// Close closes the client connection.
func (c *Client) Close() error {
	var closeErr error
	c.once.Do(func() {
		close(c.closing)
		closeErr = c.conn.Close()
	})
	return closeErr
}

// IsClosed reports whether the client has been closed.
func (c *Client) IsClosed() bool {
	select {
	case <-c.closing:
		return true
	default:
		return false
	}
}

func dial(config *Config) (net.Conn, error) {
	addr := normalizeAddr(config.Address)
	dialer := &net.Dialer{Timeout: config.ConnectTimeout}
	if config.ConnectTimeout == 0 {
		dialer.Timeout = 5 * time.Second
	}

	if config.Insecure {
		return dialer.Dial("tcp", addr)
	}

	tlsConfig, err := createClientTLSConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create tls config: %w", err)
	}
	return tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
}

func createClientTLSConfig(config *Config) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: config.InsecureSkipVerify,
	}

	if config.CAFile != "" {
		caPEM, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("append CA certificate")
		}
		tlsConfig.RootCAs = pool
	}

	if config.CertFile != "" && config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if config.ServerName != "" {
		tlsConfig.ServerName = config.ServerName
	}

	return tlsConfig, nil
}

func normalizeAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	addr = strings.TrimPrefix(addr, "tcp://")
	return addr
}
