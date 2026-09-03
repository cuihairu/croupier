// Package agent provides business logic for handling local agent requests
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/function/registrationguard"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	"github.com/cuihairu/croupier/internal/telemetry"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ProviderManager is the interface for provider function calls
type ProviderManager interface {
	IsPlatformFunction(functionID string) bool
	Call(ctx context.Context, functionID string, request []byte) ([]byte, error)
}

// OpsServerWrapper wraps the OpsServer functionality
type OpsServerWrapper interface {
	GetSystemInfo(ctx context.Context, req *emptypb.Empty) (*opsv1.SystemInfo, error)
	ListProcesses(ctx context.Context, req *emptypb.Empty) (*opsv1.ListProcessesResponse, error)
	ReportMetrics(ctx context.Context, req *opsv1.MetricsReport) (*emptypb.Empty, error)
	RestartProcess(ctx context.Context, req *opsv1.RestartProcessRequest) (*opsv1.RestartProcessResponse, error)
	StopProcess(ctx context.Context, req *opsv1.StopProcessRequest) (*opsv1.StopProcessResponse, error)
	StartProcess(ctx context.Context, req *opsv1.StartProcessRequest) (*opsv1.StartProcessResponse, error)
	ExecuteCommand(ctx context.Context, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error)

	// System services (platform-specific, read-only)
	// These use JSON request/response to avoid circular dependencies
	ListServicesJSON(ctx context.Context, jsonReq []byte) ([]byte, error)
	GetServiceStatusJSON(ctx context.Context, jsonReq []byte) ([]byte, error)
	ListCronJobsJSON(ctx context.Context) ([]byte, error)
}

// LocalHandler contains the business logic for handling agent requests
// without any transport-specific dependencies.
type LocalHandler struct {
	store            *agentlocal.LocalStore
	tasks            *TaskRunner
	pm               ProviderManager // Use field name `pm` to avoid conflict with existing providerManager in app.go
	opsServer        OpsServerWrapper
	reporter         TaskEventReporter
	providerSessions *ProviderSessionStore
	tlsCfg           *tlsutil.ClientTLSConfig
	logger           *slog.Logger
	configDir        string
	agentID          string
	expectedGameID   string // Agent 配置的 gameId，用于校验 SDK 注册
	expectedEnv      string // Agent 配置的 env，用于校验 SDK 注册
	// providerCallTimeout 是 Agent → Provider 同步调用的默认预算；
	// 请求 metadata["timeoutMs"] 声明更小值时取更小者（Go deadline
	// min 语义）。此前硬编码 10s 与 Server 派发层 15s 倒挂。
	providerCallTimeout time.Duration
	mu                  sync.RWMutex
}

// SetProviderSessionStore enables callback over the Provider's established
// TCP session instead of dialing an address supplied by the Provider.
func (h *LocalHandler) SetProviderSessionStore(store *ProviderSessionStore) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.providerSessions = store
}

// NewLocalHandler creates a new LocalHandler instance
func NewLocalHandler(store *agentlocal.LocalStore, configDir, agentID string, logger *slog.Logger) *LocalHandler {
	if logger == nil {
		logger = slog.Default()
	}
	h := &LocalHandler{
		store:     store,
		logger:    logger,
		configDir: configDir,
		agentID:   agentID,
		// 默认对齐 Server 派发层 invokeTimeout（15s），消除旧 10s<15s 倒挂。
		providerCallTimeout: 15 * time.Second,
	}
	// TaskRunner executes tasks via the handler's invoke path.
	h.tasks = NewTaskRunner(h.executeTask, nil, logger)
	return h
}

// SetProviderCallTimeout 配置 Agent → Provider 同步调用默认预算。
// 非正值回落默认 15s；上限 60s（同步通道边界，更长操作应走异步任务）。
func (h *LocalHandler) SetProviderCallTimeout(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if d <= 0 {
		d = 15 * time.Second
	}
	if d > 60*time.Second {
		d = 60 * time.Second
	}
	h.providerCallTimeout = d
}

