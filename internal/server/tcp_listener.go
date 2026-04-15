package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// TCPListenerConfig holds the configuration for the Server-side TCP session listener.
type TCPListenerConfig struct {
	// Address is the listen address (e.g., ":19090").
	Address string

	// Insecure controls whether TLS is disabled.
	Insecure bool

	// TLS files.
	CertFile string
	KeyFile  string
	CAFile   string

	// Timeouts.
	RecvTimeout time.Duration
	SendTimeout time.Duration
}

// TCPListener accepts Agent TCP sessions and creates AgentSession entries.
//
// Lifecycle:
//  1. Agent dials in → first frame must be RegisterRequest
//  2. Server validates and creates AgentSession → stores in AgentSessionStore
//  3. Subsequent Heartbeat/Invoke/StartJob/CancelJob flow through MuxConn
//  4. On disconnect, session is removed from store
type TCPListener struct {
	config       *TCPListenerConfig
	listener     net.Listener
	sessionStore *AgentSessionStore
	registry     *reg.Store
	handler      *ControlService // control-plane service

	wg      sync.WaitGroup
	closing chan struct{}
	once    sync.Once

	logger *slog.Logger
}

// NewTCPListener creates a new TCP session listener.
func NewTCPListener(config *TCPListenerConfig, sessionStore *AgentSessionStore, registry *reg.Store, logger *slog.Logger) (*TCPListener, error) {
	if config == nil {
		config = &TCPListenerConfig{Address: ":19090", Insecure: true}
	}
	if sessionStore == nil {
		sessionStore = NewAgentSessionStore()
	}
	if logger == nil {
		logger = slog.Default()
	}
	if registry == nil {
		registry = reg.NewStore()
	}

	ln, err := listenTCP(config)
	if err != nil {
		return nil, err
	}

	return &TCPListener{
		config:       config,
		listener:     ln,
		sessionStore: sessionStore,
		registry:     registry,
		closing:      make(chan struct{}),
		logger:       logger,
	}, nil
}

// SetHandler sets the control service for processing register/heartbeat.
func (l *TCPListener) SetHandler(h *ControlService) {
	l.handler = h
}

