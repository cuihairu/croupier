// Package ops provides Agent Ops client for Server to communicate with Agents via NNG
package ops

import (
	"context"
	"log/slog"
	"sync"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/nng"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// AgentOpsClient manages NNG connections to multiple Agents
type AgentOpsClient struct {
	agents map[string]*nng.Client // agentID -> NNG client
	mu     sync.RWMutex
}

var (
	globalAgentOpsClient *AgentOpsClient
	agentOpsClientOnce   sync.Once
)

// InitAgentOpsClient initializes the global AgentOpsClient with a registry store
func InitAgentOpsClient(store interface{}) {
	agentOpsClientOnce.Do(func() {
		globalAgentOpsClient = &AgentOpsClient{
			agents: make(map[string]*nng.Client),
		}
		slog.Info("Global AgentOpsClient initialized")
	})
}

// GetAgentOpsClient returns the global AgentOpsClient
func GetAgentOpsClient() *AgentOpsClient {
	if globalAgentOpsClient == nil {
		// Initialize with empty store if not initialized
		InitAgentOpsClient(nil)
	}
	return globalAgentOpsClient
}

// GetClient returns an Ops client wrapper for the specified agentID
func (c *AgentOpsClient) GetClient(ctx context.Context, agentID string) (*OpsClientWrapper, error) {
	c.mu.RLock()
	client, ok := c.agents[agentID]
	c.mu.RUnlock()

	if ok && client.IsRunning() {
		return &OpsClientWrapper{client: client}, nil
	}

	// Need to get agent address from registry
	// For now, return error - the agent should be connected via heartbeat
	return nil, errorx.NewNotFound("agent not connected or found: " + agentID)
}

// RegisterClient registers an NNG client for an agent
func (c *AgentOpsClient) RegisterClient(agentID, addr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close existing client if any
	if existing, ok := c.agents[agentID]; ok && existing.IsRunning() {
		existing.Close()
	}

	client := nng.NewClient(addr)
	if err := client.Dial(); err != nil {
		return errorx.NewInternalError("failed to dial agent")
	}

	c.agents[agentID] = client
	slog.Info("Registered Agent Ops client", "agent_id", agentID, "addr", addr)
	return nil
}

// UnregisterClient removes an agent's client
func (c *AgentOpsClient) UnregisterClient(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if client, ok := c.agents[agentID]; ok {
		client.Close()
		delete(c.agents, agentID)
		slog.Info("Unregistered Agent Ops client", "agent_id", agentID)
	}
}

// Close closes all agent connections
func (c *AgentOpsClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, client := range c.agents {
		client.Close()
	}
	c.agents = make(map[string]*nng.Client)

	return nil
}

// OpsClientWrapper wraps NNG client to implement Ops-like methods
type OpsClientWrapper struct {
	client *nng.Client
}

// GetSystemInfo gets system info from the agent
func (w *OpsClientWrapper) GetSystemInfo(ctx context.Context) (*opsv1.SystemInfo, error) {
	data, err := w.client.Call(ctx, protocol.MsgGetSystemInfoRequest, []byte{})
	if err != nil {
		return nil, err
	}

	resp := &opsv1.SystemInfo{}
	if err := proto.Unmarshal(data, resp); err != nil {
		return nil, errorx.NewInternalError("unmarshal system info failed")
	}

	return resp, nil
}

// ListProcesses lists processes on the agent
func (w *OpsClientWrapper) ListProcesses(ctx context.Context) (*opsv1.ListProcessesResponse, error) {
	data, err := w.client.Call(ctx, protocol.MsgListProcessesRequest, []byte{})
	if err != nil {
		return nil, err
	}

	resp := &opsv1.ListProcessesResponse{}
	if err := proto.Unmarshal(data, resp); err != nil {
		return nil, errorx.NewInternalError("unmarshal process list failed")
	}

	return resp, nil
}

// ReportMetrics sends metrics report to the agent
func (w *OpsClientWrapper) ReportMetrics(ctx context.Context, req *opsv1.MetricsReport) (*opsv1.MetricsReport, error) {
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, errorx.NewInternalError("marshal metrics report failed")
	}

	_, err = w.client.Call(ctx, protocol.MsgReportMetricsRequest, data)
	if err != nil {
		return nil, err
	}

	// Return the request (no response body for metrics)
	return req, nil
}

// RestartProcess restarts a process on the agent
func (w *OpsClientWrapper) RestartProcess(ctx context.Context, req *opsv1.RestartProcessRequest) (*opsv1.RestartProcessResponse, error) {
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, errorx.NewInternalError("marshal restart request failed")
	}

	respData, err := w.client.Call(ctx, protocol.MsgRestartProcessRequest, data)
	if err != nil {
		return nil, err
	}

	resp := &opsv1.RestartProcessResponse{}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return nil, errorx.NewInternalError("unmarshal restart response failed")
	}

	return resp, nil
}

// StopProcess stops a process on the agent
func (w *OpsClientWrapper) StopProcess(ctx context.Context, req *opsv1.StopProcessRequest) (*opsv1.StopProcessResponse, error) {
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, errorx.NewInternalError("marshal stop request failed")
	}

	respData, err := w.client.Call(ctx, protocol.MsgStopProcessRequest, data)
	if err != nil {
		return nil, err
	}

	resp := &opsv1.StopProcessResponse{}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return nil, errorx.NewInternalError("unmarshal stop response failed")
	}

	return resp, nil
}

// StartProcess starts a process on the agent
func (w *OpsClientWrapper) StartProcess(ctx context.Context, req *opsv1.StartProcessRequest) (*opsv1.StartProcessResponse, error) {
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, errorx.NewInternalError("marshal start request failed")
	}

	respData, err := w.client.Call(ctx, protocol.MsgStartProcessRequest, data)
	if err != nil {
		return nil, err
	}

	resp := &opsv1.StartProcessResponse{}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return nil, errorx.NewInternalError("unmarshal start response failed")
	}

	return resp, nil
}

// ExecuteCommand executes a command on the agent
func (w *OpsClientWrapper) ExecuteCommand(ctx context.Context, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error) {
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, errorx.NewInternalError("marshal execute request failed")
	}

	respData, err := w.client.Call(ctx, protocol.MsgExecuteCommandRequest, data)
	if err != nil {
		return nil, err
	}

	resp := &opsv1.ExecuteCommandResponse{}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return nil, errorx.NewInternalError("unmarshal execute response failed")
	}

	return resp, nil
}
