// Package croupier provides a Go SDK for Croupier game function registration and execution.
package croupier

import (
	"context"
	"fmt"
	"sync"
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

	// Server for handling incoming RPC calls
	rpcHandler *tcpRPCHandler

	// Task management
	tasks      map[string]*Task
	tasksMutex sync.RWMutex
	tasksSeq   uint64
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
	return handler
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

func (m *TCPManager) RegisterWithAgent(ctx context.Context, serviceID, serviceVersion string, functions []LocalFunctionDescriptor) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return "", fmt.Errorf("not connected to agent")
	}

	logInfof("Registering service %s@%s with %d functions", serviceID, serviceVersion, len(functions))

	descriptors := make([]*sdkv1.LocalFunctionDescriptor, len(functions))
	for i, f := range functions {
		descriptors[i] = &sdkv1.LocalFunctionDescriptor{
			Id:           f.ID,
			Version:      f.Version,
			Tags:         f.Tags,
			Summary:      f.Summary,
			Description:  f.Description,
			OperationId:  f.OperationID,
			Deprecated:   f.Deprecated,
			InputSchema:  f.InputSchema,
			OutputSchema: f.OutputSchema,
			Category:     f.Category,
			Risk:         f.Risk,
			Entity:       f.Entity,
			Operation:    f.Operation,
		}
	}

	req := &sdkv1.ProviderConnectRequest{
		ServiceId:   serviceID,
		Version:     serviceVersion,
		Functions:   descriptors,
		SdkLanguage: "go",
		SdkVersion:  "1.0.0",
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

	logInfof("Registered successfully, session ID: %s", m.sessionID)

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

func (m *TCPManager) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.sendHeartbeat(ctx); err != nil {
				logErrorf("Heartbeat failed: %v", err)
			}
		}
	}
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

	_, _, err = client.Call(ctx, protocol.MsgProviderHeartbeatRequest, reqBody)
	return err
}

func (h *tcpRPCHandler) invoke(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error) {
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

	taskCtx, cancel := context.WithCancel(ctx)

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
		Payload: []byte(fmt.Sprintf(`{"task_id":"%s","cancelled":%v}`, req.TaskId, ok)),
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
