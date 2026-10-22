package tcp

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// Server implements a plain TCP request-response transport server.
type Server struct {
	config   *Config
	listener net.Listener
	handler  transportcore.Handler
	closing  chan struct{}
	once     sync.Once
	wg       sync.WaitGroup

	// mu/conns 跟踪活跃连接：Close 时主动断开，避免 wg.Wait 被
	// 「阻塞在读直到 RecvTimeout」的 handler 卡住（互联 RecvTimeout
	// 90s 时优雅停机会拖满整个超时）。
	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

var _ transportcore.Server = (*Server)(nil)

// NewServer creates a new TCP transport server.
func NewServer(config *Config, handler transportcore.Handler) (*Server, error) {
	if config == nil {
		config = &Config{}
	}
	if handler == nil {
		return nil, fmt.Errorf("handler is required")
	}

	ln, err := listen(config)
	if err != nil {
		return nil, err
	}

	return &Server{
		config:   config,
		listener: ln,
		handler:  handler,
		closing:  make(chan struct{}),
	}, nil
}

// Serve accepts and handles connections until ctx is done or the server closes.
func (s *Server) Serve(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	for {
		if tcpln, ok := s.listener.(*net.TCPListener); ok {
			_ = tcpln.SetDeadline(time.Now().Add(time.Second))
		}

		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.closing:
				return nil
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				continue
			}
			return fmt.Errorf("accept: %w", err)
		}

		s.trackConn(conn)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer s.untrackConn(conn)
			s.serveConn(ctx, conn)
		}()
	}
}

func (s *Server) serveConn(ctx context.Context, conn net.Conn) {
	defer func() { _ = conn.Close() }()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.closing:
			return
		default:
		}

		if s.config.RecvTimeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(s.config.RecvTimeout))
		}

		frame, err := readFrame(conn)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return
		}

		_, msgID, reqID, body, err := protocol.ParseMessageFromBody(frame)
		if err != nil {
			return
		}

		respBody, handleErr := s.handler.Handle(ctx, msgID, reqID, body)
		if handleErr != nil {
			// Log the real error so it is diagnosable. Previously this was
			// silently JSON-encoded and sent back; a JSON body is not valid
			// proto, so the client failed with "proto: cannot parse invalid
			// wire-format data", masking the actual error.
			slog.Error("rpc handler error", "msg_id", protocol.MsgIDString(msgID), "error", handleErr)
			if msgID == protocol.MsgInvokeRequest {
				// InvokeRequest: return InvokeResponse with error payload (proto).
				resp := &sdkv1.InvokeResponse{Payload: []byte(`{"error":"` + handleErr.Error() + `"}`)}
				respBody, _ = proto.Marshal(resp)
			} else {
				// Other RPCs: send an empty body (valid proto; unmarshals to the
				// zero value of the expected response). The real error is logged.
				respBody = nil
			}
		}

		respFrame := protocol.NewMessageBody(protocol.GetResponseMsgID(msgID), reqID, respBody)
		if s.config.SendTimeout > 0 {
			_ = conn.SetWriteDeadline(time.Now().Add(s.config.SendTimeout))
		}
		if err := writeFrame(conn, respFrame); err != nil {
			return
		}
	}
}

// trackConn registers an accepted connection for Close-time shutdown.
func (s *Server) trackConn(conn net.Conn) {
	s.mu.Lock()
	if s.conns == nil {
		s.conns = map[net.Conn]struct{}{}
	}
	s.conns[conn] = struct{}{}
	s.mu.Unlock()
}

func (s *Server) untrackConn(conn net.Conn) {
	s.mu.Lock()
	delete(s.conns, conn)
	s.mu.Unlock()
}

// Close stops accepting new connections and unblocks active handlers by
// closing their connections — wg.Wait otherwise waits out the full
// RecvTimeout on idle connections.
func (s *Server) Close() error {
	var closeErr error
	s.once.Do(func() {
		close(s.closing)
		closeErr = s.listener.Close()
		s.mu.Lock()
		for conn := range s.conns {
			_ = conn.Close()
		}
		s.mu.Unlock()
		s.wg.Wait()
	})
	return closeErr
}

// IsClosed reports whether the server has been closed.
func (s *Server) IsClosed() bool {
	select {
	case <-s.closing:
		return true
	default:
		return false
	}
}

// Addr returns the bound server address.
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func listen(config *Config) (net.Listener, error) {
	addr := normalizeAddr(config.Address)
	if addr == "" {
		addr = defaultAddress()
	}

	if config.Insecure {
		return net.Listen("tcp", addr)
	}

	tlsConfig, err := tcptr.ServerTLSConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create tls config: %w", err)
	}
	return tls.Listen("tcp", addr, tlsConfig)
}

// createServerTLSConfig preserves the package-local entry point for the
// shared TLS listener configuration in core/transport/tcp.
func createServerTLSConfig(config *Config) (*tls.Config, error) { return tcptr.ServerTLSConfig(config) }

// BuildServerTLSConfig forwards to the shared TLS listener configuration.
func BuildServerTLSConfig(certFile, keyFile, caFile string, insecureSkipVerify bool) (*tls.Config, error) {
	return tcptr.BuildServerTLSConfig(certFile, keyFile, caFile, insecureSkipVerify)
}

// defaultAddress returns the listen address used when Config.Address is
// empty. Defaults to 127.0.0.1:19090; CROUPIER_TCP_ADDR overrides it so
// tests (and constrained environments) can pick a free port.
func defaultAddress() string {
	if addr := strings.TrimSpace(os.Getenv("CROUPIER_TCP_ADDR")); addr != "" {
		return addr
	}
	return "127.0.0.1:19090"
}
