package agent

import (
	"context"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

// opsServerWrapper wraps OpsServer to implement agent.OpsServerWrapper interface
type opsServerWrapper struct {
	ops *OpsServer
}

func (w *opsServerWrapper) GetSystemInfo(ctx context.Context, req *emptypb.Empty) (*opsv1.SystemInfo, error) {
	return w.ops.GetSystemInfo(ctx, req)
}

func (w *opsServerWrapper) ListProcesses(ctx context.Context, req *emptypb.Empty) (*opsv1.ListProcessesResponse, error) {
	return w.ops.ListProcesses(ctx, req)
}

func (w *opsServerWrapper) ReportMetrics(ctx context.Context, req *opsv1.MetricsReport) (*emptypb.Empty, error) {
	return w.ops.ReportMetrics(ctx, req)
}

func (w *opsServerWrapper) RestartProcess(ctx context.Context, req *opsv1.RestartProcessRequest) (*opsv1.RestartProcessResponse, error) {
	return w.ops.RestartProcess(ctx, req)
}

func (w *opsServerWrapper) StopProcess(ctx context.Context, req *opsv1.StopProcessRequest) (*opsv1.StopProcessResponse, error) {
	return w.ops.StopProcess(ctx, req)
}

func (w *opsServerWrapper) StartProcess(ctx context.Context, req *opsv1.StartProcessRequest) (*opsv1.StartProcessResponse, error) {
	return w.ops.StartProcess(ctx, req)
}

func (w *opsServerWrapper) UpdateProcess(ctx context.Context, req *opsv1.UpdateProcessRequest) (*opsv1.UpdateProcessResponse, error) {
	return w.ops.UpdateProcess(ctx, req)
}

func (w *opsServerWrapper) SetSchedulePause(ctx context.Context, req *opsv1.SchedulePauseRequest) (*opsv1.SchedulePauseResponse, error) {
	return w.ops.SetSchedulePause(ctx, req)
}

func (w *opsServerWrapper) ExecuteCommand(ctx context.Context, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error) {
	return w.ops.ExecuteCommand(ctx, req)
}

func (w *opsServerWrapper) PushFile(ctx context.Context, req *opsv1.PushFileRequest) (*opsv1.PushFileResponse, error) {
	return w.ops.PushFile(ctx, req)
}

func (w *opsServerWrapper) GetSupervisorLog(ctx context.Context, req *opsv1.GetSupervisorLogRequest) (*opsv1.GetSupervisorLogResponse, error) {
	return w.ops.GetSupervisorLog(ctx, req)
}

func (w *opsServerWrapper) ListServicesJSON(ctx context.Context, jsonReq []byte) ([]byte, error) {
	return w.ops.ListServicesJSON(ctx, jsonReq)
}

func (w *opsServerWrapper) GetServiceStatusJSON(ctx context.Context, jsonReq []byte) ([]byte, error) {
	return w.ops.GetServiceStatusJSON(ctx, jsonReq)
}

func (w *opsServerWrapper) ListCronJobsJSON(ctx context.Context) ([]byte, error) {
	return w.ops.ListCronJobsJSON(ctx)
}
