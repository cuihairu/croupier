// Package croupier provides a Go SDK for Croupier game function registration and execution.
package croupier

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	agentv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/agent/v1"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/transport"
)

// TCPManager implements Manager interface using TCP transport
type TCPManager struct {
	config   ClientConfig
	handlers map[string]FunctionHandler

	// Transport layer
	client *transport.TCPClient

	// Connection state
	mu        sync.RWMutex
	connected bool

	// Session management
	sessionID      string
	serviceID      string
	serviceVersion string
	heartbeatStop  context.CancelFunc

	// Stored function descriptors for re-registration after reconnect
	functions []*sdkv1.ProviderFunctionDescriptor

	// Server for handling incoming RPC calls
	rpcHandler *tcpRPCHandler

	// Task management
	tasks      map[string]*Task
	tasksMutex sync.RWMutex
	tasksSeq   uint64

	// Reconnection callback — called when connection is lost
	onDisconnect func()

	// Drain 状态：收到 ProviderDrainRequest 后置位——拒绝新 Invoke，
	// 在途调用清零后经 handleDisconnect 走既有重连编排恢复会话。
	draining      atomic.Bool
	inflightCalls atomic.Int64
}

// Task represents an async task execution
type Task struct {
	ID         string
	FunctionID string
	Payload    []byte
	Status     agentv1.TaskStatus
	CreatedAt  int64
	UpdatedAt  int64
	Result     []byte
	Error      string
	Progress   int32
	Cancel     context.CancelFunc
}

// tcpRPCHandler implements the transport.Handler interface
type tcpRPCHandler struct {
	manager *TCPManager
	methods map[uint32]func(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error)
}

// NewTCPManager creates a new TCP-based manager
func NewTCPManager(config ClientConfig, handlers map[string]FunctionHandler) (Manager, error) {
	m := &TCPManager{
		config:   config,
		handlers: handlers,
		tasks:    make(map[string]*Task),
	}
	m.rpcHandler = newTCPRPCHandler(m)
	return m, nil
}

func newTCPRPCHandler(manager *TCPManager) *tcpRPCHandler {
	handler := &tcpRPCHandler{
		manager: manager,
		methods: make(map[uint32]func(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error)),
	}
	handler.methods[protocol.MsgInvokeRequest] = handler.invoke
	handler.methods[protocol.MsgStartTaskRequest] = handler.startTask
	handler.methods[protocol.MsgCancelTaskRequest] = handler.cancelTask
	handler.methods[protocol.MsgStreamTaskRequest] = handler.streamTask
	// Agent 侧保活探针（LivenessProbe）：pong 必须经 SDK 事件循环回出——
	// SDK 事件循环卡死时回不了 pong，agent 侧即摘除该 session，
	// 调用路由不再选到"进程活着但处理不动"的 provider。
	handler.methods[protocol.MsgProviderHeartbeatRequest] = handler.pong
	handler.methods[protocol.MsgProviderDrainRequest] = handler.handleDrain
	return handler
}

// pong 回应 agent 的保活探测（空响应体）。
func (h *tcpRPCHandler) pong(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error) {
	return proto.Marshal(&sdkv1.ProviderHeartbeatResponse{})
}

// handleDrain 处理 agent 的优雅下线请求：置位 drain 状态（拒绝新 Invoke）、
// 异步等待在途调用清零后经 handleDisconnect 触发既有重连编排；
// 协议规定立即回空 ProviderDrainResponse 确认。幂等：重复 drain 只回确认。
func (h *tcpRPCHandler) handleDrain(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error) {
	m := h.manager
	if m.draining.CompareAndSwap(false, true) {
		var req sdkv1.ProviderDrainRequest
		if err := proto.Unmarshal(body, &req); err == nil {
			logInfof("Drain requested (session=%s, reason=%s, retryAfterMs=%d)", req.SessionId, req.Reason, req.RetryAfterMs)
		} else {
			logWarnf("Drain requested (unparsable body)")
		}
		go m.drainAndRecover()
	}
	return proto.Marshal(&sdkv1.ProviderDrainResponse{})
}

// drainAndRecover 等待在途调用完成（最多 30s），随后断开会话并触发
// client.go 的重连循环（backoff 重拨 + 重注册）。恢复完成后清除 drain 状态。
func (m *TCPManager) drainAndRecover() {
	defer m.draining.Store(false)
	deadline := time.Now().Add(30 * time.Second)
	for m.inflightCalls.Load() > 0 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if n := m.inflightCalls.Load(); n > 0 {
		logWarnf("Drain timeout with %d in-flight call(s) still running", n)
	}
	if m.config.Reconnect == nil || !m.config.Reconnect.Enabled {
		logInfof("Drain complete, auto-reconnect disabled — session closed")
		m.Disconnect()
		return
	}
	logInfof("Drain complete, reconnecting provider session")
	m.handleDisconnect()
}

