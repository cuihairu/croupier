// Package ops provides Agent Ops client for Server to communicate with Agents via TCP session
package ops

import (
	"context"
	"log/slog"
	"sync"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/transport"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// AgentSessionResolver finds active TCP sessions for connected Agents.
type AgentSessionResolver interface {
	ResolveAgentConn(agentID string) (transport.SessionCaller, bool)
}

// AgentOpsClient manages Ops communication with Agents via TCP sessions
type AgentOpsClient struct {
	resolver AgentSessionResolver
	mu       sync.RWMutex
}

var (
	globalAgentOpsClient *AgentOpsClient
	opsClientOnce        sync.Once
)

// InitAgentOpsClient initializes the global AgentOpsClient
// The session resolver must be set via SetSessionResolver before use.
func InitAgentOpsClient() {
	opsClientOnce.Do(func() {
		globalAgentOpsClient = &AgentOpsClient{}
		slog.Info("Global AgentOpsClient initialized")
	})
}

// GetAgentOpsClient returns the global AgentOpsClient
func GetAgentOpsClient() *AgentOpsClient {
	if globalAgentOpsClient == nil {
		InitAgentOpsClient()
	}
	return globalAgentOpsClient
}

// SetSessionResolver sets the TCP session resolver
func (c *AgentOpsClient) SetSessionResolver(resolver AgentSessionResolver) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resolver = resolver
}

// GetClient returns an Ops client wrapper for the specified agentID
func (c *AgentOpsClient) GetClient(ctx context.Context, agentID string) (*OpsClientWrapper, error) {
	c.mu.RLock()
	resolver := c.resolver
	c.mu.RUnlock()

	if resolver == nil {
		return nil, errorx.NewInternalError("session resolver not configured")
	}

	caller, ok := resolver.ResolveAgentConn(agentID)
	if !ok {
		return nil, errorx.NewNotFound("agent session not found: " + agentID)
	}

	return &OpsClientWrapper{caller: caller}, nil
}

// OpsClientWrapper wraps TCP session caller to implement Ops methods
type OpsClientWrapper struct {
	caller transport.SessionCaller
}

// GetSystemInfo gets system info from the agent
func (w *OpsClientWrapper) GetSystemInfo(ctx context.Context) (*opsv1.SystemInfo, error) {
	_, data, err := w.caller.Call(ctx, protocol.MsgGetSystemInfoRequest, []byte{})
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
	_, data, err := w.caller.Call(ctx, protocol.MsgListProcessesRequest, []byte{})
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

	_, _, err = w.caller.Call(ctx, protocol.MsgReportMetricsRequest, data)
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

	_, respData, err := w.caller.Call(ctx, protocol.MsgRestartProcessRequest, data)
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

	_, respData, err := w.caller.Call(ctx, protocol.MsgStopProcessRequest, data)
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

	_, respData, err := w.caller.Call(ctx, protocol.MsgStartProcessRequest, data)
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

	_, respData, err := w.caller.Call(ctx, protocol.MsgExecuteCommandRequest, data)
	if err != nil {
		return nil, err
	}

	resp := &opsv1.ExecuteCommandResponse{}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return nil, errorx.NewInternalError("unmarshal execute response failed")
	}

	return resp, nil
}
