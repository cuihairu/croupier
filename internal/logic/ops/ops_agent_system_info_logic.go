
package ops

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/internal/svc"
	

	"google.golang.org/protobuf/types/known/timestamppb"
)

type OpsAgentSystemInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Agent 系统信息
func NewOpsAgentSystemInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentSystemInfoLogic {
	return &OpsAgentSystemInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// formatTimestamp 将 timestamppb.Timestamp 格式化为 RFC3339 字符串
func formatTimestamp(ts *timestamppb.Timestamp) string {
	if ts == nil {
		return ""
	}
	return ts.AsTime().Format(time.RFC3339)
}

func (l *OpsAgentSystemInfoLogic) OpsAgentSystemInfo(req *OpsAgentSystemInfoRequest) (*OpsAgentSystemInfoResponse, error) {
	// Try cache first
	if info, ok := l.svcCtx.SystemInfoCache.Get(req.AgentID); ok {
		return &OpsAgentSystemInfoResponse{
			Code:    0,
			Message: "OK",
			Data: OpsAgentSystemInfo{
				Hostname:      info.Hostname,
				OS:            info.Os,
				OSVersion:     info.OsVersion,
				KernelVersion: info.KernelVersion,
				Arch:          info.Arch,
				CPUCores:      info.CpuCores,
				TotalMemory:   info.TotalMemory,
				BootTime:      formatTimestamp(info.BootTime),
				AgentVersion:  info.AgentVersion,
			},
		}, nil
	}

	// Query agent directly via gRPC
	client, err := GetAgentOpsClient().GetClient(l.ctx, req.AgentID)
	if err != nil {
		return &OpsAgentSystemInfoResponse{
			Code:    404,
			Message: err.Error(),
		}, nil
	}

	info, err := client.GetSystemInfo(l.ctx)
	if err != nil {
		return &OpsAgentSystemInfoResponse{
			Code:    500,
			Message: "Failed to get system info: " + err.Error(),
		}, nil
	}

	// Cache the result
	l.svcCtx.SystemInfoCache.Set(req.AgentID, info)

	return &OpsAgentSystemInfoResponse{
		Code:    0,
		Message: "OK",
		Data: OpsAgentSystemInfo{
			Hostname:      info.Hostname,
			OS:            info.Os,
			OSVersion:     info.OsVersion,
			KernelVersion: info.KernelVersion,
			Arch:          info.Arch,
			CPUCores:      info.CpuCores,
			TotalMemory:   info.TotalMemory,
			BootTime:      formatTimestamp(info.BootTime),
			AgentVersion:  info.AgentVersion,
		},
	}, nil
}