// providerCallDeadline 计算本次 Provider 调用的超时：请求 metadata 声明的
// timeout_ms 是权威预算（clamp [1s, 60s] 硬顶），无声明时用配置默认。
// 垃圾值 → 配置默认。
func (h *LocalHandler) providerCallDeadline(meta map[string]string) time.Duration {
	h.mu.RLock()
	def := h.providerCallTimeout
	h.mu.RUnlock()
	if def <= 0 {
		def = 15 * time.Second
	}
	raw := strings.TrimSpace(meta["timeoutMs"])
	if raw == "" {
		return def
	}
	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || ms <= 0 {
		return def
	}
	budget := time.Duration(ms) * time.Millisecond
	if budget < time.Second {
		budget = time.Second
	}
	if budget > 60*time.Second {
		budget = 60 * time.Second
	}
	return budget
}

// SetProviderManager sets the provider manager
func (h *LocalHandler) SetProviderManager(pm ProviderManager) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pm = pm
}

// SetOpsServer sets the ops server wrapper
func (h *LocalHandler) SetOpsServer(ops OpsServerWrapper) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.opsServer = ops
}

func (h *LocalHandler) SetTaskEventReporter(reporter TaskEventReporter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reporter = reporter
	if h.tasks != nil {
		h.tasks.SetReporter(reporter)
	}
}

// SetTLSConfig sets the TLS config for outbound connections
func (h *LocalHandler) SetTLSConfig(cfg *tlsutil.ClientTLSConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tlsCfg = cfg
}

// SetExpectedGameEnv sets the agent's expected gameId/env, used to validate
// SDK provider registrations. When set (non-empty), a provider registering with
// a mismatched game_id/env triggers a warning.
func (h *LocalHandler) SetExpectedGameEnv(gameID, env string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.expectedGameID = gameID
	h.expectedEnv = env
}

// Handle implements transportcore.Handler interface
// It dispatches requests to the appropriate handler based on message type
func (h *LocalHandler) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	return h.handleRequest(ctx, msgID, body)
}

// handleRequest dispatches request to appropriate handler
func (h *LocalHandler) handleRequest(ctx context.Context, msgID uint32, data []byte) ([]byte, error) {
	switch msgID {
	// InvokerService
	case protocol.MsgInvokeRequest:
		return h.handleInvoke(ctx, data)
	case protocol.MsgStartTaskRequest:
		return h.handleStartTask(ctx, data)
	case protocol.MsgCancelTaskRequest:
		return h.handleCancelTask(ctx, data)

	// OpsService
	case protocol.MsgGetSystemInfoRequest:
		return h.handleGetSystemInfo(ctx, data)
	case protocol.MsgListProcessesRequest:
		return h.handleListProcesses(ctx, data)
	case protocol.MsgReportMetricsRequest:
		return h.handleReportMetrics(ctx, data)
	case protocol.MsgRestartProcessRequest:
		return h.handleRestartProcess(ctx, data)
	case protocol.MsgStopProcessRequest:
		return h.handleStopProcess(ctx, data)
	case protocol.MsgStartProcessRequest:
		return h.handleStartProcess(ctx, data)
	case protocol.MsgExecuteCommandRequest:
		return h.handleExecuteCommand(ctx, data)
	case protocol.MsgListServicesRequest:
		return h.handleListServices(ctx, data)
	case protocol.MsgGetServiceStatusRequest:
		return h.handleGetServiceStatus(ctx, data)
	case protocol.MsgListCronJobsRequest:
		return h.handleListCronJobs(ctx, data)
	case protocol.MsgRegisterCapabilitiesReq:
		return h.handleRegisterCapabilities(ctx, data)

	// Provider session messages (for SDK Provider connections)
	case protocol.MsgProviderConnectRequest:
		return h.handleProviderConnect(ctx, data)
	case protocol.MsgProviderHeartbeatRequest:
		return h.handleProviderHeartbeat(ctx, data)
	case protocol.MsgProviderDrainRequest:
		return h.handleProviderDrain(ctx, data)

	default:
		return nil, fmt.Errorf("unknown message type: 0x%06X", msgID)
	}
}

