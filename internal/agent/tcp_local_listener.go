package agent

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

	"github.com/cuihairu/croupier/agents/sidecar"
	transportcore "github.com/cuihairu/croupier/core/transport"
	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
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

	// TLSConfig, when non-nil, wraps every accepted connection in TLS
	// (crypto/tls) — above TCP, below the framing layer, so the wire protocol
	// is unaffected. Handshake failures drop only the offending connection;
	// the accept loop keeps serving (plaintext probes must not kill the
	// gateway, hence per-conn wrapping instead of tls.NewListener).
	TLSConfig *tls.Config

	// Provider handshake auth (CR-FIX-11): when Require=true, the
	// ProviderConnectRequest's metadata["authToken"] must equal Token, or the
	// handshake is rejected (connection closed, no session registered). Same
	// source of configuration as LocalHandler.SetProviderAuth (one startup
	// parameter injected into both paths); validation logic is shared in
	// checkProviderAuth.
	RequireProviderAuth bool
	ProviderAuthToken   string
}

// tlsHandshakeTimeout bounds the server-side TLS handshake of an accepted
// connection; half-open peers are dropped instead of pinning a goroutine.
const tlsHandshakeTimeout = 10 * time.Second

// TCPLocalListener accepts SDK Provider TCP sessions on the Agent's local gateway.
//
// Lifecycle:
//  1. SDK Provider dials in → first frame must be ProviderConnectRequest
//  2. Agent validates and creates ProviderSession → stores in ProviderSessionStore
//  3. Heartbeat/Drain flow through MuxConn
//  4. Agent can send Invoke/StartTask to Provider via session.conn.Call()
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
	closeMu sync.Mutex // guards idempotent close-out of closed/listener.Close/wg.Wait (see Close)
	closed  bool
	// serveStarted/serveDone let Close close out on the Accept loop exiting
	// itself: after Close closes the listener, Accept unblocks, the loop reads
	// closing and returns, serveDone closes, and Close waits on it before
	// returning. Otherwise the Serve goroutine would keep reading l.listener /
	// l.closing after Close returns, forming a data race with Close's writes
	// (proven by -race: App.Stop racing the serve goroutine started by
	// StartLocalServer, 2026-11-12 full-package shuffle).
	serveStarted bool
	serveDone    chan struct{}

	logger *slog.Logger
}

// NewTCPLocalListener creates a new local TCP listener for SDK connections.
// resolveLocalListenerAddress decides the local listen address: explicit
// config wins; when config is nil, CROUPIER_AGENT_LOCAL_ADDR is read, falling
// back to 127.0.0.1:19091 when empty. It is a standalone function so the
// default can be tested without actually binding a port (the constructor calls
// net.Listen immediately).
func resolveLocalListenerAddress(config *TCPLocalListenerConfig) *TCPLocalListenerConfig {
	if config != nil {
		return config
	}
	addr := os.Getenv("CROUPIER_AGENT_LOCAL_ADDR")
	if strings.TrimSpace(addr) == "" {
		addr = "127.0.0.1:19091"
	}
	return &TCPLocalListenerConfig{Address: addr}
}

