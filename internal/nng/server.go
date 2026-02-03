// Package nng provides NNG-based server implementation for Croupier control plane
package nng

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	serverv1 "github.com/cuihairu/croupier/pkg/pb/croupier/server/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	_ "go.nanomsg.org/mangos/v3/transport/tcp"
	"google.golang.org/protobuf/proto"
)

// Server implements an NNG-based control server (replaces gRPC ControlService)
type Server struct {
	addr     string
	sock     mangos.Socket
	registry *reg.Store

	// Session management
	defaultSessionTTL time.Duration

	// Metrics
	metricsStore    *reg.MetricsStore
	systemInfoCache *reg.SystemInfoCache

	// Upstream forwarding (for Edge mode)
	upstream Handler

	// State
	mu     sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	// Logging
	logger *slog.Logger
}

// Handler handles control service requests
type Handler interface {
	// HandleRegister handles agent registration
	HandleRegister(ctx context.Context, req *serverv1.RegisterRequest) (*serverv1.RegisterResponse, error)

	// HandleHeartbeat handles agent heartbeat
	HandleHeartbeat(ctx context.Context, req *serverv1.HeartbeatRequest) (*serverv1.HeartbeatResponse, error)

	// HandleRegisterCapabilities handles provider capabilities registration
	HandleRegisterCapabilities(ctx context.Context, req *serverv1.RegisterCapabilitiesRequest) (*serverv1.RegisterCapabilitiesResponse, error)
}

// NewServer creates a new NNG control server
func NewServer(addr string, registry *reg.Store) *Server {
	if registry == nil {
		registry = reg.NewStore()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		addr:             addr,
		registry:         registry,
		metricsStore:     reg.NewMetricsStore(),
		systemInfoCache:  reg.NewSystemInfoCache(),
		defaultSessionTTL: 5 * time.Minute,
		ctx:              ctx,
		cancel:           cancel,
		logger:           slog.Default(),
	}
}

// SetDefaultSessionTTL sets the default session TTL
func (s *Server) SetDefaultSessionTTL(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defaultSessionTTL = ttl
}

// SetUpstreamHandler sets an upstream handler for forwarding requests
func (s *Server) SetUpstreamHandler(h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upstream = h
}

// SetLogger sets the logger
func (s *Server) SetLogger(logger *slog.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger = logger
}

// Store returns the registry store
func (s *Server) Store() *reg.Store {
	return s.registry
}

// MetricsStore returns the metrics store
func (s *Server) MetricsStore() *reg.MetricsStore {
	return s.metricsStore
}

// SystemInfoCache returns the system info cache
func (s *Server) SystemInfoCache() *reg.SystemInfoCache {
	return s.systemInfoCache
}

// Start starts the NNG server
func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.running = true
	s.mu.Unlock()

	// Create REP socket
	sock, err := rep.NewSocket()
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}

	// Configure options
	if err := sock.SetOption(mangos.OptionRecvDeadline, time.Second); err != nil {
		sock.Close()
		return fmt.Errorf("failed to set recv deadline: %w", err)
	}

	// Listen (TCP transport is enabled via import side-effect)
	listenAddr := "tcp://" + s.addr
	if err := sock.Listen(listenAddr); err != nil {
		sock.Close()
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}

	s.sock = sock
	s.logger.Info("NNG Control server started", "addr", s.addr)

	// Start serving
	go s.serve()

	// Start metrics pruning
	go s.pruneOldMetrics()

	return nil
}

// Stop stops the NNG server
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()

	s.cancel()

	if s.sock != nil {
		if err := s.sock.Close(); err != nil {
			s.logger.Error("failed to close socket", "error", err)
		}
		s.sock = nil
	}

	s.logger.Info("NNG Control server stopped")
	return nil
}

