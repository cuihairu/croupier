package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	"github.com/cuihairu/croupier/internal/platform/devopssessions"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
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

	// RequireAuth 收紧 agent 注册凭据（AUD-040）：true 时监听面必须处于 mTLS
	// 状态（CAFile 配置 + 非 Insecure），否则 NewTCPListener 拒绝启动（fail-
	// closed）；注册时还要求对端携带已校验的客户端证书身份。缺省 false =
	// 兼容存量部署（insecure 仍可启动，但启动日志 WARN 提示未加固）。
	RequireAuth bool

	// AllowedGameEnvs 是 agent 注册可声明的作用域白名单（AUD-040 game/env
	// 授权）：非空时注册声明的 (game_id, env) 必须命中，否则拒绝；空 = 不限制
	// （兼容存量，授权细粒度化登记为后续）。按 (GameID, Env) 精确匹配。
	AllowedGameEnvs []GameEnvScope
}

// GameEnvScope 是 agent 注册声明作用域白名单条目（AUD-040）。运行时值，
// 不做 YAML/JSON 解码（解码面在 config.ControlConfig.allowedGameEnvs）。
type GameEnvScope struct {
	GameID string
	Env    string
}

// TCPListener accepts Agent TCP sessions and creates AgentSession entries.
//
// Lifecycle:
//  1. Agent dials in → first frame must be RegisterRequest
//  2. Server validates and creates AgentSession → stores in AgentSessionStore
//  3. Subsequent Heartbeat/Invoke/StartTask/CancelTask flow through MuxConn
//  4. On disconnect, session is removed from store
type TCPListener struct {
	config       *TCPListenerConfig
	listener     net.Listener
	sessionStore *AgentSessionStore
	// clusterHooks 集群归属钩子（断连释放归属；nil = 未启用）。
	clusterHooks interface {
		OnAgentDisconnected(ctx context.Context, agentID string)
	}
	// agentDisconnectReaper（ARCH-014 D3）：agent 会话被移除时收敛该 agent 的
	// 非终态 task_runs 到 unknown。nil = 未装配（任务行不被收割）。装配期由
	// cmd/server 注入具体 task store 的闭包。
	agentDisconnectReaper func(ctx context.Context, agentID string) error
	// devopsSessions devops 会话中继运营面（AGENT-002）：会话被真正移除时
	// 收束该 agent 的全部代理会话（控制面丢失 = 会话面关闭）。nil = 未装配。
	devopsSessions *devopssessions.Hub
	registry       *reg.Store
	handler        *ControlService // control-plane service

	wg      sync.WaitGroup
	closing chan struct{}
	// stopOnce guards the accept-loop teardown (StopAccepting). Close
	// delegates to StopAccepting before waiting on wg, so the once must
	// cover the shared teardown, not the connection wait.
	stopOnce sync.Once
	// activeConns counts live serveConn goroutines. Purely observational
	// (shutdown orchestration waits on wg); tests poll it to know a dialed
	// connection was actually accepted before exercising the wait paths.
	activeConns atomic.Int64

	// serveStarted/serveExited bracket the accept loop: Close drains the
	// loop before waiting on wg so the WaitGroup contract (the first
	// positive delta must happen-before Wait) holds structurally.
	serveStarted   chan struct{}
	serveExited    chan struct{}
	serveStartOnce sync.Once
	serveExitOnce  sync.Once

	logger *slog.Logger
}

// agentIdentityKey carries the server-verified registration identity through
// control-plane dispatch so uplink payloads cannot override Agent scope.
type agentIdentityKey struct{}

// AgentIdentity is the scope bound to a live Agent session.
type AgentIdentity struct {
	AgentID string
	GameID  string
	Env     string
}

// AgentIdentityFromContext returns the verified identity attached to an Agent
// control connection. Nil means the request did not enter through one.
func AgentIdentityFromContext(ctx context.Context) *AgentIdentity {
	if ctx == nil {
		return nil
	}
	identity, _ := ctx.Value(agentIdentityKey{}).(*AgentIdentity)
	return identity
}

func withAgentIdentity(ctx context.Context, identity *AgentIdentity) context.Context {
	return context.WithValue(ctx, agentIdentityKey{}, identity)
}

