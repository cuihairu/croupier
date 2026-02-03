// Package nng provides NNG (Nanomsg Next Generation) transport layer for Croupier.
package nng

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	_ "go.nanomsg.org/mangos/v3/transport/tcp"
	_ "go.nanomsg.org/mangos/v3/transport/tlstcp"

	"github.com/cuihairu/croupier/internal/transport/nng/protocol"
)

// Handler handles incoming NNG requests.
type Handler interface {
	// Handle handles an incoming request and returns the response body.
	Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error)
}

// HandlerFunc is an adapter to allow the use of ordinary functions as Handlers.
type HandlerFunc func(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error)

// Handle calls f(ctx, msgID, reqID, body).
func (f HandlerFunc) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error) {
	return f(ctx, msgID, reqID, body)
}

// Server represents a NNG transport server.
// It uses the Rep protocol for handling request/response communication.
type Server struct {
	sock    mangos.Socket
	config  *Config
	handler Handler
	mu      sync.RWMutex
	closing chan struct{}
	once    sync.Once
}

// Config holds server configuration.
type Config struct {
	// Address is the listen address (e.g., "127.0.0.1:19090")
	Address string

	// Insecure disables TLS
	Insecure bool

	// RecvTimeout is the receive timeout
	RecvTimeout time.Duration

	// TLS configuration
	CAFile   string
	CertFile string
	KeyFile  string

	// ServerName for TLS verification
	ServerName string

	// InsecureSkipVerify skips TLS verification
	InsecureSkipVerify bool
}

// NewServer creates a new NNG server with the given configuration.
func NewServer(config *Config, handler Handler) (*Server, error) {
	sock, err := rep.NewSocket()
	if err != nil {
		return nil, fmt.Errorf("create rep socket: %w", err)
	}

	// Apply TLS configuration
	if !config.Insecure {
		tlsConfig, err := createTLSConfig(config)
		if err != nil {
			sock.Close()
			return nil, fmt.Errorf("create tls config: %w", err)
		}
		if err := sock.SetOption(mangos.OptionTLSConfig, tlsConfig); err != nil {
			sock.Close()
			return nil, fmt.Errorf("set tls config: %w", err)
		}
	}

	// Configure receive timeout
	if config.RecvTimeout > 0 {
		if err := sock.SetOption(mangos.OptionRecvDeadline, config.RecvTimeout); err != nil {
			sock.Close()
			return nil, fmt.Errorf("set receive deadline: %w", err)
		}
	}

	// Listen on address
	listenAddr := listenAddr(config)
	if err := sock.Listen(listenAddr); err != nil {
		sock.Close()
		return nil, fmt.Errorf("listen %s: %w", listenAddr, err)
	}

	return &Server{
		sock:    sock,
		config:  config,
		handler: handler,
		closing: make(chan struct{}),
	}, nil
}

// Serve starts the server's receive loop.
// It blocks until the context is cancelled or an error occurs.
func (s *Server) Serve(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.closing:
			return fmt.Errorf("server is closing")
		default:
		}

		// Receive request
		msg, err := s.sock.RecvMsg()
		if err != nil {
			// Check if server is closing
			select {
			case <-s.closing:
				return nil
			default:
			}
			// Continue on temporary errors
			continue
		}

		// Parse request
		_, msgID, reqID, body, err := protocol.ParseMessage(msg)
		msg.Free()
		if err != nil {
			s.sendError(0, protocol.MsgInvokeResponse, err)
			continue
		}

		// Handle request synchronously (Rep protocol requires response in same goroutine)
		ctx := context.Background()
		respBody, err := s.handler.Handle(ctx, msgID, reqID, body)

		// Create response message
		respMsgID := protocol.GetResponseMsgID(msgID)
		respMsg := mangos.NewMessage(0)
		respMsg.Header = make([]byte, protocol.HeaderSize)
		respMsg.Header[0] = protocol.Version1
		protocol.PutMsgID(respMsg.Header[1:4], respMsgID)
		binary.BigEndian.PutUint32(respMsg.Header[4:8], reqID)

		if err != nil {
			// Error response
			respMsg.Body = []byte(fmt.Sprintf("{\"error\": \"%s\"}", err.Error()))
		} else {
			respMsg.Body = respBody
		}

		// Send response
		if err := s.sock.SendMsg(respMsg); err != nil {
			// Log error but continue serving
			continue
		}
	}
}

// sendError sends an error response.
func (s *Server) sendError(reqID uint32, msgID uint32, err error) {
	respMsg := mangos.NewMessage(0)
	respMsg.Header = make([]byte, protocol.HeaderSize)
	respMsg.Header[0] = protocol.Version1
	protocol.PutMsgID(respMsg.Header[1:4], msgID)
	binary.BigEndian.PutUint32(respMsg.Header[4:8], reqID)
	respMsg.Body = []byte(fmt.Sprintf("{\"error\": \"%s\"}", err.Error()))
	s.sock.SendMsg(respMsg)
}

// Close closes the server and stops accepting new connections.
func (s *Server) Close() error {
	var closeErr error
	s.once.Do(func() {
		close(s.closing)
		closeErr = s.sock.Close()
	})
	return closeErr
}

// IsClosed returns true if the server has been closed.
func (s *Server) IsClosed() bool {
	select {
	case <-s.closing:
		return true
	default:
		return false
	}
}

// listenAddr returns the appropriate listen address string.
func listenAddr(cfg *Config) string {
	if !cfg.Insecure {
		return "tls+tcp://" + cfg.Address
	}
	return "tcp://" + cfg.Address
}

// Addr returns the actual listen address.
func (s *Server) Addr() string {
	if s.sock == nil {
		return ""
	}
	// mangos doesn't expose a direct way to get the listen address
	// Return the configured address instead
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config != nil {
		return s.config.Address
	}
	return ""
}