// handleInvoke handles InvokeRequest
func (h *LocalHandler) handleInvoke(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.InvokeRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal InvokeRequest: %w", err)
	}

	functionID := req.GetFunctionId()
	ctx = telemetry.ExtractContext(ctx, req.GetMetadata())
	ctx, span := agentTracer().Start(ctx, "agent.invoke",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("function.id", functionID),
			attribute.String("agent.id", h.agentID),
			attribute.String("service.id", req.GetMetadata()["serviceId"]),
			attribute.String("task.id", req.GetMetadata()["taskId"]),
		),
	)
	defer span.End()

	// Check if this is a provider function call
	h.mu.RLock()
	pm := h.pm
	h.mu.RUnlock()

	if pm != nil && pm.IsPlatformFunction(functionID) {
		resp, err := h.invokePlatform(ctx, functionID, req)
		recordSpanResult(span, err)
		return resp, err
	}

	// Regular function forwarding - need to dial game server
	addr, err := h.pickInstance(functionID, req.GetMetadata())
	if err != nil {
		recordSpanResult(span, err)
		return nil, err
	}
	span.SetAttributes(attribute.String("provider.addr", addr))

	callCtx, cancel := context.WithTimeout(ctx, h.providerCallDeadline(req.GetMetadata()))
	defer cancel()

	reqBytes, err := proto.Marshal(req)
	if err != nil {
		err = fmt.Errorf("marshal InvokeRequest: %w", err)
		recordSpanResult(span, err)
		return nil, err
	}

	respBytes, err := h.callProvider(callCtx, functionID, req.GetMetadata(), addr, protocol.MsgInvokeRequest, reqBytes)
	if err != nil {
		err = fmt.Errorf("invoke local provider at %s: %w", addr, err)
		recordSpanResult(span, err)
		return nil, err
	}

	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		err = fmt.Errorf("unmarshal InvokeResponse: %w", err)
		recordSpanResult(span, err)
		return nil, err
	}

	out, err := proto.Marshal(resp)
	recordSpanResult(span, err)
	return out, err
}

func (h *LocalHandler) callProvider(ctx context.Context, functionID string, metadata map[string]string, fallbackAddr string, msgID uint32, data []byte) ([]byte, error) {
	h.mu.RLock()
	sessions := h.providerSessions
	h.mu.RUnlock()
	if sessions != nil {
		serviceID := metadata["serviceId"]
		if serviceID != "" {
			if session, ok := sessions.GetByServiceID(serviceID); ok && session.Conn() != nil {
				_, response, err := session.Conn().Call(ctx, msgID, data)
				return response, err
			}
		}
		for _, session := range sessions.List() {
			for _, id := range session.FunctionIDs() {
				if id == functionID && session.Conn() != nil {
					_, response, err := session.Conn().Call(ctx, msgID, data)
					return response, err
				}
			}
		}
	}
	return h.callLocalProvider(ctx, fallbackAddr, msgID, data)
}