// defaultControlAddress is the Server-side Agent control-plane listen address
// used when no configuration is supplied. Kept as a named constant so tests can
// assert the default without binding the port (see TestNewTCPListener_NilConfigDefaults).
const defaultControlAddress = ":19090"

// defaultTCPListenerConfig returns config with nil fields defaulted.
//
// Split out of NewTCPListener so the defaulting contract is testable without
// binding a fixed port: a test that constructs a listener with the real default
// address fails whenever a local Server (or a parallel test) already owns it.
func defaultTCPListenerConfig(config *TCPListenerConfig) *TCPListenerConfig {
	if config != nil {
		return config
	}
	return &TCPListenerConfig{Address: defaultControlAddress, Insecure: true}
}

// NewTCPListener creates a new TCP session listener.
func NewTCPListener(config *TCPListenerConfig, sessionStore *AgentSessionStore, registry *reg.Store, logger *slog.Logger) (*TCPListener, error) {
	config = defaultTCPListenerConfig(config)
	if sessionStore == nil {
		sessionStore = NewAgentSessionStore()
	}
	if logger == nil {
		logger = slog.Default()
	}
	if registry == nil {
		registry = reg.NewStore()
	}

	// AUD-040：RequireAuth 是显式加固开关（insecure 默认值收紧，兼容存量部署）。
	// 置位后监听面必须处于 mTLS（CAFile 配置 + 非明文），否则 fail-closed 拒绝
	// 启动——不允许「声称要鉴权却跑明文/服务端证书-only」。未置位时明文/无
	// 客户端证书校验仍可启动（兼容存量），但 WARN 提示该缺口（生产应加固）。
	if config.RequireAuth {
		if config.Insecure || config.CAFile == "" {
			return nil, fmt.Errorf("control listener RequireAuth=true requires mTLS (CAFile configured and Insecure=false); refusing to start unauthenticated listener")
		}
	} else if config.Insecure || config.CAFile == "" {
		logger.Warn("agent control listener is not authenticated: no mTLS client-certificate verification (set control.ca + control.requireAuth=true to enforce)")
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
		serveStarted: make(chan struct{}),
		serveExited:  make(chan struct{}),
		logger:       logger,
	}, nil
}

// SetHandler sets the control service for processing register/heartbeat.
func (l *TCPListener) SetHandler(h *ControlService) {
	l.handler = h
}

