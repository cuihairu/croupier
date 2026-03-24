// Package nng provides NNG-based server implementation for Croupier control plane
package nng

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/function/converter"
	"github.com/cuihairu/croupier/internal/platform/registry"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	_ "go.nanomsg.org/mangos/v3/transport/ipc"
	_ "go.nanomsg.org/mangos/v3/transport/tcp"
	"google.golang.org/protobuf/proto"
)

var (
	functionIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:\.[a-z0-9]+(?:[._-][a-z0-9]+)*)+$`)
	semverPattern     = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
)

type registerWarning struct {
	Code       string
	FunctionID string
	Version    string
	Message    string
}

// ListenAddr represents a single listen address with transport type
type ListenAddr struct {
	Addr      string // Raw address (e.g., ":19090", "ipc://croupier-server")
	Transport string // Transport type: "tcp", "ipc", etc.
	URL       string // Full URL for NNG (e.g., "tcp://:19090", "ipc://croupier-server")
}

// ParseListenAddr parses a string address into a ListenAddr
func ParseListenAddr(addr string) ListenAddr {
	// If already has transport prefix, use as-is
	if strings.Contains(addr, "://") {
		parts := strings.SplitN(addr, "://", 2)
		return ListenAddr{
			Addr:      parts[1],
			Transport: parts[0],
			URL:       addr,
		}
	}

	// Default to TCP
	return ListenAddr{
		Addr:      addr,
		Transport: "tcp",
		URL:       "tcp://" + addr,
	}
}

// IsLocalTCP checks if an address is a local TCP address
func IsLocalTCP(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// Might be just a host, try parsing as-is
		host = addr
	}

	// Remove brackets from IPv6 addresses
	host = strings.Trim(host, "[]")

	// Check for localhost variants
	switch strings.ToLower(host) {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// AgentSessionLoader defines the interface for loading and managing agent sessions from a database.
// This allows the Server to be decoupled from specific database implementations.
type AgentSessionLoader interface {
	LoadActiveSessions(ctx context.Context) ([]*reg.AgentSession, error)
	Upsert(ctx context.Context, sess *reg.AgentSession) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// Server implements an NNG-based control server (replaces gRPC ControlService)
type Server struct {
	addrs              []ListenAddr // Multiple listen addresses
	sock               mangos.Socket
	sockets            []mangos.Socket // Multiple sockets for multi-listener
	registry           *reg.Store
	agentSessionLoader AgentSessionLoader // Interface for database operations

	// Session management
	defaultSessionTTL time.Duration

	// Metrics
	metricsStore    *reg.MetricsStore
	systemInfoCache *reg.SystemInfoCache

	// Upstream forwarding (for Edge mode)
	upstream Handler

	// State
	mu      sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	// Logging
	logger *slog.Logger

	backgroundOnce sync.Once
}

// Handler handles control service requests
type Handler interface {
	// HandleRegister handles agent registration
	HandleRegister(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error)

	// HandleHeartbeat handles agent heartbeat
	HandleHeartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error)

	// HandleRegisterCapabilities handles provider capabilities registration
	HandleRegisterCapabilities(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error)
}

// NewServer creates a new NNG control server
// addr can be a single address or comma-separated multiple addresses
// Examples: ":19090" or ":19090,ipc://croupier-server"
func NewServer(addr string, registry *reg.Store) *Server {
	if registry == nil {
		registry = reg.NewStore()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Parse addresses
	var addrs []ListenAddr
	if addr != "" {
		parts := strings.Split(addr, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				addrs = append(addrs, ParseListenAddr(part))
			}
		}
	}

	// Default if none specified
	if len(addrs) == 0 {
		addrs = []ListenAddr{ParseListenAddr(":19090")}
	}

	return &Server{
		addrs:             addrs,
		registry:          registry,
		metricsStore:      reg.NewMetricsStore(),
		systemInfoCache:   reg.NewSystemInfoCache(),
		defaultSessionTTL: 5 * time.Minute,
		ctx:               ctx,
		cancel:            cancel,
		logger:            slog.Default(),
	}
}

// NewServerWithAddrs creates a new NNG control server with explicit listen addresses
func NewServerWithAddrs(addrs []ListenAddr, registry *reg.Store) *Server {
	if registry == nil {
		registry = reg.NewStore()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Default if none specified
	if len(addrs) == 0 {
		addrs = []ListenAddr{ParseListenAddr(":19090")}
	}

	return &Server{
		addrs:             addrs,
		registry:          registry,
		metricsStore:      reg.NewMetricsStore(),
		systemInfoCache:   reg.NewSystemInfoCache(),
		defaultSessionTTL: 5 * time.Minute,
		ctx:               ctx,
		cancel:            cancel,
		logger:            slog.Default(),
	}
}

// NewServerWithDB creates a new NNG control server with database persistence
func NewServerWithDB(addrs []ListenAddr, registry *reg.Store, loader AgentSessionLoader) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	// Default if none specified
	if len(addrs) == 0 {
		addrs = []ListenAddr{ParseListenAddr(":19090")}
	}

	// Use provided registry or create new one
	if registry == nil {
		registry = reg.NewStore()
	}

	server := &Server{
		addrs:              addrs,
		registry:           registry,
		agentSessionLoader: loader,
		metricsStore:       reg.NewMetricsStore(),
		systemInfoCache:    reg.NewSystemInfoCache(),
		defaultSessionTTL:  5 * time.Minute,
		ctx:                ctx,
		cancel:             cancel,
		logger:             slog.Default(),
	}

	return server
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

	// Create a socket for each listen address
	sockets := make([]mangos.Socket, 0, len(s.addrs))

	for _, la := range s.addrs {
		// Create REP socket
		sock, err := rep.NewSocket()
		if err != nil {
			// Close any already created sockets
			for _, s := range sockets {
				s.Close()
			}
			return fmt.Errorf("failed to create socket for %s: %w", la.URL, err)
		}

		// Configure options
		if err := sock.SetOption(mangos.OptionRecvDeadline, time.Second); err != nil {
			sock.Close()
			for _, s := range sockets {
				s.Close()
			}
			return fmt.Errorf("failed to set recv deadline for %s: %w", la.URL, err)
		}

		// Listen on the address
		if err := sock.Listen(la.URL); err != nil {
			sock.Close()
			for _, s := range sockets {
				s.Close()
			}
			return fmt.Errorf("failed to listen on %s: %w", la.URL, err)
		}

		sockets = append(sockets, sock)
		s.logger.Info("NNG Control server listening", "addr", la.URL, "transport", la.Transport)
	}

	s.sockets = sockets
	s.sock = sockets[0] // Primary socket for serving

	s.startBackgroundTasks()

	// Start serving
	go s.serve()

	return nil
}

func (s *Server) startBackgroundTasks() {
	s.backgroundOnce.Do(func() {
		if err := s.LoadAgentSessions(); err != nil {
			s.logger.Error("failed to load agent sessions from database", "error", err)
		}
		go s.pruneOldMetrics()
		if s.agentSessionLoader != nil {
			go s.cleanupLoop()
		}
	})
}

// StartBackgroundTasks starts background maintenance loops for non-NNG transports.
func (s *Server) StartBackgroundTasks() {
	s.startBackgroundTasks()
}

// TransportHandler exposes the control-plane request handler for alternate transports.
func (s *Server) TransportHandler() transportcore.Handler {
	return transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return s.handleRequest(ctx, msgID, body)
	})
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

	// Close all sockets
	for _, sock := range s.sockets {
		if sock != nil {
			if err := sock.Close(); err != nil {
				s.logger.Error("failed to close socket", "error", err)
			}
		}
	}
	s.sockets = nil
	s.sock = nil

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
	req := &agentv1.RegisterRequest{}
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
	req := &agentv1.HeartbeatRequest{}
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
	req := &agentv1.RegisterCapabilitiesRequest{}
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
func (s *Server) handleRegisterRequest(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	// Forward to upstream if configured
	if s.upstream != nil {
		return s.upstream.HandleRegister(ctx, req)
	}

	// Calculate TTL (default 1 day)
	ttl := 24 * time.Hour
	if req.TtlSeconds > 0 {
		ttl = time.Duration(req.TtlSeconds) * time.Second
	}

	functions, warnings := validateAndNormalizeFunctions(req.GetFunctions())
	warningTexts := make([]string, 0, len(warnings))
	for _, warnMsg := range warnings {
		warningTexts = append(warningTexts, warnMsg.Message)
		s.logger.Warn("register validation warning", "agent_id", req.AgentId, "warning", warnMsg.Message, "code", warnMsg.Code, "function_id", warnMsg.FunctionID, "version", warnMsg.Version)
		s.registry.UpsertRegistrationWarning(reg.FunctionRegistrationWarning{
			AgentID:    req.AgentId,
			FunctionID: warnMsg.FunctionID,
			Version:    warnMsg.Version,
			Code:       warnMsg.Code,
			Message:    warnMsg.Message,
		})
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
	for _, f := range functions {
		if f == nil || f.Id == "" {
			continue
		}
		sess.Functions[f.Id] = reg.FunctionMeta{
			Enabled: f.Enabled,
			Version: f.Version,
		}
		displayName := ""
		if v := f.GetDisplayName(); v != nil {
			displayName = v.GetEn()
		}
		description := ""
		if v := f.GetSummary(); v != nil {
			description = v.GetEn()
		}
		if op, err := converter.ToOpenAPIOperation(converter.LocalFunctionDescriptorDesc{
			ID:           f.Id,
			Version:      f.Version,
			Tags:         f.Tags,
			Summary:      displayName,
			Description:  description,
			OperationID:  f.Id,
			Deprecated:   false,
			InputSchema:  f.GetInputSchema(),
			OutputSchema: f.GetOutputSchema(),
			Category:     f.GetCategory(),
			Risk:         f.GetRisk(),
			Entity:       f.GetEntity(),
			Operation:    f.GetOperation(),
		}); err == nil {
			if err := s.registry.UpsertOpenAPI(f.Id, op); err != nil {
				s.logger.Warn("failed to upsert openapi operation from register request", "function_id", f.Id, "error", err)
			}
		} else {
			s.logger.Warn("failed to convert function descriptor to openapi operation", "function_id", f.Id, "error", err)
		}
	}

	// Populate providers from request
	if len(req.Processes) > 0 {
		providers := make([]reg.ProviderSession, 0, len(req.Processes))
		for _, p := range req.Processes {
			if p == nil || p.ServiceId == "" {
				continue
			}
			providers = append(providers, reg.ProviderSession{
				ProviderID:   p.ServiceId,
				GameID:       req.GameId,
				Env:          req.Env,
				Addr:         p.Addr,
				Version:      p.Version,
				LastSeenUnix: p.LastSeenUnix,
				FunctionIDs:  p.FunctionIds,
			})
		}
		sess.Providers = providers
	}

	// Dual-write to database if configured
	if s.agentSessionLoader != nil {
		if err := s.agentSessionLoader.Upsert(ctx, sess); err != nil {
			s.logger.Error("failed to write agent session to database", "agent_id", req.AgentId, "error", err)
			// Don't block registration if database write fails
		}
	}

	s.registry.UpsertAgent(sess)

	s.logger.Info("Agent registered via NNG", "agent_id", req.AgentId, "game_id", req.GameId, "functions", len(functions), "warnings", len(warnings))

	return &agentv1.RegisterResponse{
		Warnings: warningTexts,
	}, nil
}

func validateAndNormalizeFunctions(items []*agentv1.FunctionDescriptor) ([]*agentv1.FunctionDescriptor, []registerWarning) {
	if len(items) == 0 {
		return nil, nil
	}
	byID := make(map[string]*agentv1.FunctionDescriptor, len(items))
	warnings := make([]registerWarning, 0)
	for idx, f := range items {
		if f == nil {
			warnings = append(warnings, registerWarning{Code: "nil_function", Message: fmt.Sprintf("functions[%d] is nil and skipped", idx)})
			continue
		}
		fid := strings.TrimSpace(strings.ToLower(f.GetId()))
		if fid == "" {
			warnings = append(warnings, registerWarning{Code: "empty_function_id", Message: fmt.Sprintf("functions[%d] has empty function_id and skipped", idx)})
			continue
		}
		if !functionIDPattern.MatchString(fid) {
			warnings = append(warnings, registerWarning{Code: "invalid_function_id", FunctionID: fid, Version: f.GetVersion(), Message: fmt.Sprintf("function_id=%s invalid format (expected lowercase dotted id) and skipped", fid)})
			continue
		}
		version := strings.TrimSpace(f.GetVersion())
		if !isValidSemver(version) {
			warnings = append(warnings, registerWarning{Code: "invalid_version", FunctionID: fid, Version: version, Message: fmt.Sprintf("function_id=%s version=%s invalid semver and skipped", fid, version)})
			continue
		}
		f.Id = fid

		if prev, ok := byID[fid]; ok {
			compare := compareSemver(f.GetVersion(), prev.GetVersion())
			if compare > 0 {
				warnings = append(warnings, registerWarning{Code: "duplicate_function_id", FunctionID: fid, Version: f.GetVersion(), Message: fmt.Sprintf("duplicate function_id=%s detected; keep higher version %s over %s", fid, f.GetVersion(), prev.GetVersion())})
				byID[fid] = f
			} else {
				warnings = append(warnings, registerWarning{Code: "duplicate_function_id", FunctionID: fid, Version: prev.GetVersion(), Message: fmt.Sprintf("duplicate function_id=%s detected; keep higher version %s over %s", fid, prev.GetVersion(), f.GetVersion())})
			}
			continue
		}
		byID[fid] = f
	}

	out := make([]*agentv1.FunctionDescriptor, 0, len(byID))
	for _, f := range byID {
		out = append(out, f)
	}
	return out, warnings
}

func isValidSemver(v string) bool {
	return semverPattern.MatchString(strings.TrimSpace(v))
}

func compareSemver(a, b string) int {
	parse := func(raw string) [3]int {
		s := strings.TrimPrefix(strings.TrimSpace(raw), "v")
		parts := strings.SplitN(s, "-", 2)
		num := parts[0]
		p := strings.Split(num, ".")
		out := [3]int{0, 0, 0}
		for i := 0; i < len(p) && i < 3; i++ {
			n, err := strconv.Atoi(p[i])
			if err != nil {
				return out
			}
			out[i] = n
		}
		return out
	}
	va := parse(a)
	vb := parse(b)
	for i := 0; i < 3; i++ {
		if va[i] > vb[i] {
			return 1
		}
		if va[i] < vb[i] {
			return -1
		}
	}
	return 0
}

// handleHeartbeatRequest implements the actual Heartbeat logic
func (s *Server) handleHeartbeatRequest(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	// Forward to upstream if configured
	if s.upstream != nil {
		return s.upstream.HandleHeartbeat(ctx, req)
	}

	if req.AgentId == "" {
		return &agentv1.HeartbeatResponse{}, nil
	}

	s.registry.Mu().Lock()
	agent := s.registry.AgentsUnsafe()[req.AgentId]
	if agent != nil {
		agent.ExpireAt = time.Now().Add(s.defaultSessionTTL)
		agent.LastSeen = time.Now()

		// Async dual-write to database if configured
		if s.agentSessionLoader != nil {
			// Capture the agent reference for the goroutine
			agentToUpdate := agent
			go func() {
				if err := s.agentSessionLoader.Upsert(context.Background(), agentToUpdate); err != nil {
					s.logger.Error("failed to update agent session in database", "agent_id", req.AgentId, "error", err)
				}
			}()
		}
	}
	s.registry.Mu().Unlock()

	return &agentv1.HeartbeatResponse{}, nil
}

// handleRegisterCapabilitiesRequest implements the actual RegisterCapabilities logic
func (s *Server) handleRegisterCapabilitiesRequest(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
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
	providerCaps := registry.OpenAPIProviderCaps{
		ID:         req.Provider.Id,
		Version:    req.Provider.Version,
		Lang:       req.Provider.Lang,
		SDK:        req.Provider.Sdk,
		OpenAPIDoc: manifestData,
		UpdatedAt:  time.Now(),
	}

	s.registry.UpsertOpenAPIProvider(providerCaps)

	s.logger.Info("Provider capabilities registered via NNG", "provider_id", req.Provider.Id)

	return &agentv1.RegisterCapabilitiesResponse{}, nil
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
	resp := &agentv1.RegisterResponse{}
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
	defer s.registry.Mu().RUnlock()

	agents := s.registry.AgentsUnsafe()

	// Build list of listen URLs
	urls := make([]string, 0, len(s.addrs))
	for _, la := range s.addrs {
		urls = append(urls, la.URL)
	}

	return map[string]interface{}{
		"agent_count": len(agents),
		"addresses":   urls,
		"running":     s.running,
		"session_ttl": s.defaultSessionTTL.String(),
	}
}

// GetLocalAddr returns the primary configured listening address
func (s *Server) GetLocalAddr() (string, error) {
	if len(s.addrs) == 0 {
		return "", fmt.Errorf("address not configured")
	}
	return s.addrs[0].URL, nil
}

// GetAddrs returns all configured listen addresses
func (s *Server) GetAddrs() []ListenAddr {
	return s.addrs
}

// GetLocalAddrs returns all configured listen URLs
func (s *Server) GetLocalAddrs() []string {
	urls := make([]string, 0, len(s.addrs))
	for _, la := range s.addrs {
		urls = append(urls, la.URL)
	}
	return urls
}

// ControlHandler wraps Server to implement Handler interface
type ControlHandler struct {
	server *Server
}

// NewControlHandler creates a new control handler
func NewControlHandler(server *Server) *ControlHandler {
	return &ControlHandler{server: server}
}

func (h *ControlHandler) HandleRegister(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	return h.server.handleRegisterRequest(ctx, req)
}

func (h *ControlHandler) HandleHeartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	return h.server.handleHeartbeatRequest(ctx, req)
}

func (h *ControlHandler) HandleRegisterCapabilities(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
	return h.server.handleRegisterCapabilitiesRequest(ctx, req)
}

// LoadAgentSessions loads active agent sessions from the database into memory
func (s *Server) LoadAgentSessions() error {
	if s.agentSessionLoader == nil {
		return nil // No database configured
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Load sessions into registry
	if err := s.registry.LoadFromDB(ctx, s.agentSessionLoader); err != nil {
		return fmt.Errorf("failed to load agent sessions: %w", err)
	}

	return nil
}

// cleanupLoop periodically deletes expired sessions from the database
func (s *Server) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			deleted, err := s.agentSessionLoader.DeleteExpired(ctx)
			cancel()

			if err != nil {
				s.logger.Error("failed to delete expired sessions", "error", err)
			} else if deleted > 0 {
				s.logger.Info("deleted expired sessions from database", "count", deleted)
			}
		}
	}
}