func NewTCPLocalListener(config *TCPLocalListenerConfig, sessionStore *ProviderSessionStore, logger *slog.Logger) (*TCPLocalListener, error) {
	config = resolveLocalListenerAddress(config)
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
		serveDone:    make(chan struct{}),
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
	// Mark the loop as started and register the exit signal (Close uses this to
	// wait for the Accept loop to exit itself, see the serveStarted/serveDone
	// comment).
	l.closeMu.Lock()
	l.serveStarted = true
	l.closeMu.Unlock()
	defer close(l.serveDone)

	for {
		// The listener/closing/closed snapshot must be read under closeMu: Close
		// holds closeMu for its entire run, writing closed / close(closing) /
		// listener.Close(). If Serve read these fields bare, it would race with
		// Close's writes (-race proven: App.Stop racing the serve goroutine
		// started by StartLocalServer, 2026-11-12 full-package shuffle; the race
		// point is Serve's first-loop read of l.listener/l.closing against
		// Close's header writes). closeMu serializes both sides' access to these
		// fields and establishes happens-before.
		l.closeMu.Lock()
		ln := l.listener
		closing := l.closing
		closed := l.closed
		l.closeMu.Unlock()
		if closed {
			return nil
		}

		if tcpln, ok := ln.(*net.TCPListener); ok {
			_ = tcpln.SetDeadline(time.Now().Add(time.Second))
		}

		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-closing:
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
		_ = conn.Close()
		l.logger.Info("Provider TCP connection closed", "remote", remoteAddr)
	}()

	// TLS sits above TCP and below framing (capability matrix batch E): a
	// handshake failure only drops the current connection with a warning; the
	// Accept loop and existing sessions are unaffected — plaintext probes/scans
	// are not fatal.
	if tlsCfg := l.config.TLSConfig; tlsCfg != nil {
		tlsConn := tls.Server(conn, tlsCfg)
		hsCtx, cancel := context.WithTimeout(ctx, tlsHandshakeTimeout)
		err := tlsConn.HandshakeContext(hsCtx)
		cancel()
		if err != nil {
			l.logger.Warn("Provider TLS handshake failed", "remote", remoteAddr, "error", err)
			return
		}
		conn = tlsConn
	}

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
	// Join lane workers before reading handler state: registration (handleConnect)
	// runs on a worker goroutine, and without this wait its writes to
	// handler.sessionID/serviceID race the disconnect cleanup below. Mirrors
	// internal/server/tcp_listener.go (same MuxConn lane model).
	mux.Wait()

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
// Idempotent; uses a mutex rather than sync.Once: the lock establishes explicit
// happens-before between Close and Close, and Close and Serve, avoiding a data
// race between the atomic write of once.done and reads in a startup race
// window (2026-09-22 -race proven: flagged by the race detector when App.Stop
// overlapped the serve goroutine's startup window).
func (l *TCPLocalListener) Close() error {
	l.closeMu.Lock()
	if l.closed {
		l.closeMu.Unlock()
		return nil
	}
	l.closed = true
	close(l.closing)
	closeErr := l.listener.Close()
	l.wg.Wait()
	// serveStarted must be read under closeMu (Serve writes it under the lock);
	// but waiting on serveDone must happen outside the lock — otherwise the
	// final Serve loop cannot get closeMu to read closed and exit, while Close
	// holds the lock waiting on serveDone, deadlocking the two. Once released,
	// Serve acquires closeMu, reads closed=true, exits and closes serveDone,
	// and Close just waits for it to return.
	serveStarted := l.serveStarted
	l.closeMu.Unlock()
	if serveStarted {
		<-l.serveDone
	}
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
	mu         sync.Mutex
	registered bool
	sessionID  string
	serviceID  string
}

func (h *providerSessionHandler) state() (registered bool, sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.registered, h.sessionID
}

func (h *providerSessionHandler) markRegistered(sessionID, serviceID string) {
	h.mu.Lock()
	h.sessionID, h.serviceID, h.registered = sessionID, serviceID, true
	h.mu.Unlock()
}

// Handle processes inbound requests on a Provider TCP session.
// It enforces the SDK-Agent subprotocol handshake:
//   - Before registration: only MsgProviderConnectRequest is accepted
//     (InvokeRequest from an unregistered client is rejected — Invokers are
//     not a bypass for the Provider handshake).
//   - After registration: MsgProviderConnectRequest is rejected (duplicate).
//   - After registration: Heartbeat must reference the session-bound session_id.
func (h *providerSessionHandler) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	registered, sessionID := h.state()
	// Enforce handshake: first frame must be ProviderConnectRequest.
	// InvokeRequest is NOT a valid first frame — it is only valid as an
	// outbound Agent→Provider message on an established session, not as an
	// inbound client→Agent message before connect. Rejecting it here closes
	// the gap where an Invoker could bypass the Provider handshake.
	if !registered && msgID != protocol.MsgProviderConnectRequest {
		return nil, tcptr.NewProtocolError(fmt.Errorf(
			"protocol violation: first frame must be %s, got %s",
			protocol.MsgIDString(protocol.MsgProviderConnectRequest),
			protocol.MsgIDString(msgID)))
	}

	// After registration, reject duplicate connect.
	if registered && msgID == protocol.MsgProviderConnectRequest {
		return nil, tcptr.NewProtocolError(fmt.Errorf(
			"protocol violation: session already connected as %s", sessionID))
	}

	switch msgID {
	case protocol.MsgProviderConnectRequest:
		return h.handleConnect(ctx, body)
	case protocol.MsgProviderHeartbeatRequest:
		return h.handleHeartbeat(ctx, body)
	case protocol.MsgProviderDrainRequest:
		return h.handleDrain(ctx, body)
	default:
		// Delegate to local handler for other message types.
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

	// Handshake auth (CR-FIX-11): validated before the session is registered;
	// a rejection returns ProtocolError so MuxConn closes the connection
	// (fail-closed, no half-registered session). Token extraction must happen
	// before metadata merging — merging drops reserved keys (including
	// authToken).
	if require, token := h.listener.config.RequireProviderAuth, h.listener.config.ProviderAuthToken; require {
		if err := checkProviderAuth(require, token, extractProviderAuthToken(req)); err != nil {
			h.listener.logger.Warn("provider handshake auth rejected",
				"service_id", req.ServiceId, "remote", h.conn.RemoteAddr(), "error", err)
			return nil, tcptr.NewProtocolError(err)
		}
	}

	sessionID := fmt.Sprintf("ps-%d", time.Now().UnixNano())

	// User instance metadata enters the session with the connection (reserved
	// keys dropped with a warning; semantics identical to the local_handler
	// path); at onConnect registration time it is folded into the instance
	// metadata via MergeProviderInstanceMetadata. authToken is already a
	// reserved key and never enters session metadata (prevents leakage to the
	// SDK distribution page).
	metadata, metaWarnings := MergeUserProviderMetadata(map[string]string{}, req.GetMetadata())
	for _, warning := range metaWarnings {
		h.listener.logger.Warn("provider metadata reserved key dropped",
			"service_id", req.ServiceId, "session_id", sessionID, "warning", warning)
	}

	sess := sidecar.NewProviderSession(h.conn)
	sess.SessionID = sessionID
	sess.ServiceID = req.ServiceId
	sess.Version = req.Version
	sess.Functions = convertProtoFunctions(req.Functions)
	sess.ConnectedAt = time.Now()
	sess.SDKLanguage = req.SdkLanguage
	sess.SDKVersion = req.SdkVersion
	sess.SDKName = req.SdkName
	sess.Metadata = metadata

	sess.UpdateLastSeen()

	h.listener.sessionStore.Upsert(sess)
	h.markRegistered(sessionID, req.ServiceId)

	h.listener.logger.Info("Provider connected via TCP session",
		"service_id", req.ServiceId,
		"version", req.Version,
		"session_id", sessionID,
		"sdk_language", req.SdkLanguage,
		"sdk_version", req.SdkVersion,
		"sdk_name", req.SdkName,
		"functions", len(req.Functions),
		"user_metadata_keys", len(metadata),
		"remote", h.conn.RemoteAddr(),
	)

	// Notify onConnect callback
	if h.listener.onConnect != nil {
		h.listener.onConnect(sess)
	}

	resp := &sdkv1.ProviderConnectResponse{
		SessionId: sessionID,
		Warnings:  metaWarnings,
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
		// The session and local function table represent the same Provider.
		// Refresh both clocks so maintenance does not prune a live provider
		// between its normal SDK heartbeats.
		if h.listener.localHandler != nil {
			if _, err := h.listener.localHandler.Handle(ctx, protocol.MsgProviderHeartbeatRequest, 0, body); err != nil {
				return nil, err
			}
		}
	}

	resp := &sdkv1.ProviderHeartbeatResponse{}
	return proto.Marshal(resp)
}

func (h *providerSessionHandler) handleDrain(ctx context.Context, body []byte) ([]byte, error) {
	req := &sdkv1.ProviderDrainRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal ProviderDrainRequest: %w", err)
	}

	// SDK self-drain: the target is this very session. Requests often omit
	// session_id (an SDK can only drain itself); fall back to the session id
	// bound at handshake.
	targetSession := req.SessionId
	if targetSession == "" {
		_, sessionID := h.state()
		targetSession = sessionID
	}
	undrain := req.Reason == protocol.ProviderDrainReasonUndrain
	grace := defaultDrainGrace
	if req.RetryAfterMs > 0 {
		grace = time.Duration(req.RetryAfterMs) * time.Millisecond
	}

	h.listener.logger.Info("Provider drain requested",
		"session_id", targetSession,
		"reason", req.Reason,
		"retry_after_ms", req.RetryAfterMs,
	)

	if local, ok := h.listener.localHandler.(*LocalHandler); ok {
		if _, err := local.DrainProviderSession(ctx, targetSession, undrain, grace); err != nil {
			return nil, err
		}
	} else {
		// Non-LocalHandler wiring (tests / degraded): only reply, no session
		// removal — sessions are cleaned up by the disconnect path
		// (onDisconnect → RemoveProvider), matching the old behavior.
		h.listener.logger.Info("Provider drain acknowledged without shared LocalHandler",
			"session_id", targetSession, "undrain", undrain)
	}

	resp := &sdkv1.ProviderDrainResponse{}
	return proto.Marshal(resp)
}

// convertProtoFunctions converts proto ProviderFunctionDescriptor slices to the compat struct.
// The proto definition is the same type since we use the generated Go struct.
func convertProtoFunctions(funcs []*sdkv1.ProviderFunctionDescriptor) []*sdkv1.ProviderFunctionDescriptor {
	if funcs == nil {
		return nil
	}
	return funcs
}