// serve handles incoming requests
func (s *Server) serve() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		if s.sock == nil {
			return
		}

		// Receive message
		msg, err := s.sock.RecvMsg()
		if err != nil {
			// Check if context is cancelled
			select {
			case <-s.ctx.Done():
				return
			default:
			}
			// Timeout is expected due to RecvDeadline
			continue
		}

		// Parse protocol header from body prefix
		_, msgID, reqID, data, err := protocol.ParseMessageFromBody(msg.Body)
		msg.Free()
		if err != nil {
			s.logger.Error("failed to parse message", "error", err)
			continue
		}

		// Handle request based on message type
		respData, err := s.handleRequest(s.ctx, msgID, data)
		if err != nil {
			s.logger.Error("failed to handle request", "msgID", protocol.MsgIDString(msgID), "error", err)
			// Send error response
			respData = s.createErrorResponse(err)
		}

		// Create response with protocol header in body
		respMsgID := protocol.GetResponseMsgID(msgID)
		respBodyWithHeader := protocol.NewMessageBody(respMsgID, reqID, respData)

		respMsg := mangos.NewMessage(0)
		respMsg.Body = respBodyWithHeader

		if err := s.sock.SendMsg(respMsg); err != nil {
			s.logger.Error("failed to send response", "error", err)
		}
	}
}

// handleRequest dispatches request to appropriate handler based on msgID
func (s *Server) handleRequest(ctx context.Context, msgID uint32, data []byte) ([]byte, error) {
	switch msgID {
	case protocol.MsgRegisterRequest:
		return s.handleRegister(ctx, data)
	case protocol.MsgHeartbeatRequest:
		return s.handleHeartbeat(ctx, data)
	case protocol.MsgRegisterCapabilitiesReq:
		return s.handleRegisterCapabilities(ctx, data)
	default:
		return nil, fmt.Errorf("unknown message type: 0x%06X", msgID)
	}
}

// handleRegister handles RegisterRequest
func (s *Server) handleRegister(ctx context.Context, data []byte) ([]byte, error) {
	req := &serverv1.RegisterRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal RegisterRequest: %w", err)
	}

	resp, err := s.handleRegisterRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	return proto.Marshal(resp)
}

// handleHeartbeat handles HeartbeatRequest
func (s *Server) handleHeartbeat(ctx context.Context, data []byte) ([]byte, error) {
	req := &serverv1.HeartbeatRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal HeartbeatRequest: %w", err)
	}

	resp, err := s.handleHeartbeatRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	return proto.Marshal(resp)
}

// handleRegisterCapabilities handles RegisterCapabilitiesRequest
func (s *Server) handleRegisterCapabilities(ctx context.Context, data []byte) ([]byte, error) {
	req := &serverv1.RegisterCapabilitiesRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal RegisterCapabilitiesRequest: %w", err)
	}

	resp, err := s.handleRegisterCapabilitiesRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	return proto.Marshal(resp)
}

// handleRegisterRequest implements the actual Register logic
func (s *Server) handleRegisterRequest(ctx context.Context, req *serverv1.RegisterRequest) (*serverv1.RegisterResponse, error) {
	// Forward to upstream if configured
	if s.upstream != nil {
		return s.upstream.HandleRegister(ctx, req)
	}

	// Calculate TTL (default 1 day)
	ttl := 24 * time.Hour
	if req.TtlSeconds > 0 {
		ttl = time.Duration(req.TtlSeconds) * time.Second
	}

	sess := &reg.AgentSession{
		AgentID:   req.AgentId,
		GameID:    req.GameId,
		Env:       req.Env,
		RPCAddr:   req.RpcAddr,
		Version:   req.Version,
		Region:    "",
		Zone:      "",
		Labels:    map[string]string{},
		ExpireAt:  time.Now().Add(ttl),
		LastSeen:  time.Now(),
		Functions: map[string]reg.FunctionMeta{},
	}

	// Populate functions from request
	for _, f := range req.Functions {
		if f == nil || f.Id == "" {
			continue
		}
		sess.Functions[f.Id] = reg.FunctionMeta{
			Enabled: f.Enabled,
			Version: f.Version,
		}
	}

	// Populate processes from request
	if len(req.Processes) > 0 {
		processes := make([]reg.ProcessSession, 0, len(req.Processes))
		for _, p := range req.Processes {
			if p == nil || p.ServiceId == "" {
				continue
			}
			processes = append(processes, reg.ProcessSession{
				ServiceID:    p.ServiceId,
				Addr:         p.Addr,
				Version:      p.Version,
				LastSeenUnix: p.LastSeenUnix,
				FunctionIDs:  p.FunctionIds,
			})
		}
		sess.Processes = processes
	}

	s.registry.UpsertAgent(sess)

	s.logger.Info("Agent registered via NNG", "agent_id", req.AgentId, "game_id", req.GameId)

	return &serverv1.RegisterResponse{}, nil
}