func (h *tcpRPCHandler) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error) {
	method, ok := h.methods[msgID]
	if !ok {
		return nil, fmt.Errorf("unknown message ID: %s", protocol.MsgIDString(msgID))
	}
	return method(ctx, msgID, reqID, body)
}

func (m *TCPManager) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.connected {
		return nil
	}

	logInfof("Connecting to Croupier Agent via TCP: %s", m.config.AgentAddr)

	client, err := transport.NewTCPClient(&transport.Config{
		Address:     m.config.AgentAddr,
		Insecure:    m.config.Insecure,
		DialTimeout: 30 * time.Second,
		InboundHandler: func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
			return m.rpcHandler.Handle(ctx, msgID, reqID, body)
		},
	})
	if err != nil {
		return fmt.Errorf("create TCP client: %w", err)
	}
	m.client = client
	m.connected = true

	logInfof("Connected to Croupier Agent")
	return nil
}

func (m *TCPManager) Disconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return
	}

	if m.heartbeatStop != nil {
		m.heartbeatStop()
		m.heartbeatStop = nil
	}

	if m.client != nil {
		_ = m.client.Close()
		m.client = nil
	}

	m.connected = false
	m.sessionID = ""
	logInfof("Disconnected from Croupier Agent")
}

func (m *TCPManager) RegisterWithAgent(ctx context.Context, serviceID, serviceVersion string, functions []ProviderFunctionDescriptor) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return "", fmt.Errorf("not connected to agent")
	}

	logInfof("Registering service %s@%s with %d functions", serviceID, serviceVersion, len(functions))

	descriptors := make([]*sdkv1.ProviderFunctionDescriptor, len(functions))
	for i, f := range functions {
		descriptors[i] = &sdkv1.ProviderFunctionDescriptor{
			Id:                f.ID,
			Version:           f.Version,
			Tags:              f.Tags,
			Summary:           f.Summary,
			Description:       f.Description,
			OperationId:       f.OperationID,
			Deprecated:        f.Deprecated,
			InputSchema:       f.InputSchema,
			OutputSchema:      f.OutputSchema,
			Resource:          f.Resource,
			Operation:         f.Operation,
			Capability:        f.Capability,
			Execution:         f.Execution,
			ApprovalRequired:  f.ApprovalRequired,
			ApprovalPolicyKey: f.ApprovalPolicyKey,
			Risk:              f.Risk,
			Enabled:           f.Enabled,
			Permission:        f.Permission,
		}
	}

	req := &sdkv1.ProviderConnectRequest{
		ServiceId:   serviceID,
		Version:     serviceVersion,
		Functions:   descriptors,
		SdkLanguage: "go",
		SdkVersion:  "1.0.0",
		SdkName:     "croupier-go-sdk",
		GameId:      m.config.GameID,
		Env:         m.config.Env,
	}

	reqBody, err := proto.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	respMsgID, respBody, err := m.client.Call(ctx, protocol.MsgProviderConnectRequest, reqBody)
	if err != nil {
		return "", fmt.Errorf("call ProviderConnect: %w", err)
	}

	if respMsgID != protocol.MsgProviderConnectResponse {
		return "", fmt.Errorf("unexpected response message: %s", protocol.MsgIDString(respMsgID))
	}

	resp := &sdkv1.ProviderConnectResponse{}
	if err := proto.Unmarshal(respBody, resp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.SessionId == "" {
		return "", fmt.Errorf("registration failed: no session ID returned")
	}

	m.sessionID = resp.SessionId
	m.serviceID = serviceID
	m.serviceVersion = serviceVersion
	m.functions = descriptors // Store for reconnect

	logInfof("Registered successfully, session ID: %s", m.sessionID)

	// Set onClose callback so we get notified when TCP connection drops
	m.client.SetOnClose(func(err error) {
		logErrorf("Connection lost: %v", err)
		m.handleDisconnect()
	})

	ctx, cancel := context.WithCancel(context.Background())
	m.heartbeatStop = cancel
	go m.heartbeatLoop(ctx)

	return m.sessionID, nil
}