// Serve accepts connections until ctx is done or the listener is closed.
func (l *TCPListener) Serve(ctx context.Context) error {
	l.serveStartOnce.Do(func() { close(l.serveStarted) })
	defer l.serveExitOnce.Do(func() { close(l.serveExited) })

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
		l.activeConns.Add(1)
		go func() {
			defer func() {
				l.activeConns.Add(-1)
				l.wg.Done()
			}()
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
		_ = conn.Close()
		l.logger.Info("Agent TCP connection closed", "remote", remoteAddr)
	}()

	muxCfg := &tcptr.Config{
		RecvTimeout: l.config.RecvTimeout,
		SendTimeout: l.config.SendTimeout,
	}

	// AUD-040：tls.Listener 的握手惰性发生在首次读写，而 accept 后的
	// ConnectionState 尚未产出对端证书——身份必须在注册门禁读取前确定，
	// 这里显式提前完成握手（失败即断开，客户端证书校验失败以 TLS alert
	// 呈现，而非在连接深处误报成注册错误）。握手受 deadline 约束：对端
	// 卡在握手阶段时在此期限内断开，不悬挂 goroutine。
	if tlsConn, ok := conn.(*tls.Conn); ok {
		handshakeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := tlsConn.HandshakeContext(handshakeCtx)
		cancel()
		if err != nil {
			l.logger.Warn("Agent TLS handshake failed", "remote", remoteAddr, "error", err)
			return
		}
	}

	// Create a handler that dispatches through the control plane logic
	// and also tracks AgentSession lifecycle.
	handler := &agentSessionHandler{
		listener:   l,
		conn:       nil, // set below
		registered: false,
		agentID:    "",
		sessionID:  "",
		// AUD-040：对端 mTLS 身份（握手已完成，State.PeerCertificates 非空），
		// 注册门禁据此判断连接是否携带已校验客户端证书。
		peerIdentity: peerIdentityFromConn(conn),
	}

	mux := tcptr.NewMuxConn(conn, muxCfg, handler)
	handler.conn = mux

	// Run blocks until connection closes or ctx is done.
	if err := mux.Run(ctx); err != nil {
		l.logger.Debug("Agent MuxConn ended", "remote", remoteAddr, "error", err)
	}
	// Join lane workers before reading handler state: registration runs on a
	// worker goroutine, and without this wait its writes to handler fields
	// race the disconnect cleanup below.
	mux.Wait()

	// Clean up session on disconnect.
	// Use RemoveSession (not Remove) so that an old connection's cleanup
	// does not delete a newer session that replaced it during a reconnect.
	if handler.agentID != "" {
		disconnectedAt := time.Now()
		removed := l.sessionStore.RemoveSession(handler.agentID, handler.sessionID)
		if removed {
			l.logger.Info("Agent session removed on disconnect",
				"agent_id", handler.agentID,
				"session_id", handler.sessionID)
			// 同步清 registry 内存条目（函数视图/lb/broadcast 候选即时
			// 真实化——此前僵尸滞留到 ExpireAt 24h）。notAfter 校验挡住
			// 「断连瞬间重连注册已覆盖 registry」的竞态：新注册的
			// LastSeen 必然晚于断连时刻。
			if l.registry != nil && l.registry.RemoveAgentIfStale(handler.agentID, disconnectedAt) {
				l.logger.Info("Removed registry session on disconnect",
					"agent_id", handler.agentID)
			}
			if l.clusterHooks != nil {
				l.clusterHooks.OnAgentDisconnected(context.Background(), handler.agentID)
			}
			// ARCH-014 D3：会话被真正移除后收敛该 agent 的非终态任务到
			// unknown（执行状态未确立，不得自动重跑）。收割失败仅记日志：
			// 断连清理不得阻断会话移除。
			if l.agentDisconnectReaper != nil {
				if err := l.agentDisconnectReaper(context.Background(), handler.agentID); err != nil {
					l.logger.Warn("task reap on agent disconnect failed",
						"agent_id", handler.agentID, "error", err)
				}
			}
			// AGENT-002：控制面丢失 = 会话面关闭——收束该 agent 的全部代理
			// 会话（与 agent 侧 OnConnClosed 对称；重连后面板须重新开会话）。
			if l.devopsSessions != nil {
				l.devopsSessions.CloseAgentSessions(handler.agentID, "agent disconnected")
			}
		} else {
			l.logger.Debug("Agent session already replaced, skipping removal",
				"agent_id", handler.agentID,
				"session_id", handler.sessionID)
		}
	}
}

// SetClusterHooks 注入集群归属钩子（装配期调用）。
func (l *TCPListener) SetClusterHooks(h interface {
	OnAgentDisconnected(ctx context.Context, agentID string)
}) {
	l.clusterHooks = h
}

// SetAgentDisconnectReaper 注入 agent 断连任务收割回调（ARCH-014 D3）。装配
// 期由 cmd/server 绑定 task store 的 ReapAgentNonTerminal 闭包；nil 或零值
// 关闭收割。
func (l *TCPListener) SetAgentDisconnectReaper(reap func(ctx context.Context, agentID string) error) {
	l.agentDisconnectReaper = reap
}