// callLocalProvider calls a local provider using TCP transport only
func (h *LocalHandler) callLocalProvider(ctx context.Context, addr string, msgID uint32, data []byte) ([]byte, error) {
	// Only TCP transport is supported for LocalHandler
	client, err := tcptr.NewClient(&tcptr.Config{
		Address:        addr,
		Insecure:       true,
		ConnectTimeout: 5 * time.Second,
		RecvTimeout:    30 * time.Second,
		SendTimeout:    30 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	defer client.Close()
	_, respBody, err := client.Call(ctx, msgID, data)
	return respBody, err
}

// invokePlatform handles provider function calls
func (h *LocalHandler) invokePlatform(ctx context.Context, functionID string, req *sdkv1.InvokeRequest) ([]byte, error) {
	request := req.GetPayload()

	h.mu.RLock()
	pm := h.pm
	h.mu.RUnlock()

	if pm == nil {
		return nil, fmt.Errorf("provider manager not configured")
	}

	response, err := pm.Call(ctx, functionID, request)
	if err != nil {
		return nil, fmt.Errorf("provider call failed: %w", err)
	}

	resp := &sdkv1.InvokeResponse{
		Payload: response,
	}
	return proto.Marshal(resp)
}

// pickInstance selects a game server instance for the function.
// Uses double-index routing: function_id -> service_id -> instances (Nacos style)
func (h *LocalHandler) pickInstance(functionID string, metadata map[string]string) (string, error) {
	if h.store == nil {
		return "", fmt.Errorf("instance store not initialized")
	}
	if functionID == "" {
		return "", fmt.Errorf("function ID is required")
	}

	// 使用双层索引获取实例
	snap := h.store.ListByService()
	serviceMap := snap[functionID]
	if len(serviceMap) == 0 {
		return "", fmt.Errorf("function '%s' is not registered", functionID)
	}

	// 按 service_id 路由（一级过滤）
	targetService := metadata["serviceId"]
	var arr []agentlocal.Instance
	if targetService != "" {
		arr = serviceMap[targetService]
		if len(arr) == 0 {
			return "", fmt.Errorf("no instance for service '%s'", targetService)
		}
	} else {
		// 无 service_id，合并所有实例
		for _, instances := range serviceMap {
			arr = append(arr, instances...)
		}
	}

	// 优先选择健康实例（30秒内有心跳）
	now := time.Now()
	var healthy []agentlocal.Instance
	for _, inst := range arr {
		if now.Sub(inst.LastSeen) < 30*time.Second {
			healthy = append(healthy, inst)
		}
	}

	if len(healthy) > 0 {
		// 使用 fnvIndex 做负载均衡（round-robin风格）
		idx := fnvIndex(functionID, len(healthy))
		return healthy[idx].Addr, nil
	}

	// 降级：返回第一个实例
	h.logger.Warn("function instances are stale, routing to first instance anyway",
		"function_id", functionID, "service_id", targetService, "addr", arr[0].Addr)
	return arr[0].Addr, nil
}

// handleStartTask handles StartTaskRequest.
func (h *LocalHandler) handleStartTask(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.InvokeRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal InvokeRequest for StartTask: %w", err)
	}
	req.Metadata = telemetry.InjectContext(telemetry.ExtractContext(ctx, req.GetMetadata()), req.Metadata)

	taskID := h.tasks.Start(req)

	resp := &sdkv1.StartTaskResponse{
		TaskId: taskID,
	}
	return proto.Marshal(resp)
}

// handleCancelTask handles CancelTaskRequest.
func (h *LocalHandler) handleCancelTask(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.CancelTaskRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal CancelTaskRequest: %w", err)
	}

	if h.tasks == nil {
		return nil, fmt.Errorf("task tracking not available")
	}

	h.tasks.Cancel(req.GetTaskId())

	resp := &sdkv1.StartTaskResponse{}
	return proto.Marshal(resp)
}

// executeTask is the TaskExecutor seam: it routes a task invocation through
// the same provider-call path as a synchronous invoke. Used by TaskRunner.
func (h *LocalHandler) executeTask(ctx context.Context, req *sdkv1.InvokeRequest) ([]byte, error) {
	ctx = telemetry.ExtractContext(ctx, req.GetMetadata())
	ctx, span := agentTracer().Start(ctx, "agent.task.execute",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("function.id", req.GetFunctionId()),
			attribute.String("agent.id", h.agentID),
			attribute.String("task.id", req.GetMetadata()["taskId"]),
		),
	)
	defer span.End()

	respBytes, err := h.handleInvoke(ctx, mustMarshal(req))
	if err != nil {
		recordSpanResult(span, err)
		return nil, err
	}
	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		err = fmt.Errorf("unmarshal task invoke response: %w", err)
		recordSpanResult(span, err)
		return nil, err
	}
	if len(resp.GetPayload()) == 0 {
		recordSpanResult(span, nil)
		return []byte("null"), nil
	}
	recordSpanResult(span, nil)
	return resp.GetPayload(), nil
}