func (m *TCPManager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

// SetOnDisconnect sets a callback that is invoked when the connection is lost.
// The callback is called from the heartbeat goroutine or the TCP onClose handler.
func (m *TCPManager) SetOnDisconnect(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onDisconnect = fn
}

func (m *TCPManager) heartbeatLoop(ctx context.Context) {
	interval := m.config.HeartbeatInterval
	if interval <= 0 {
		interval = 10
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	consecutiveFailures := 0
	maxFailures := 2

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.sendHeartbeat(ctx); err != nil {
				consecutiveFailures++
				logErrorf("Heartbeat failed (%d/%d): %v", consecutiveFailures, maxFailures, err)
				if consecutiveFailures >= maxFailures {
					logErrorf("Heartbeat failed %d times, triggering reconnect", maxFailures)
					m.handleDisconnect()
					return
				}
			} else {
				consecutiveFailures = 0
			}
		}
	}
}

// handleDisconnect sets the connection state to false and notifies the onDisconnect callback.
// It is safe to call multiple times (idempotent).
func (m *TCPManager) handleDisconnect() {
	m.mu.Lock()
	if !m.connected {
		m.mu.Unlock()
		return
	}
	m.connected = false
	m.sessionID = ""
	fn := m.onDisconnect
	// Stop heartbeat if running
	if m.heartbeatStop != nil {
		m.heartbeatStop()
		m.heartbeatStop = nil
	}
	// Close transport
	if m.client != nil {
		_ = m.client.Close()
		m.client = nil
	}
	m.mu.Unlock()

	logInfof("Connection lost, notifying reconnect handler")
	if fn != nil {
		fn()
	}
}

// Reconnect establishes a new connection to the agent and re-registers.
// It is called by the client after handleDisconnect signals via onDisconnect.
func (m *TCPManager) Reconnect(ctx context.Context) error {
	m.mu.Lock()
	if m.connected {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	logInfof("Reconnecting to Croupier Agent: %s", m.config.AgentAddr)

	// Create new TCP client
	client, err := transport.NewTCPClient(&transport.Config{
		Address:     m.config.AgentAddr,
		Insecure:    m.config.Insecure,
		DialTimeout: 30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("reconnect dial: %w", err)
	}

	// Re-register with agent using stored function descriptors
	req := &sdkv1.ProviderConnectRequest{
		ServiceId:   m.serviceID,
		Version:     m.serviceVersion,
		Functions:   m.functions,
		SdkLanguage: "go",
		SdkVersion:  "1.0.0",
		SdkName:     "croupier-go-sdk",
		GameId:      m.config.GameID,
		Env:         m.config.Env,
	}
	reqBody, err := proto.Marshal(req)
	if err != nil {
		client.Close()
		return fmt.Errorf("marshal connect request: %w", err)
	}

	respMsgID, respBody, err := client.Call(ctx, protocol.MsgProviderConnectRequest, reqBody)
	if err != nil {
		client.Close()
		return fmt.Errorf("reconnect register: %w", err)
	}
	if respMsgID != protocol.MsgProviderConnectResponse {
		client.Close()
		return fmt.Errorf("unexpected response: %s", protocol.MsgIDString(respMsgID))
	}

	resp := &sdkv1.ProviderConnectResponse{}
	if err := proto.Unmarshal(respBody, resp); err != nil {
		client.Close()
		return fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.SessionId == "" {
		client.Close()
		return fmt.Errorf("reconnect failed: no session ID")
	}

	// Set onClose on new client
	client.SetOnClose(func(err error) {
		logErrorf("Connection lost: %v", err)
		m.handleDisconnect()
	})

	// Commit
	m.mu.Lock()
	m.client = client
	m.sessionID = resp.SessionId
	m.connected = true
	m.mu.Unlock()

	// Resume heartbeat
	hbCtx, cancel := context.WithCancel(context.Background())
	m.heartbeatStop = cancel
	go m.heartbeatLoop(hbCtx)

	logInfof("Reconnected successfully, session ID: %s", resp.SessionId)
	return nil
}

func (m *TCPManager) sendHeartbeat(ctx context.Context) error {
	m.mu.RLock()
	if !m.connected || m.client == nil {
		m.mu.RUnlock()
		return nil
	}
	client := m.client
	m.mu.RUnlock()

	req := &sdkv1.ProviderHeartbeatRequest{
		SessionId: m.sessionID,
		ServiceId: m.serviceID,
	}

	reqBody, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal heartbeat: %w", err)
	}

	hbCtx, hbCancel := context.WithTimeout(ctx, 30*time.Second)
	defer hbCancel()
	_, _, err = client.Call(hbCtx, protocol.MsgProviderHeartbeatRequest, reqBody)
	return err
}

func (h *tcpRPCHandler) invoke(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error) {
	// drain 期间拒绝新调用，等待 agent 停止投递（对齐 C# 参考实现）。
	if h.manager.draining.Load() {
		errResp := &sdkv1.InvokeResponse{Payload: []byte(`{"error":"provider is draining"}`)}
		if b, marshalErr := proto.Marshal(errResp); marshalErr == nil {
			return b, nil
		}
		return nil, fmt.Errorf("provider is draining")
	}
	h.manager.inflightCalls.Add(1)
	defer h.manager.inflightCalls.Add(-1)

	req := &sdkv1.InvokeRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal InvokeRequest: %w", err)
	}

	h.manager.mu.RLock()
	handler, ok := h.manager.handlers[req.FunctionId]
	h.manager.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("function not found: %s", req.FunctionId)
	}

	// OTel 一期传播：metadata trace 字段进 handler 上下文（零侵入，无则原 ctx）
	ctx = WithTraceMetadata(ctx, req.GetMetadata())
	result, err := handler(ctx, req.Payload)
	if err != nil {
		return nil, err
	}

	resp := &sdkv1.InvokeResponse{
		Payload: result,
	}
	return proto.Marshal(resp)
}

