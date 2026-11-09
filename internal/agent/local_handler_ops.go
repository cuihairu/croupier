package agent

import (
	"context"
	"fmt"

	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

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

// handleUpdateProcess handles UpdateProcessRequest
func (h *LocalHandler) handleUpdateProcess(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.UpdateProcessRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal UpdateProcessRequest: %w", err)
	}

	h.mu.RLock()
	ops := h.opsServer
	h.mu.RUnlock()

	if ops == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := ops.UpdateProcess(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("update process: %w", err)
	}

	return proto.Marshal(resp)
}

// handleSchedulePause suspends or resumes a managed process's schedule firing
// (§10.2 暂停与恢复，暂停对象=调度).
func (h *LocalHandler) handleSchedulePause(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.SchedulePauseRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal SchedulePauseRequest: %w", err)
	}

	h.mu.RLock()
	ops := h.opsServer
	h.mu.RUnlock()

	if ops == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := ops.SetSchedulePause(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("schedule pause: %w", err)
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

// handlePushFile writes an OPS-006 configuration file delivered by the server.
func (h *LocalHandler) handlePushFile(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.PushFileRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal PushFileRequest: %w", err)
	}

	h.mu.RLock()
	ops := h.opsServer
	h.mu.RUnlock()
	if ops == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := ops.PushFile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("push file: %w", err)
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

// handleGetSupervisorLog handles GetSupervisorLogRequest（回源读 agent 本地轮转日志）。
func (h *LocalHandler) handleGetSupervisorLog(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.GetSupervisorLogRequest{}
	if len(data) > 0 {
		if err := proto.Unmarshal(data, req); err != nil {
			return nil, fmt.Errorf("unmarshal GetSupervisorLogRequest: %w", err)
		}
	}

	h.mu.RLock()
	ops := h.opsServer
	h.mu.RUnlock()

	if ops == nil {
		return nil, fmt.Errorf("ops server not configured")
	}

	resp, err := ops.GetSupervisorLog(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get supervisor log: %w", err)
	}
	return proto.Marshal(resp)
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
