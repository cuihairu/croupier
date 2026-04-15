package agent

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	transportcore "github.com/cuihairu/croupier/internal/transport"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// TCPLocalListenerConfig holds configuration for the Agent's local TCP listener
// that accepts SDK Provider connections.
type TCPLocalListenerConfig struct {
	// Address is the listen address (e.g., "127.0.0.1:19091").
	Address string

	// Timeouts.
	RecvTimeout time.Duration
	SendTimeout time.Duration
}

// TCPLocalListener accepts SDK Provider TCP sessions on the Agent's local gateway.
//
// Lifecycle:
//  1. SDK Provider dials in → first frame must be ProviderConnectRequest
//  2. Agent validates and creates ProviderSession → stores in ProviderSessionStore
//  3. Heartbeat/Drain flow through MuxConn
//  4. Agent can send Invoke/StartJob to Provider via session.conn.Call()
//  5. On disconnect, session is removed from store
type TCPLocalListener struct {
	config       *TCPLocalListenerConfig
	listener     net.Listener
	sessionStore *ProviderSessionStore

	// onConnect is called after a successful ProviderConnect with the session.
	// This allows the Agent to register the provider's functions in its local store.
	onConnect func(sess *ProviderSession)

	// onDisconnect is called when a Provider session disconnects.
	onDisconnect func(sess *ProviderSession)

	// localHandler handles non-session-control messages (e.g., Invoke forwarded from upstream).
	localHandler transportcore.Handler

	wg      sync.WaitGroup
	closing chan struct{}
	once    sync.Once

	logger *slog.Logger
}

// NewTCPLocalListener creates a new local TCP listener for SDK connections.
func NewTCPLocalListener(config *TCPLocalListenerConfig, sessionStore *ProviderSessionStore, logger *slog.Logger) (*TCPLocalListener, error) {
	if config == nil {
		config = &TCPLocalListenerConfig{Address: "127.0.0.1:19091"}
	}
	if sessionStore == nil {
		sessionStore = NewProviderSessionStore()
	}
	if logger == nil {
		logger = slog.Default()
	}

	ln, err := net.Listen("tcp", config.Address)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", config.Address, err)
	}

	return &TCPLocalListener{
		config:       config,
		listener:     ln,
		sessionStore: sessionStore,
		closing:      make(chan struct{}),
		logger:       logger,
	}, nil
}

// SetOnConnect sets the callback invoked after a Provider successfully connects.
func (l *TCPLocalListener) SetOnConnect(fn func(sess *ProviderSession)) {
	l.onConnect = fn
}

// SetOnDisconnect sets the callback invoked when a Provider disconnects.
func (l *TCPLocalListener) SetOnDisconnect(fn func(sess *ProviderSession)) {
	l.onDisconnect = fn
}

// SetLocalHandler sets the handler for inbound requests from providers (e.g., invoke responses).
func (l *TCPLocalListener) SetLocalHandler(h transportcore.Handler) {
	l.localHandler = h
}