func (h *tcpRPCHandler) startTask(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error) {
	req := &sdkv1.InvokeRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal InvokeRequest: %w", err)
	}

	h.manager.mu.RLock()
	handler, ok := h.manager.handlers[req.FunctionId]
	h.manager.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("function not found: %s", req.FunctionId)
	}

	// OTel 一期传播：任务 handler 上下文同样可读 trace 字段
	taskCtx, cancel := context.WithCancel(WithTraceMetadata(ctx, req.GetMetadata()))

	// Generate new task ID
	h.manager.tasksMutex.Lock()
	h.manager.tasksSeq++
	taskID := fmt.Sprintf("%s-%d", generateUUID(), h.manager.tasksSeq)

	task := &Task{
		ID:         taskID,
		FunctionID: req.FunctionId,
		Payload:    req.Payload,
		Status:     agentv1.TaskStatus_TASK_STATUS_RUNNING,
		CreatedAt:  time.Now().Unix(),
		UpdatedAt:  time.Now().Unix(),
		Cancel:     cancel,
	}
	h.manager.tasks[taskID] = task
	h.manager.tasksMutex.Unlock()

	go func() {
		result, execErr := handler(taskCtx, req.Payload)

		h.manager.tasksMutex.Lock()
		defer h.manager.tasksMutex.Unlock()

		task.UpdatedAt = time.Now().Unix()
		// Don't override status if task was already cancelled
		if task.Status != agentv1.TaskStatus_TASK_STATUS_CANCEL_REQUESTED {
			if execErr != nil {
				task.Status = agentv1.TaskStatus_TASK_STATUS_FAILED
				task.Error = execErr.Error()
			} else {
				task.Status = agentv1.TaskStatus_TASK_STATUS_SUCCEEDED
				task.Result = result
			}
		} else if execErr != nil {
			// Task was cancelled, but still record the error
			task.Error = execErr.Error()
		} else {
			task.Result = result
		}
	}()

	resp := &sdkv1.StartTaskResponse{
		TaskId: taskID,
	}
	return proto.Marshal(resp)
}

func (h *tcpRPCHandler) cancelTask(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error) {
	req := &sdkv1.CancelTaskRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal CancelTaskRequest: %w", err)
	}

	h.manager.tasksMutex.Lock()
	task, ok := h.manager.tasks[req.TaskId]
	if ok {
		task.Status = agentv1.TaskStatus_TASK_STATUS_CANCEL_REQUESTED
		h.manager.tasksMutex.Unlock()
		if task.Cancel != nil {
			task.Cancel()
		}
	} else {
		h.manager.tasksMutex.Unlock()
	}

	// Cancel response uses InvokeResponse format
	resp := &sdkv1.InvokeResponse{
		Payload: []byte(fmt.Sprintf(`{"taskId":"%s","cancelled":%v}`, req.TaskId, ok)),
	}
	return proto.Marshal(resp)
}

func (h *tcpRPCHandler) streamTask(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error) {
	req := &sdkv1.TaskStreamRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, fmt.Errorf("unmarshal TaskStreamRequest: %w", err)
	}

	h.manager.tasksMutex.RLock()
	task, ok := h.manager.tasks[req.TaskId]
	h.manager.tasksMutex.RUnlock()

	if !ok {
		return nil, fmt.Errorf("task not found: %s", req.TaskId)
	}

	// Return task event as JSON in InvokeResponse payload
	event := &sdkv1.TaskEvent{
		TaskId:   req.TaskId,
		Type:     "progress",
		Progress: int32(task.Progress),
		Message:  "task streaming",
	}
	eventBytes, _ := proto.Marshal(event)

	resp := &sdkv1.InvokeResponse{
		Payload: eventBytes,
	}
	return proto.Marshal(resp)
}