// SetDevopsSessions 注入 devops 会话中继运营面（AGENT-002）。会话被真正
// 移除（非重连替换竞态）时收束该 agent 的全部代理会话。
func (l *TCPListener) SetDevopsSessions(h *devopssessions.Hub) {
	l.devopsSessions = h
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

// StopAccepting closes the accept loop and waits for Serve to return, without
// waiting for already-accepted connections (ARCH-017). The graceful-shutdown
// path needs exactly this: waiting for active sessions before cancelling the
// root context deadlocks, because sessions only end after the cancel.
//
// Ordering contract: the accept loop is drained before anything may wait on
// wg (its exit implies every wg.Add already happened). Waiting concurrently
// with a first wg.Add (counter at zero) violates the WaitGroup contract and
// is reported by the race detector via the sync.WaitGroup annotations.
func (l *TCPListener) StopAccepting() error {
	var closeErr error
	l.stopOnce.Do(func() {
		close(l.closing)
		closeErr = l.listener.Close()
		select {
		case <-l.serveStarted:
			<-l.serveExited
		default:
			// Serve was never started: no connection goroutines exist.
		}
	})
	return closeErr
}

// WaitConnections waits for already-accepted connection goroutines to finish,
// bounded by ctx (ARCH-017: shutdown cleanup must be bounded, not wait
// forever on a stuck session). The caller must StopAccepting (or Close) first
// so no new connection goroutines can be added while waiting. Returns
// ctx.Err() when the bound expires before the connections drain.
func (l *TCPListener) WaitConnections(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ActiveConnections reports the number of live connection goroutines
// (observability and tests; shutdown waits on wg, not on this counter).
func (l *TCPListener) ActiveConnections() int64 {
	return l.activeConns.Load()
}

// Close stops accepting new connections and waits for active ones to finish.
// Full-join semantics: StopAccepting followed by an unbounded wg wait. The
// graceful-shutdown orchestration in cmd/server uses the split pair
// (StopAccepting + WaitConnections) so HTTP shutdown and the root cancel are
// not blocked by long-lived sessions; Close remains for callers that want a
// fully drained listener (tests, fixtures).
func (l *TCPListener) Close() error {
	closeErr := l.StopAccepting()
	l.wg.Wait()
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
	mu         sync.Mutex
	registered bool
	agentID    string
	sessionID  string // assigned on successful registration
	// peerIdentity 是本次连接对端经 TLS 校验的客户端证书身份（AUD-040）：
	// 空 = 明文连接或未配置 CA（无 mTLS 身份）。RequireAuth=true 时注册必须
	// 非空（服务端启动已保证监听面处于 mTLS，此为注册级二次防线）。
	peerIdentity string
}

func (h *agentSessionHandler) state() (registered bool, agentID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.registered, h.agentID
}

func (h *agentSessionHandler) markRegistered(agentID, sessionID string) {
	h.mu.Lock()
	h.agentID, h.sessionID, h.registered = agentID, sessionID, true
	h.mu.Unlock()
}

// Handle processes inbound requests on an Agent TCP session.
// It enforces a strict handshake state machine:
//   - Before registration: only MsgRegisterRequest is accepted
//   - After registration: MsgRegisterRequest is rejected (duplicate registration)
//   - After registration: Heartbeat must carry the same agent_id as the session
func (h *agentSessionHandler) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	registered, agentID := h.state()
	// Enforce handshake state machine:
	// 1. Before registration, only RegisterRequest is allowed.
	if !registered && msgID != protocol.MsgRegisterRequest {
		return nil, fmt.Errorf("protocol violation: must register first before sending %s", protocol.MsgIDString(msgID))
	}

	// 2. After initial registration, allow re-register to update functions.
	// Agent sends a full RegisterRequest on every function change (e.g. 0 then
	// 19 as SDK providers register). Treat same-agent re-register as an
	// idempotent upsert; reject if agent_id changes (anti-spoofing).
	if registered && msgID == protocol.MsgRegisterRequest {
		req := &agentv1.RegisterRequest{}
		if err := proto.Unmarshal(body, req); err != nil {
			return nil, fmt.Errorf("unmarshal RegisterRequest: %w", err)
		}
		if req.AgentId != agentID {
			return nil, fmt.Errorf("protocol violation: re-register agent_id=%q does not match registered %q",
				req.AgentId, agentID)
		}
		// Same agent: allow — fall through to handleRegister via the switch below.
	}

	// 3. After registration, Heartbeat must carry the same agent_id as the
	//    session-bound ID, preventing cross-agent ID spoofing on a live conn.
	if registered && msgID == protocol.MsgHeartbeatRequest {
		if err := h.validateHeartbeatAgentID(body); err != nil {
			return nil, err
		}
	}

	switch msgID {
	case protocol.MsgRegisterRequest:
		return h.handleRegister(ctx, body)
	case protocol.MsgHeartbeatRequest:
		return h.handleHeartbeat(ctx, body)
	default:
		// Delegate to the control service if available
		if h.listener.handler != nil {
			identity := &AgentIdentity{AgentID: agentID}
			if sess, ok := h.listener.sessionStore.Get(agentID); ok {
				identity.GameID = sess.GameID
				identity.Env = sess.Env
			}
			return h.listener.handler.TransportHandler().Handle(withAgentIdentity(ctx, identity), msgID, reqID, body)
		}
		return nil, fmt.Errorf("unsupported message type for Agent session: %s", protocol.MsgIDString(msgID))
	}
}

// validateHeartbeatAgentID ensures a Heartbeat's agent_id matches the
// agent_id bound to this session at registration time.
func (h *agentSessionHandler) validateHeartbeatAgentID(body []byte) error {
	req := &agentv1.HeartbeatRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		// Let handleHeartbeat surface the unmarshal error; do not block here.
		return nil
	}
	_, agentID := h.state()
	if req.AgentId != agentID {
		return fmt.Errorf("protocol violation: heartbeat agent_id=%q does not match registered agent_id=%q",
			req.AgentId, agentID)
	}
	return nil
}

