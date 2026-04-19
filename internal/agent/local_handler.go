// Package agent provides business logic for handling local agent requests
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
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

type TaskEventReporter interface {
	ReportTaskEvent(ctx context.Context, event *sdkv1.TaskEvent) error
}

// LocalHandler contains the business logic for handling agent requests
// without any transport-specific dependencies.
type LocalHandler struct {
	store     *agentlocal.LocalStore
	tasks     *taskIndex
	pm        ProviderManager // Use field name `pm` to avoid conflict with existing providerManager in app.go
	opsServer OpsServerWrapper
	reporter  TaskEventReporter
	tlsCfg    *tlsutil.ClientTLSConfig
	logger    *slog.Logger
	configDir string
	agentID   string
	mu        sync.RWMutex
}

// NewLocalHandler creates a new LocalHandler instance
func NewLocalHandler(store *agentlocal.LocalStore, configDir, agentID string, logger *slog.Logger) *LocalHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &LocalHandler{
		store:     store,
		tasks:     newTaskIndex(),
		logger:    logger,
		configDir: configDir,
		agentID:   agentID,
	}
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
}

// SetTLSConfig sets the TLS config for outbound connections
func (h *LocalHandler) SetTLSConfig(cfg *tlsutil.ClientTLSConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tlsCfg = cfg
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
	case protocol.MsgRegisterCapabilitiesReq:
		return h.handleRegisterCapabilities(ctx, data)

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

	// Check if this is a provider function call
	h.mu.RLock()
	pm := h.pm
	h.mu.RUnlock()

	if pm != nil && pm.IsPlatformFunction(functionID) {
		return h.invokePlatform(ctx, functionID, req)
	}

	// Regular function forwarding - need to dial game server
	addr, err := h.pickInstance(functionID, req.GetMetadata())
	if err != nil {
		return nil, err
	}

	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal InvokeRequest: %w", err)
	}

	respBytes, err := h.callLocalProvider(callCtx, addr, protocol.MsgInvokeRequest, reqBytes)
	if err != nil {
		return nil, fmt.Errorf("invoke local provider at %s: %w", addr, err)
	}

	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		return nil, fmt.Errorf("unmarshal InvokeResponse: %w", err)
	}

	return proto.Marshal(resp)
}

// callLocalProvider calls a local provider using TCP transport only
func (h *LocalHandler) callLocalProvider(ctx context.Context, addr string, msgID uint32, data []byte) ([]byte, error) {
	// Only TCP transport is supported for LocalHandler
	client, err := tcptr.NewClient(&tcptr.Config{
		Address:        addr,
		Insecure:       true,
		ConnectTimeout: 5 * time.Second,
		RecvTimeout:    10 * time.Second,
		SendTimeout:    10 * time.Second,
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

// pickInstance selects a game server instance for the function
func (h *LocalHandler) pickInstance(functionID string, metadata map[string]string) (string, error) {
	if h.store == nil {
		return "", fmt.Errorf("instance store not initialized")
	}
	if functionID == "" {
		return "", fmt.Errorf("function ID is required")
	}

	snap := h.store.List()
	arr := snap[functionID]
	if len(arr) == 0 {
		return "", fmt.Errorf("function '%s' is not registered", functionID)
	}

	// Check if instances are healthy (not too old)
	now := time.Now()
	for _, inst := range arr {
		if now.Sub(inst.LastSeen) < 30*time.Second {
			return inst.Addr, nil
		}
	}

	// All instances are stale but return the first one
	h.logger.Warn("function instances are stale, routing to first instance anyway", "function_id", functionID, "addr", arr[0].Addr)
	return arr[0].Addr, nil
}

// handleStartTask handles StartTaskRequest.
func (h *LocalHandler) handleStartTask(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.InvokeRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal InvokeRequest for StartTask: %w", err)
	}

	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	resp := &sdkv1.StartTaskResponse{
		TaskId: taskID,
	}

	taskCtx, cancel := context.WithCancel(context.Background())
	h.tasks.Set(taskID, cancel)
	go h.runTask(taskCtx, taskID, cancel, req)

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

	if cancel, ok := h.tasks.Get(req.GetTaskId()); ok {
		cancel()
		h.tasks.Delete(req.GetTaskId())
		_ = h.emitTaskEvent(context.Background(), &sdkv1.TaskEvent{
			TaskId:   req.GetTaskId(),
			Type:     "cancel_requested",
			Message:  "已请求取消任务",
			Progress: 0,
			Payload:  []byte("null"),
		})
		h.logger.Info("task cancel requested", "task_id", req.GetTaskId())
	}

	resp := &sdkv1.StartTaskResponse{}
	return proto.Marshal(resp)
}

func (h *LocalHandler) runTask(ctx context.Context, taskID string, cancel context.CancelFunc, req *sdkv1.InvokeRequest) {
	defer func() {
		h.tasks.Delete(taskID)
		cancel() // Ensure the context is canceled to prevent leaks
	}()

	_ = h.emitTaskEvent(context.Background(), &sdkv1.TaskEvent{
		TaskId:   taskID,
		Type:     "started",
		Message:  "任务开始执行",
		Progress: 0,
		Payload:  []byte("null"),
	})

	result, err := h.executeTask(ctx, req)
	if err != nil {
		eventType := "failed"
		message := err.Error()
		if ctx.Err() != nil {
			eventType = "cancelled"
			message = "任务已取消"
		}
		_ = h.emitTaskEvent(context.Background(), &sdkv1.TaskEvent{
			TaskId:   taskID,
			Type:     eventType,
			Message:  message,
			Progress: 0,
			Payload:  []byte("null"),
		})
		return
	}

	_ = h.emitTaskEvent(context.Background(), &sdkv1.TaskEvent{
		TaskId:   taskID,
		Type:     "completed",
		Message:  "任务执行完成",
		Progress: 100,
		Payload:  result,
	})
}

func (h *LocalHandler) executeTask(ctx context.Context, req *sdkv1.InvokeRequest) ([]byte, error) {
	respBytes, err := h.handleInvoke(ctx, mustMarshal(req))
	if err != nil {
		return nil, err
	}
	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		return nil, fmt.Errorf("unmarshal task invoke response: %w", err)
	}
	if len(resp.GetPayload()) == 0 {
		return []byte("null"), nil
	}
	return resp.GetPayload(), nil
}

func (h *LocalHandler) emitTaskEvent(ctx context.Context, event *sdkv1.TaskEvent) error {
	h.mu.RLock()
	reporter := h.reporter
	h.mu.RUnlock()
	if reporter == nil || event == nil {
		return nil
	}
	if len(event.GetPayload()) == 0 {
		event.Payload = []byte("null")
	}
	return reporter.ReportTaskEvent(ctx, event)
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

// taskIndex tracks running tasks.
type taskIndex struct {
	mu    sync.RWMutex
	tasks map[string]context.CancelFunc
}

func newTaskIndex() *taskIndex {
	return &taskIndex{
		tasks: make(map[string]context.CancelFunc),
	}
}

func (j *taskIndex) Set(taskID string, cancel context.CancelFunc) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.tasks[taskID] = cancel
}

func (j *taskIndex) Get(taskID string) (context.CancelFunc, bool) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	cancel, ok := j.tasks[taskID]
	return cancel, ok
}

func (j *taskIndex) Delete(taskID string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	delete(j.tasks, taskID)
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