func agentTracer() trace.Tracer {
	return otel.Tracer("croupier.agent")
}

func recordSpanResult(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}
	span.SetStatus(codes.Ok, "")
}

func mustMarshal(msg proto.Message) []byte {
	if msg == nil {
		return nil
	}
	data, err := proto.Marshal(msg)
	if err == nil {
		return data
	}
	fallback, _ := json.Marshal(map[string]string{"error": err.Error()})
	return fallback
}

// handleGetSystemInfo handles GetSystemInfoRequest
func (h *LocalHandler) handleGetSystemInfo(ctx context.Context, data []byte) ([]byte, error) {
	req := &emptypb.Empty{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal Empty: %w", err)
	}

	h.mu.RLock()
	ops := h.opsServer
	h.mu.RUnlock()

	if ops == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := ops.GetSystemInfo(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get system info: %w", err)
	}

	return proto.Marshal(resp)
}

// handleListProcesses handles ListProcessesRequest
func (h *LocalHandler) handleListProcesses(ctx context.Context, data []byte) ([]byte, error) {
	req := &emptypb.Empty{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal Empty: %w", err)
	}

	h.mu.RLock()
	ops := h.opsServer
	h.mu.RUnlock()

	if ops == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := ops.ListProcesses(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}

	return proto.Marshal(resp)
}

// handleReportMetrics handles ReportMetricsRequest
func (h *LocalHandler) handleReportMetrics(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.MetricsReport{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal MetricsReport: %w", err)
	}

	h.mu.RLock()
	ops := h.opsServer
	h.mu.RUnlock()

	if ops == nil {
		// Just ack, don't error
		resp := &emptypb.Empty{}
		return proto.Marshal(resp)
	}

	resp, err := ops.ReportMetrics(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("report metrics: %w", err)
	}

	return proto.Marshal(resp)
}

// handleRestartProcess handles RestartProcessRequest
func (h *LocalHandler) handleRestartProcess(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.RestartProcessRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal RestartProcessRequest: %w", err)
	}

	h.mu.RLock()
	ops := h.opsServer
	h.mu.RUnlock()

	if ops == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := ops.RestartProcess(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("restart process: %w", err)
	}

	return proto.Marshal(resp)
}

// handleStopProcess handles StopProcessRequest
func (h *LocalHandler) handleStopProcess(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.StopProcessRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal StopProcessRequest: %w", err)
	}

	h.mu.RLock()
	ops := h.opsServer
	h.mu.RUnlock()

	if ops == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := ops.StopProcess(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("stop process: %w", err)
	}

	return proto.Marshal(resp)
}

// handleStartProcess handles StartProcessRequest
func (h *LocalHandler) handleStartProcess(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.StartProcessRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal StartProcessRequest: %w", err)
	}

	h.mu.RLock()
	ops := h.opsServer
	h.mu.RUnlock()

	if ops == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := ops.StartProcess(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	return proto.Marshal(resp)
}

// handleExecuteCommand handles ExecuteCommandRequest
func (h *LocalHandler) handleExecuteCommand(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.ExecuteCommandRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal ExecuteCommandRequest: %w", err)
	}

	h.mu.RLock()
	ops := h.opsServer
	h.mu.RUnlock()

	if ops == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := ops.ExecuteCommand(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("execute command: %w", err)
	}

	return proto.Marshal(resp)
}

// handleListServices handles ListServicesRequest
func (h *LocalHandler) handleListServices(ctx context.Context, data []byte) ([]byte, error) {
	h.mu.RLock()
	ops := h.opsServer
	h.mu.RUnlock()

	if ops == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	// Use JSON to avoid circular dependency
	return ops.ListServicesJSON(ctx, data)
}

