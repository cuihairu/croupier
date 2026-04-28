package tcp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	transportcore "github.com/cuihairu/croupier/internal/transport"
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

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.serveConn(ctx, conn)
		}()
	}
}

func (s *Server) serveConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

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
			// For InvokeRequest, return a proper InvokeResponse with error payload
			// instead of plain JSON error
			if msgID == 0x030101 { // MsgInvokeRequest
				errorJSON := []byte(`{"error":"` + handleErr.Error() + `"}`)
				resp := &sdkv1.InvokeResponse{Payload: errorJSON}
				respBody, _ = proto.Marshal(resp)
			} else {
				respBody, _ = json.Marshal(map[string]string{"error": handleErr.Error()})
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

// Close stops accepting new connections.
func (s *Server) Close() error {
	var closeErr error
	s.once.Do(func() {
		close(s.closing)
		closeErr = s.listener.Close()
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
		addr = "127.0.0.1:19090"
	}

	if config.Insecure {
		return net.Listen("tcp", addr)
	}

	tlsConfig, err := createServerTLSConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create tls config: %w", err)
	}
	return tls.Listen("tcp", addr, tlsConfig)
}

func createServerTLSConfig(config *Config) (*tls.Config, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

	if config.CertFile != "" && config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load server certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if config.CAFile != "" {
		pemBytes, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			return nil, fmt.Errorf("append CA certificate")
		}
		tlsConfig.ClientCAs = pool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	if config.InsecureSkipVerify {
		tlsConfig.ClientAuth = tls.NoClientCert
	}

	return tlsConfig, nil
}