// Serve accepts connections until ctx is done or the listener is closed.
func (l *TCPLocalListener) Serve(ctx context.Context) error {
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

// serveConn handles a single Provider TCP connection.
// First frame must be ProviderConnectRequest.
func (l *TCPLocalListener) serveConn(ctx context.Context, conn net.Conn) {
	remoteAddr := conn.RemoteAddr().String()
	l.logger.Info("Provider TCP connection accepted", "remote", remoteAddr)
	defer func() {
		conn.Close()
		l.logger.Info("Provider TCP connection closed", "remote", remoteAddr)
	}()

	muxCfg := &tcptr.Config{
		RecvTimeout: l.config.RecvTimeout,
		SendTimeout: l.config.SendTimeout,
	}

	handler := &providerSessionHandler{
		listener:   l,
		conn:       nil, // set below
		registered: false,
		sessionID:  "",
		serviceID:  "",
	}

	mux := tcptr.NewMuxConn(conn, muxCfg, handler)
	handler.conn = mux

	// Run blocks until connection closes or ctx is done.
	if err := mux.Run(ctx); err != nil {
		l.logger.Debug("Provider MuxConn ended", "remote", remoteAddr, "error", err)
	}

	// Clean up session on disconnect
	if handler.sessionID != "" {
		if sess, ok := l.sessionStore.GetBySessionID(handler.sessionID); ok {
			if l.onDisconnect != nil {
				l.onDisconnect(sess)
			}
			l.sessionStore.Remove(handler.sessionID)
			l.logger.Info("Provider session removed on disconnect", "service_id", handler.serviceID, "session_id", handler.sessionID)
		}
	}
}

// Addr returns the bound listener address.
func (l *TCPLocalListener) Addr() string {
	if l.listener == nil {
		return ""
	}
	return l.listener.Addr().String()
}

// SessionStore returns the Provider session store.
func (l *TCPLocalListener) SessionStore() *ProviderSessionStore {
	return l.sessionStore
}

// Close stops accepting new connections and waits for active ones to finish.
func (l *TCPLocalListener) Close() error {
	var closeErr error
	l.once.Do(func() {
		close(l.closing)
		closeErr = l.listener.Close()
		l.wg.Wait()
	})
	return closeErr
}

// IsClosed reports whether the listener has been closed.
func (l *TCPLocalListener) IsClosed() bool {
	select {
	case <-l.closing:
		return true
	default:
		return false
	}
}

// providerSessionHandler implements transport.Handler for Provider TCP sessions.
type providerSessionHandler struct {
	listener   *TCPLocalListener
	conn       *tcptr.MuxConn
	registered bool
	sessionID  string
	serviceID  string
}

// Handle processes inbound requests on a Provider TCP session.
func (h *providerSessionHandler) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	switch msgID {
	case protocol.MsgProviderConnectRequest:
		return h.handleConnect(ctx, body)
	case protocol.MsgProviderHeartbeatRequest:
		return h.handleHeartbeat(ctx, body)
	case protocol.MsgProviderDrainRequest:
		return h.handleDrain(ctx, body)
	default:
		// Delegate to local handler for other message types
		if h.listener.localHandler != nil {
			return h.listener.localHandler.Handle(ctx, msgID, reqID, body)
		}
		return nil, fmt.Errorf("unsupported message type for Provider session: %s", protocol.MsgIDString(msgID))
	}
}

func (h *providerSessionHandler) handleConnect(ctx context.Context, body []byte) ([]byte, error) {
	req := &sdkv1.ProviderConnectRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal ProviderConnectRequest: %w", err)
	}

	if req.ServiceId == "" {
		return nil, fmt.Errorf("service_id is required")
	}

	sessionID := fmt.Sprintf("ps-%d", time.Now().UnixNano())

	sess := &ProviderSession{
		conn:        h.conn,
		SessionID:   sessionID,
		ServiceID:   req.ServiceId,
		Version:     req.Version,
		Functions:   convertProtoFunctions(req.Functions),
		ConnectedAt: time.Now(),
		SDKLanguage: req.SdkLanguage,
		SDKVersion:  req.SdkVersion,
	}
	sess.UpdateLastSeen()

	h.listener.sessionStore.Upsert(sess)
	h.sessionID = sessionID
	h.serviceID = req.ServiceId
	h.registered = true

	h.listener.logger.Info("Provider connected via TCP session",
		"service_id", req.ServiceId,
		"version", req.Version,
		"session_id", sessionID,
		"sdk_language", req.SdkLanguage,
		"sdk_version", req.SdkVersion,
		"functions", len(req.Functions),
		"remote", h.conn.RemoteAddr(),
	)

	// Notify onConnect callback
	if h.listener.onConnect != nil {
		h.listener.onConnect(sess)
	}

	resp := &sdkv1.ProviderConnectResponse{
		SessionId: sessionID,
	}
	return proto.Marshal(resp)
}

func (h *providerSessionHandler) handleHeartbeat(ctx context.Context, body []byte) ([]byte, error) {
	req := &sdkv1.ProviderHeartbeatRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal ProviderHeartbeatRequest: %w", err)
	}

	// Update session last seen
	if sess, ok := h.listener.sessionStore.GetBySessionID(req.SessionId); ok {
		sess.UpdateLastSeen()
	}

	resp := &sdkv1.ProviderHeartbeatResponse{}
	return proto.Marshal(resp)
}

func (h *providerSessionHandler) handleDrain(ctx context.Context, body []byte) ([]byte, error) {
	req := &sdkv1.ProviderDrainRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal ProviderDrainRequest: %w", err)
	}

	h.listener.logger.Info("Provider drain requested",
		"session_id", req.SessionId,
		"reason", req.Reason,
		"retry_after_ms", req.RetryAfterMs,
	)

	resp := &sdkv1.ProviderDrainResponse{}
	return proto.Marshal(resp)
}

// convertProtoFunctions converts proto LocalFunctionDescriptor slices to the compat struct.
// The proto definition is the same type since we use the generated Go struct.
func convertProtoFunctions(funcs []*sdkv1.LocalFunctionDescriptor) []*sdkv1.LocalFunctionDescriptor {
	if funcs == nil {
		return nil
	}
	return funcs
}