// handleHeartbeatRequest implements the actual Heartbeat logic
func (s *Server) handleHeartbeatRequest(ctx context.Context, req *serverv1.HeartbeatRequest) (*serverv1.HeartbeatResponse, error) {
	// Forward to upstream if configured
	if s.upstream != nil {
		return s.upstream.HandleHeartbeat(ctx, req)
	}

	if req.AgentId == "" {
		return &serverv1.HeartbeatResponse{}, nil
	}

	s.registry.Mu().Lock()
	agent := s.registry.AgentsUnsafe()[req.AgentId]
	if agent != nil {
		agent.ExpireAt = time.Now().Add(s.defaultSessionTTL)
		agent.LastSeen = time.Now()
	}
	s.registry.Mu().Unlock()

	return &serverv1.HeartbeatResponse{}, nil
}

// handleRegisterCapabilitiesRequest implements the actual RegisterCapabilities logic
func (s *Server) handleRegisterCapabilitiesRequest(ctx context.Context, req *serverv1.RegisterCapabilitiesRequest) (*serverv1.RegisterCapabilitiesResponse, error) {
	// Forward to upstream if configured
	if s.upstream != nil {
		return s.upstream.HandleRegisterCapabilities(ctx, req)
	}

	if req.Provider == nil || req.Provider.Id == "" {
		return nil, fmt.Errorf("provider metadata is required")
	}

	// Decompress manifest JSON
	manifestData, err := s.decompressManifest(req.ManifestJsonGz)
	if err != nil {
		return nil, fmt.Errorf("invalid manifest (gzip): %w", err)
	}

	// Store provider capabilities
	providerCaps := reg.ProviderCaps{
		ID:        req.Provider.Id,
		Version:   req.Provider.Version,
		Lang:      req.Provider.Lang,
		SDK:       req.Provider.Sdk,
		Manifest:  manifestData,
		UpdatedAt: time.Now(),
	}

	s.registry.UpsertProviderCaps(providerCaps)

	s.logger.Info("Provider capabilities registered via NNG", "provider_id", req.Provider.Id)

	return &serverv1.RegisterCapabilitiesResponse{}, nil
}

// decompressManifest decompresses gzipped manifest data
func (s *Server) decompressManifest(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("manifest data is empty")
	}

	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// createErrorResponse creates an error response
func (s *Server) createErrorResponse(err error) []byte {
	// For now, return a simple error message
	// In production, this should be a proper protobuf error response
	resp := &serverv1.RegisterResponse{}
	// Can't directly add error to RegisterResponse, so we log it
	s.logger.Error("Request error", "error", err)
	data, _ := proto.Marshal(resp)
	return data
}

// pruneOldMetrics periodically prunes old metrics entries
func (s *Server) pruneOldMetrics() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.metricsStore.Prune(time.Hour)
			s.systemInfoCache.Prune(time.Hour)
		}
	}
}

// GetStats returns server statistics
func (s *Server) GetStats() map[string]interface{} {
	s.registry.Mu().RLock()
	defer s.registry.Mu().RLock()

	agents := s.registry.AgentsUnsafe()

	return map[string]interface{}{
		"agent_count": len(agents),
		"address":     s.addr,
		"running":     s.running,
		"session_ttl": s.defaultSessionTTL.String(),
	}
}

// GetLocalAddr returns the configured listening address
func (s *Server) GetLocalAddr() (string, error) {
	if s.addr == "" {
		return "", fmt.Errorf("address not configured")
	}
	return s.addr, nil
}

// ControlHandler wraps Server to implement Handler interface
type ControlHandler struct {
	server *Server
}

// NewControlHandler creates a new control handler
func NewControlHandler(server *Server) *ControlHandler {
	return &ControlHandler{server: server}
}

func (h *ControlHandler) HandleRegister(ctx context.Context, req *serverv1.RegisterRequest) (*serverv1.RegisterResponse, error) {
	return h.server.handleRegisterRequest(ctx, req)
}

func (h *ControlHandler) HandleHeartbeat(ctx context.Context, req *serverv1.HeartbeatRequest) (*serverv1.HeartbeatResponse, error) {
	return h.server.handleHeartbeatRequest(ctx, req)
}

func (h *ControlHandler) HandleRegisterCapabilities(ctx context.Context, req *serverv1.RegisterCapabilitiesRequest) (*serverv1.RegisterCapabilitiesResponse, error) {
	return h.server.handleRegisterCapabilitiesRequest(ctx, req)
}
