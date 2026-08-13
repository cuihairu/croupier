// Package server provides the Server-side control plane for Croupier.
package server

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
	"github.com/cuihairu/croupier/internal/function/registrationguard"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/tasks"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
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

// ListenAddr represents a single listen address with transport type.
type ListenAddr struct {
	Addr      string // Raw address (e.g., ":19090", "ipc://croupier-server")
	Transport string // Transport type: "tcp", "ipc", etc.
	URL       string // Full URL (e.g., "tcp://:19090", "ipc://croupier-server")
}

// ParseListenAddr parses a string address into a ListenAddr.
func ParseListenAddr(addr string) ListenAddr {
	if strings.Contains(addr, "://") {
		parts := strings.SplitN(addr, "://", 2)
		return ListenAddr{
			Addr:      parts[1],
			Transport: parts[0],
			URL:       addr,
		}
	}
	return ListenAddr{
		Addr:      addr,
		Transport: "tcp",
		URL:       "tcp://" + addr,
	}
}

// IsLocalTCP checks if an address is a local TCP address.
func IsLocalTCP(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.Trim(host, "[]")
	switch strings.ToLower(host) {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// AgentSessionLoader defines the interface for loading and managing agent sessions from a database.
type AgentSessionLoader interface {
	LoadActiveSessions(ctx context.Context) ([]*reg.AgentSession, error)
	Upsert(ctx context.Context, sess *reg.AgentSession) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// TaskStore defines the interface for task run persistence used by handleTaskEvent.
type TaskStore interface {
	UpdateRun(ctx context.Context, taskID string, updates map[string]interface{}) error
	AppendEvent(ctx context.Context, taskID string, eventType tasks.EventType, progress int32, message string, payload []byte) error
}

// Handler handles control service requests.
type Handler interface {
	HandleRegister(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error)
	HandleHeartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error)
	HandleRegisterCapabilities(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error)
}

// ControlService implements the control-plane business logic (register, heartbeat, capabilities).
// It is transport-agnostic — TCPListener delegates inbound frames to this service.
type ControlService struct {
	registry           *reg.Store
	agentSessionLoader AgentSessionLoader

	defaultSessionTTL time.Duration
	metricsStore      *reg.MetricsStore
	systemInfoCache   *reg.SystemInfoCache
	taskStore         TaskStore

	upstream Handler

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	logger *slog.Logger

	backgroundOnce sync.Once
}

// NewControlService creates a new ControlService.
func NewControlService(registry *reg.Store, loader AgentSessionLoader) *ControlService {
	if registry == nil {
		registry = reg.NewStore()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &ControlService{
		registry:           registry,
		agentSessionLoader: loader,
		metricsStore:       reg.NewMetricsStore(),
		systemInfoCache:    reg.NewSystemInfoCache(),
		defaultSessionTTL:  5 * time.Minute,
		ctx:                ctx,
		cancel:             cancel,
		logger:             slog.Default(),
	}
}

func (s *ControlService) SetTaskStore(store TaskStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskStore = store
}

// Store returns the registry store.
func (s *ControlService) Store() *reg.Store { return s.registry }

// MetricsStore returns the metrics store.
func (s *ControlService) MetricsStore() *reg.MetricsStore { return s.metricsStore }

// SetMetricsDB sets the database for metrics persistence.
func (s *ControlService) SetMetricsDB(db *gorm.DB) {
	if s.metricsStore != nil {
		s.metricsStore.SetDB(db)
		s.metricsStore.StartCleanupRoutine(s.ctx, 1*time.Hour)
	}
}

// SystemInfoCache returns the system info cache.
func (s *ControlService) SystemInfoCache() *reg.SystemInfoCache { return s.systemInfoCache }

// SetDefaultSessionTTL sets the default session TTL.
func (s *ControlService) SetDefaultSessionTTL(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defaultSessionTTL = ttl
}

// SetUpstreamHandler sets an upstream handler for forwarding requests.
func (s *ControlService) SetUpstreamHandler(h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upstream = h
}

// SetLogger sets the logger.
func (s *ControlService) SetLogger(logger *slog.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger = logger
}

// TransportHandler exposes the control-plane request handler for TCP transport.
func (s *ControlService) TransportHandler() transportcore.Handler {
	return transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return s.handleRequest(ctx, msgID, body)
	})
}

// StartBackgroundTasks starts background maintenance loops (DB loading, metrics pruning, etc.).
func (s *ControlService) StartBackgroundTasks() {
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

// Stop cancels background goroutines.
func (s *ControlService) Stop() {
	s.cancel()
}

// GetStats returns server statistics.
func (s *ControlService) GetStats() map[string]interface{} {
	s.registry.Mu().RLock()
	defer s.registry.Mu().RUnlock()
	agents := s.registry.AgentsUnsafe()
	return map[string]interface{}{
		"agent_count": len(agents),
		"session_ttl": s.defaultSessionTTL.String(),
	}
}

// ---- internal ----

func (s *ControlService) handleRequest(ctx context.Context, msgID uint32, data []byte) ([]byte, error) {
	switch msgID {
	case protocol.MsgRegisterRequest:
		return s.handleRegister(ctx, data)
	case protocol.MsgHeartbeatRequest:
		return s.handleHeartbeat(ctx, data)
	case protocol.MsgRegisterCapabilitiesReq:
		return s.handleRegisterCapabilities(ctx, data)
	case protocol.MsgTaskEvent:
		return s.handleTaskEvent(ctx, data)
	case protocol.MsgMetricEvent:
		return s.handleMetricEvent(ctx, data)
	default:
		return nil, fmt.Errorf("unknown message type: 0x%06X", msgID)
	}
}

func (s *ControlService) handleRegister(ctx context.Context, data []byte) ([]byte, error) {
	req := &agentv1.RegisterRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal RegisterRequest: %w", err)
	}
	resp, err := s.handleRegisterRequest(ctx, req, "")
	if err != nil {
		return nil, err
	}
	return proto.Marshal(resp)
}

func (s *ControlService) handleHeartbeat(ctx context.Context, data []byte) ([]byte, error) {
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

func (s *ControlService) handleRegisterCapabilities(ctx context.Context, data []byte) ([]byte, error) {
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

func (s *ControlService) handleTaskEvent(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.TaskEvent{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal TaskEvent: %w", err)
	}

	s.mu.RLock()
	taskStore := s.taskStore
	s.mu.RUnlock()
	if taskStore == nil {
		return nil, fmt.Errorf("task store not configured")
	}

	taskID := strings.TrimSpace(req.GetTaskId())
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"progress": req.GetProgress(),
		"message":  req.GetMessage(),
	}

	switch strings.ToLower(strings.TrimSpace(req.GetType())) {
	case string(tasks.EventStarted):
		updates["status"] = tasks.StatusRunning
		updates["started_at"] = &now
	case string(tasks.EventProgress), string(tasks.EventLog):
		updates["status"] = tasks.StatusRunning
	case string(tasks.EventCompleted):
		updates["status"] = tasks.StatusSucceeded
		updates["progress"] = int32(100)
		updates["finished_at"] = &now
		updates["result_payload"] = model.EncodeTaskPayload(req.GetPayload())
	case string(tasks.EventFailed):
		updates["status"] = tasks.StatusFailed
		updates["finished_at"] = &now
		updates["error_message"] = req.GetMessage()
	case string(tasks.EventCancelRequested):
		updates["status"] = tasks.StatusCancelRequested
		updates["cancel_requested_at"] = &now
	case string(tasks.EventCancelled):
		updates["status"] = tasks.StatusCancelled
		updates["finished_at"] = &now
	default:
		updates["status"] = tasks.StatusRunning
	}

	if err := taskStore.UpdateRun(ctx, taskID, updates); err != nil {
		return nil, fmt.Errorf("update task run: %w", err)
	}
	if err := taskStore.AppendEvent(ctx, taskID, tasks.EventType(req.GetType()), req.GetProgress(), req.GetMessage(), req.GetPayload()); err != nil {
		return nil, fmt.Errorf("append task event: %w", err)
	}
	return nil, nil
}

// handleMetricEvent accepts a pushed MetricsReport snapshot from an agent.
// The report is stored in the MetricsStore for later retrieval by the ops API.
func (s *ControlService) handleMetricEvent(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.MetricsReport{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal MetricsReport: %w", err)
	}

	agentID := strings.TrimSpace(req.GetAgentId())
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	// Store metrics for later retrieval
	if s.metricsStore != nil {
		s.metricsStore.Add(agentID, req)
	}

	if s.logger != nil {
		s.logger.Debug("received metrics report",
			"agent_id", agentID,
			"cpu_percent", req.GetCpu().GetUsagePercent(),
			"mem_used_bytes", req.GetMemory().GetUsedBytes(),
			"disk_count", len(req.GetDisks()),
			"network_count", len(req.GetNetworks()),
		)
	}
	return nil, nil
}

// handleRegisterRequest implements the actual Register logic.
func (s *ControlService) handleRegisterRequest(ctx context.Context, req *agentv1.RegisterRequest, remoteAddr string) (*agentv1.RegisterResponse, error) {
	if s.upstream != nil {
		return s.upstream.HandleRegister(ctx, req)
	}

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
		Addr:      remoteAddr,
		Version:   req.Version,
		Region:    "",
		Zone:      "",
		Labels:    map[string]string{},
		ExpireAt:  time.Now().Add(ttl),
		LastSeen:  time.Now(),
		Functions: map[string]reg.FunctionMeta{},
	}

	for _, f := range functions {
		if f == nil || f.Id == "" {
			continue
		}
		sess.Functions[f.Id] = reg.FunctionMeta{
			Enabled:           f.Enabled,
			Version:           f.Version,
			Tags:              append([]string(nil), f.Tags...),
			Summary:           f.GetSummary(),
			Description:       f.GetDescription(),
			OperationID:       f.Id,
			InputSchema:       f.GetInputSchema(),
			OutputSchema:      f.GetOutputSchema(),
			Resource:          f.GetResource(),
			Operation:         f.GetOperation(),
			Capability:        f.GetCapability(),
			Execution:         f.GetExecution(),
			ApprovalRequired:  f.GetApprovalRequired(),
			ApprovalPolicyKey: f.GetApprovalPolicyKey(),
			Risk:              f.GetRisk(),
			Permission:        f.GetPermission(),
		}
		if op, err := converter.ToOpenAPIOperation(converter.ProviderFunctionDescriptorDesc{
			ID:           f.Id,
			Version:      f.Version,
			Tags:         f.Tags,
			Summary:      f.GetSummary(),
			Description:  f.GetDescription(),
			OperationID:  f.Id,
			Deprecated:   f.GetDeprecated(),
			InputSchema:  f.GetInputSchema(),
			OutputSchema: f.GetOutputSchema(),
			Resource:     f.GetResource(),
			Operation:    f.GetOperation(),
			Capability:   f.GetCapability(),
			Execution:    f.GetExecution(),
			Risk:         f.GetRisk(),
			Permission:   f.GetPermission(),
		}); err == nil {
			if err := s.registry.UpsertOpenAPI(f.Id, op); err != nil {
				s.logger.Warn("failed to upsert openapi operation from register request", "function_id", f.Id, "error", err)
			}
		} else {
			s.logger.Warn("failed to convert function descriptor to openapi operation", "function_id", f.Id, "error", err)
		}
	}

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
				SDKLanguage:  p.SdkLanguage,
				SDKVersion:   p.SdkVersion,
				SDKName:      p.SdkName,
				LastSeenUnix: p.LastSeenUnix,
				FunctionIDs:  p.FunctionIds,
			})
		}
		sess.Providers = providers
	}

	if err := s.registry.UpsertAgent(sess); err != nil {
		return nil, fmt.Errorf("register agent dashboard contract rebuild failed: %w", err)
	}

	// NewStoreWithDB persists the session only after contract/proposal
	// materialization succeeds. Do not duplicate that write through the loader;
	// the loader remains the persistence path for memory-only registries and is
	// still used by heartbeat/expiry maintenance.
	if s.agentSessionLoader != nil && !s.registry.SessionPersistenceEnabled() {
		if err := s.agentSessionLoader.Upsert(ctx, sess); err != nil {
			s.logger.Error("failed to write agent session to database", "agent_id", req.AgentId, "error", err)
		}
	}

	s.logger.Info("Agent registered", "agent_id", req.AgentId, "game_id", req.GameId, "functions", len(functions), "warnings", len(warnings))

	return &agentv1.RegisterResponse{
		Warnings: warningTexts,
	}, nil
}