// Serve accepts connections until ctx is done or the listener is closed.
func (l *TCPListener) Serve(ctx context.Context) error {
	for {
		if tcpln, ok := l.listener.(*net.TCPListener); ok {
			_ = tcpln.SetDeadline(time.Now().Add(time.Second))
		}

		conn, err := l.listener.Accept()
		if err != nil {
			select {
			case <-l.closing:
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

		l.wg.Add(1)
		go func() {
			defer l.wg.Done()
			l.serveConn(ctx, conn)
		}()
	}
}

// serveConn handles a single Agent TCP connection.
// First frame must be RegisterRequest; subsequent frames are bidirectional.
func (l *TCPListener) serveConn(ctx context.Context, conn net.Conn) {
	remoteAddr := conn.RemoteAddr().String()
	l.logger.Info("Agent TCP connection accepted", "remote", remoteAddr)
	defer func() {
		conn.Close()
		l.logger.Info("Agent TCP connection closed", "remote", remoteAddr)
	}()

	muxCfg := &tcptr.Config{
		RecvTimeout: l.config.RecvTimeout,
		SendTimeout: l.config.SendTimeout,
	}

	// Create a handler that dispatches through the control plane logic
	// and also tracks AgentSession lifecycle.
	handler := &agentSessionHandler{
		listener:   l,
		conn:       nil, // set below
		registered: false,
		agentID:    "",
	}

	mux := tcptr.NewMuxConn(conn, muxCfg, handler)
	handler.conn = mux

	// Run blocks until connection closes or ctx is done.
	if err := mux.Run(ctx); err != nil {
		l.logger.Debug("Agent MuxConn ended", "remote", remoteAddr, "error", err)
	}

	// Clean up session on disconnect
	if handler.agentID != "" {
		l.sessionStore.Remove(handler.agentID)
		l.logger.Info("Agent session removed on disconnect", "agent_id", handler.agentID)
	}
}

// Addr returns the bound listener address.
func (l *TCPListener) Addr() string {
	if l.listener == nil {
		return ""
	}
	return l.listener.Addr().String()
}

// SessionStore returns the Agent session store.
func (l *TCPListener) SessionStore() *AgentSessionStore {
	return l.sessionStore
}

// Close stops accepting new connections and waits for active ones to finish.
func (l *TCPListener) Close() error {
	var closeErr error
	l.once.Do(func() {
		close(l.closing)
		closeErr = l.listener.Close()
		l.wg.Wait()
	})
	return closeErr
}

// IsClosed reports whether the listener has been closed.
func (l *TCPListener) IsClosed() bool {
	select {
	case <-l.closing:
		return true
	default:
		return false
	}
}

// agentSessionHandler implements transport.Handler for Agent TCP sessions.
// It processes the first frame as RegisterRequest and subsequent frames
// as normal control plane requests (Heartbeat, etc.).
type agentSessionHandler struct {
	listener   *TCPListener
	conn       *tcptr.MuxConn
	registered bool
	agentID    string
}

// Handle processes inbound requests on an Agent TCP session.
func (h *agentSessionHandler) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	switch msgID {
	case protocol.MsgRegisterRequest:
		return h.handleRegister(ctx, body)
	case protocol.MsgHeartbeatRequest:
		return h.handleHeartbeat(ctx, body)
	default:
		// Delegate to the control service if available
		if h.listener.handler != nil {
			return h.listener.handler.TransportHandler().Handle(ctx, msgID, reqID, body)
		}
		return nil, fmt.Errorf("unsupported message type for Agent session: %s", protocol.MsgIDString(msgID))
	}
}

func (h *agentSessionHandler) handleRegister(ctx context.Context, body []byte) ([]byte, error) {
	req := &agentv1.RegisterRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal RegisterRequest: %w", err)
	}

	if req.AgentId == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	// Compute session TTL
	ttl := 5 * time.Minute
	if req.TtlSeconds > 0 {
		ttl = time.Duration(req.TtlSeconds) * time.Second
	}
	expireAt := time.Now().Add(ttl)

	// Create Agent session
	sess := &AgentSession{
		conn:        h.conn,
		AgentID:     req.AgentId,
		SessionID:   fmt.Sprintf("as-%d", time.Now().UnixNano()),
		GameID:      req.GameId,
		Env:         req.Env,
		Version:     req.Version,
		RPCAddr:     req.RpcAddr,
		ConnectedAt: time.Now(),
	}
	sess.UpdateLastSeen()

	// Store session
	h.listener.sessionStore.Upsert(sess)
	h.agentID = req.AgentId
	h.registered = true

	h.listener.logger.Info("Agent registered via TCP session",
		"agent_id", req.AgentId,
		"game_id", req.GameId,
		"session_id", sess.SessionID,
		"remote", h.conn.RemoteAddr(),
	)

	// Build response
	resp := &agentv1.RegisterResponse{
		SessionId: sess.SessionID,
		ExpireAt:  expireAt.Unix(),
	}
	return proto.Marshal(resp)
}

func (h *agentSessionHandler) handleHeartbeat(ctx context.Context, body []byte) ([]byte, error) {
	req := &agentv1.HeartbeatRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal HeartbeatRequest: %w", err)
	}

	// Update session last seen
	if sess, ok := h.listener.sessionStore.Get(req.AgentId); ok {
		sess.UpdateLastSeen()
	}

	resp := &agentv1.HeartbeatResponse{}
	return proto.Marshal(resp)
}

// listenTCP creates a net.Listener based on the config (plain or TLS).
func listenTCP(config *TCPListenerConfig) (net.Listener, error) {
	addr := config.Address
	if addr == "" {
		addr = ":19090"
	}

	if config.Insecure {
		return net.Listen("tcp", addr)
	}

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

	return tls.Listen("tcp", addr, tlsConfig)
}