// handleListCronJobs handles ListCronJobsRequest（Agent 所在主机的定时任务）。
func (h *LocalHandler) handleListCronJobs(ctx context.Context, _ []byte) ([]byte, error) {
	h.mu.RLock()
	ops := h.opsServer
	h.mu.RUnlock()

	if ops == nil {
		return nil, fmt.Errorf("ops server not configured")
	}
	return ops.ListCronJobsJSON(ctx)
}

// handleGetServiceStatus handles GetServiceStatusRequest
func (h *LocalHandler) handleGetServiceStatus(ctx context.Context, data []byte) ([]byte, error) {
	h.mu.RLock()
	ops := h.opsServer
	h.mu.RUnlock()

	if ops == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	// Use JSON to avoid circular dependency
	return ops.GetServiceStatusJSON(ctx, data)
}

// handleRegisterCapabilities accepts provider capability registration from local SDK clients.
// Capabilities are currently handled by the upstream control service path; local agent treats this as no-op ack.
func (h *LocalHandler) handleRegisterCapabilities(ctx context.Context, data []byte) ([]byte, error) {
	req := &agentv1.RegisterCapabilitiesRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal RegisterCapabilitiesRequest: %w", err)
	}
	resp := &agentv1.RegisterCapabilitiesResponse{}
	return proto.Marshal(resp)
}

// hostFromAddr extracts hostname from address
func hostFromAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(strings.TrimPrefix(addr, "["), "]")
}

// fnvIndex computes hash-based index
func fnvIndex(key string, mod int) int {
	if mod <= 1 {
		return 0
	}
	var h uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return int(h % uint32(mod))
}