// validateRegisterAuthorization 是 AUD-040 的注册凭据与作用域门禁（注册成功即
// 绑定连接、scope 与函数表，门禁必须在物化会话之前拒绝）：
//   - RequireAuth=true（insecure 默认值收紧后的显式加固）：要求本次连接携带经
//     TLS 校验的客户端证书身份；明文或无客户端证书的连接拒绝注册。启动期
//     NewTCPListener 已保证 RequireAuth 时监听面处于 mTLS（否则拒绝启动），
//     此处为注册级二次防线。
//   - AllowedGameEnvs 非空：注册声明的 (game_id, env) 必须命中白名单，否则拒绝
//     （game/env 授权）；空白名单 = 不限制（兼容存量部署）。
func (h *agentSessionHandler) validateRegisterAuthorization(agentID, gameID, env string) error {
	cfg := h.listener.config
	if cfg == nil {
		return nil
	}
	if cfg.RequireAuth && h.peerIdentity == "" {
		return fmt.Errorf("registration requires a verified mTLS client identity (agent %q)", agentID)
	}
	if len(cfg.AllowedGameEnvs) > 0 {
		for _, allowed := range cfg.AllowedGameEnvs {
			if allowed.GameID == gameID && allowed.Env == env {
				return nil
			}
		}
		return fmt.Errorf("agent %q is not authorized for game_id=%q env=%q", agentID, gameID, env)
	}
	return nil
}