func (s *ControlService) handleHeartbeatRequest(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
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
		if s.agentSessionLoader != nil {
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

func (s *ControlService) handleRegisterCapabilitiesRequest(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
	if s.upstream != nil {
		return s.upstream.HandleRegisterCapabilities(ctx, req)
	}

	if req.Provider == nil || req.Provider.Id == "" {
		return nil, fmt.Errorf("provider metadata is required")
	}

	manifestData, err := decompressManifest(req.ManifestJsonGz)
	if err != nil {
		return nil, fmt.Errorf("invalid manifest (gzip): %w", err)
	}

	providerCaps := reg.OpenAPIProviderCaps{
		ID:         req.Provider.Id,
		Version:    req.Provider.Version,
		Lang:       req.Provider.Lang,
		SDK:        req.Provider.Sdk,
		OpenAPIDoc: manifestData,
		UpdatedAt:  time.Now(),
	}
	s.registry.UpsertOpenAPIProvider(providerCaps)

	s.logger.Info("Provider capabilities registered", "provider_id", req.Provider.Id)

	return &agentv1.RegisterCapabilitiesResponse{}, nil
}

func (s *ControlService) pruneOldMetrics() {
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

func (s *ControlService) cleanupLoop() {
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

// LoadAgentSessions loads active agent sessions from the database into memory.
func (s *ControlService) LoadAgentSessions() error {
	if s.agentSessionLoader == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.registry.LoadFromDB(ctx, s.agentSessionLoader); err != nil {
		return fmt.Errorf("failed to load agent sessions: %w", err)
	}
	return nil
}

// ---- validation helpers ----

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
		if forbiddenKey, ok := descriptorPresentationField(f); ok {
			warnings = append(warnings, registerWarning{Code: "function_presentation_field_not_allowed", FunctionID: fid, Version: version, Message: fmt.Sprintf("function_id=%s registers presentation field %q; function registration only accepts executable capability contract and is skipped", fid, forbiddenKey)})
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

func descriptorPresentationField(f *agentv1.FunctionDescriptor) (string, bool) {
	if f == nil {
		return "", false
	}
	// FunctionDescriptor has no presentation fields. Reject unknown protobuf
	// fields at the registration boundary instead of silently carrying an old
	// page/UI extension into FunctionContract.
	if unknown := f.ProtoReflect().GetUnknown(); len(unknown) > 0 {
		return "unknown_proto_fields", true
	}
	// Schemas belong to the executable capability contract, but they must not
	// carry dashboard presentation extensions (x-menu, x-table-columns,
	// formily, ...). Reject the whole descriptor instead of silently
	// persisting a smuggled UI hint.
	if field, _, ok := registrationguard.ScanJSON(f.GetInputSchema()); ok {
		return "input_schema." + field, true
	}
	if field, _, ok := registrationguard.ScanJSON(f.GetOutputSchema()); ok {
		return "output_schema." + field, true
	}
	return "", false
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

func decompressManifest(data []byte) ([]byte, error) {
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

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

// ControlHandler wraps ControlService to implement the Handler interface.
type ControlHandler struct {
	service *ControlService
}

// NewControlHandler creates a new control handler.
func NewControlHandler(service *ControlService) *ControlHandler {
	return &ControlHandler{service: service}
}

func (h *ControlHandler) HandleRegister(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	return h.service.handleRegisterRequest(ctx, req, "")
}

func (h *ControlHandler) HandleHeartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	return h.service.handleHeartbeatRequest(ctx, req)
}

func (h *ControlHandler) HandleRegisterCapabilities(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
	return h.service.handleRegisterCapabilitiesRequest(ctx, req)
}