// handleProviderConnect handles ProviderConnectRequest from SDK Providers.
// This allows SDK Providers to register their functions with the Agent.
func (h *LocalHandler) handleProviderConnect(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.ProviderConnectRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal ProviderConnectRequest: %w", err)
	}

	if req.ServiceId == "" {
		return nil, fmt.Errorf("service_id is required")
	}

	// 校验 SDK 的 game_id/env 是否与 Agent 配置一致（作用域规范 §14：
	// provider 必须上报 scope 且与 agent 一致——无空值兼容，空值即
	// mismatch；agent 侧 scope 由启动校验保证非空）。
	h.mu.RLock()
	expectedGameID := h.expectedGameID
	expectedEnv := h.expectedEnv
	h.mu.RUnlock()
	var warnings []string
	if req.GameId != expectedGameID {
		msg := fmt.Sprintf("game_id mismatch: provider=%q agent=%q", req.GameId, expectedGameID)
		warnings = append(warnings, msg)
		h.logger.Warn("provider game_id mismatch",
			"service_id", req.ServiceId,
			"provider_game_id", req.GameId,
			"agent_game_id", expectedGameID,
		)
	}
	if req.Env != expectedEnv {
		msg := fmt.Sprintf("env mismatch: provider=%q agent=%q", req.Env, expectedEnv)
		warnings = append(warnings, msg)
		h.logger.Warn("provider env mismatch",
			"service_id", req.ServiceId,
			"provider_env", req.Env,
			"agent_env", expectedEnv,
		)
	}

	sessionID := fmt.Sprintf("ps-%d", time.Now().UnixNano())

	// Use sessionID as providerID for tracking
	// Register all functions from the provider in a single call
	if h.store != nil && len(req.Functions) > 0 {
		// Convert proto functions to ProviderFunctionDescriptor
		funcs := make([]*sdkv1.ProviderFunctionDescriptor, 0, len(req.Functions))
		for _, fn := range req.Functions {
			if violation, ok := providerDescriptorPresentationViolation(fn); ok {
				warning := fmt.Sprintf("function_id=%q registers forbidden presentation field %q at %s and is skipped", fn.GetId(), violation.Field, violation.Location)
				warnings = append(warnings, warning)
				h.logger.Warn("provider function registration rejected", "service_id", req.ServiceId, "function_id", fn.GetId(), "field", violation.Field, "location", violation.Location)
				continue
			}
			funcs = append(funcs, &sdkv1.ProviderFunctionDescriptor{
				Id:                fn.GetId(),
				Version:           fn.GetVersion(),
				Tags:              fn.GetTags(),
				Summary:           fn.GetSummary(),
				Description:       fn.GetDescription(),
				OperationId:       fn.GetOperationId(),
				Deprecated:        fn.GetDeprecated(),
				InputSchema:       fn.GetInputSchema(),
				OutputSchema:      fn.GetOutputSchema(),
				Resource:          fn.GetResource(),
				Operation:         fn.GetOperation(),
				Capability:        fn.GetCapability(),
				Execution:         fn.GetExecution(),
				ApprovalRequired:  fn.GetApprovalRequired(),
				ApprovalPolicyKey: fn.GetApprovalPolicyKey(),
				Risk:              fn.GetRisk(),
				Enabled:           fn.GetEnabled(),
				Permission:        fn.GetPermission(),
			})
		}
		// 提取元数据（参考 Nacos metadata）
		metadata := map[string]string{
			"sdkLanguage":      req.SdkLanguage,
			"sdkVersion":       req.SdkVersion,
			"sdkName":          req.SdkName,
			"protocol_version": req.ProtocolVersion,
			"gameId":           req.GameId,
			"env":              req.Env,
		}
		// Use empty addr here; the TCP onConnect path sets the real address.
		h.store.Register(sessionID, req.ServiceId, "", req.Version, funcs, metadata)
	}

	h.logger.Info("Provider connected via TCP session",
		"service_id", req.ServiceId,
		"version", req.Version,
		"session_id", sessionID,
		"sdk_language", req.SdkLanguage,
		"sdk_version", req.SdkVersion,
		"sdk_name", req.SdkName,
		"game_id", req.GameId,
		"env", req.Env,
		"functions", len(req.Functions),
		"warnings", len(warnings),
	)

	resp := &sdkv1.ProviderConnectResponse{
		SessionId: sessionID,
		Warnings:  warnings,
	}
	return proto.Marshal(resp)
}

func providerDescriptorPresentationViolation(fn *sdkv1.ProviderFunctionDescriptor) (registrationguard.PresentationViolation, bool) {
	if fn == nil {
		return registrationguard.PresentationViolation{}, false
	}
	return registrationguard.FindPresentationViolation(nil, fn.GetInputSchema(), fn.GetOutputSchema())
}

// handleProviderHeartbeat handles ProviderHeartbeatRequest from SDK Providers.
func (h *LocalHandler) handleProviderHeartbeat(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.ProviderHeartbeatRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal ProviderHeartbeatRequest: %w", err)
	}

	if req.SessionId == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	// Update LastSeen for this provider's instances without touching the
	// registered functions. Must NOT use Register(nil) here — Register has
	// replace semantics, so a nil func list silently clears all of the
	// provider's functions (this was the demo-site "nothing works" root cause).
	if h.store != nil {
		h.store.Heartbeat(req.SessionId)
	}

	resp := &sdkv1.ProviderHeartbeatResponse{}
	return proto.Marshal(resp)
}

// handleProviderDrain handles ProviderDrainRequest from SDK Providers.
func (h *LocalHandler) handleProviderDrain(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.ProviderDrainRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal ProviderDrainRequest: %w", err)
	}

	if req.SessionId == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	// Drain by removing all of this provider's function instances.
	if h.store != nil {
		h.store.RemoveProvider(req.SessionId)
		h.logger.Info("Provider drained",
			"session_id", req.SessionId,
		)
	}

	resp := &sdkv1.ProviderDrainResponse{}
	return proto.Marshal(resp)
}