func (h *agentSessionHandler) handleRegister(ctx context.Context, body []byte) ([]byte, error) {
	req := &agentv1.RegisterRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal RegisterRequest: %w", err)
	}

	agentID := strings.TrimSpace(req.AgentId)
	gameID := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)
	if agentID == "" || gameID == "" || env == "" {
		return nil, fmt.Errorf("agent_id, game_id and env are required")
	}
	req.AgentId, req.GameId, req.Env = agentID, gameID, env

	// AUD-040：凭据与作用域授权门禁（注册成功即绑定连接、scope、函数表，
	// 必须在物化会话前拒绝，避免未授权 agent 劫持注册）。
	if err := h.validateRegisterAuthorization(agentID, gameID, env); err != nil {
		return nil, err
	}

	// Compute session TTL
	ttl := 5 * time.Minute
	if req.TtlSeconds > 0 {
		ttl = time.Duration(req.TtlSeconds) * time.Second
	}
	expireAt := time.Now().Add(ttl)

	// Create Agent session. RpcAddr is kept only as a legacy compatibility
	// mirror from the register request; the active TCP session is the primary
	// runtime route.
	sess := &AgentSession{
		conn:        h.conn,
		AgentID:     req.AgentId,
		SessionID:   fmt.Sprintf("as-%d", time.Now().UnixNano()),
		GameID:      req.GameId,
		Env:         req.Env,
		Version:     req.Version,
		ConnectedAt: time.Now(),
	}
	sess.UpdateLastSeen()

	// Store session (carries the live MuxConn for request forwarding). Fail
	// closed so a same-ID agent cannot overwrite a live session from another
	// game environment while the control-plane registry is still validating.
	if err := h.listener.sessionStore.Upsert(sess); err != nil {
		return nil, fmt.Errorf("bind agent session: %w", err)
	}
	h.markRegistered(req.AgentId, sess.SessionID)

	// Register declared functions into the dispatcher registry. The session
	// store above carries the live connection; the registry carries the
	// function table that pickAgentWithRouting consults. Without this, the
	// registry's Functions map stays empty and every dispatch fails with
	// "no live agent for function".
	remoteAddr := ""
	if h.conn != nil {
		remoteAddr = h.conn.RemoteAddr()
	}

	var regWarnings []string
	if h.listener.handler != nil {
		if rr, err := h.listener.handler.handleRegisterRequest(ctx, req, remoteAddr); err == nil {
			regWarnings = rr.GetWarnings()
		} else {
			h.listener.logger.Warn("register functions to dispatcher registry failed",
				"agent_id", req.AgentId, "error", err)
			// 注册错误黑洞修复：物化失败仍回成功（避免 agent 重连风暴），
			// 但必须经 warnings 显式回传，否则 agent 侧完全不可见，
			// 调用端只会看到 "no live agent for function"。
			regWarnings = append(regWarnings, "registration_materialize_failed: "+err.Error())
		}
	}

	h.listener.logger.Info("Agent registered via TCP session",
		"agent_id", req.AgentId,
		"game_id", req.GameId,
		"session_id", sess.SessionID,
		"functions", len(req.GetFunctions()),
		"remote", remoteAddr,
	)

	// Build response
	resp := &agentv1.RegisterResponse{
		SessionId: sess.SessionID,
		ExpireAt:  expireAt.Unix(),
		Warnings:  regWarnings,
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

	// 完整心跳链路必须走 ControlService：registry 内存续期 + 集群
	// owner Touch（跨实例转发路由依据）+ DB 异步落盘。此前的捷径只
	// 更新 sessionStore 就应答——owner 表 3 分钟后过期、拓扑页
	// agentCount 归零、跨实例调用解析不到 owner 全部源于这里被绕过。
	if h.listener.handler != nil {
		if _, err := h.listener.handler.handleHeartbeatRequest(ctx, req); err != nil {
			h.listener.logger.Warn("heartbeat full-path handling failed",
				"agent_id", req.AgentId, "error", err)
		}
	}

	resp := &agentv1.HeartbeatResponse{}
	return proto.Marshal(resp)
}

// listenTCP creates a net.Listener based on the config (plain or TLS).
func listenTCP(config *TCPListenerConfig) (net.Listener, error) {
	addr := tcpListenAddress(config)

	if config.Insecure {
		return net.Listen("tcp", addr)
	}

	// AUD-040：握手整体在 serveConn 里用带 deadline 的 HandshakeContext 完成
	//（tls.Config 无 HandshakeTimeout 字段，超时经 ctx 传递），避免对端卡在
	// 握手阶段时悬挂 goroutine。
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

// peerIdentityFromConn 从已建立的连接推导对端经 TLS 校验的客户端证书身份
// （AUD-040）。非 TLS 连接（明文）或 TLS 但未强制客户端证书（服务端证书-only）
// 返回空串；有已校验客户端证书时返回其身份串，供 RequireAuth 注册门禁与日志留证。
func peerIdentityFromConn(conn net.Conn) string {
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return ""
	}
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return ""
	}
	return certificateIdentity(state.PeerCertificates[0])
}

// certificateIdentity 从客户端证书派生稳定身份串：优先 CommonName，缺 CN 时
// 用第一个 DNS SAN；均无则不可推导（返回空，交由 RequireAuth 拒绝注册）。
func certificateIdentity(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	if cn := strings.TrimSpace(cert.Subject.CommonName); cn != "" {
		return cn
	}
	if len(cert.DNSNames) > 0 {
		if san := strings.TrimSpace(cert.DNSNames[0]); san != "" {
			return san
		}
	}
	return ""
}

// tcpListenAddress resolves the configured listener address without opening a
// socket. Keeping this separate lets callers and tests verify the default
// without making a fixed production port part of the test environment.
func tcpListenAddress(config *TCPListenerConfig) string {
	if config == nil || config.Address == "" {
		return ":19090"
	}
	return config.Address
}
