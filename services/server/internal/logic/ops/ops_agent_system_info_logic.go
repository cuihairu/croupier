// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OpsAgentSystemInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Agent 系统信息
func NewOpsAgentSystemInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentSystemInfoLogic {
	return &OpsAgentSystemInfoLogic{
		Logger: logx.WithContext(ctx),
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

func (l *OpsAgentSystemInfoLogic) OpsAgentSystemInfo(req *types.OpsAgentSystemInfoRequest) (*types.OpsAgentSystemInfoResponse, error) {
	// Try cache first
	if info, ok := l.svcCtx.SystemInfoCache.Get(req.AgentID); ok {
		return &types.OpsAgentSystemInfoResponse{
			Code:    0,
			Message: "OK",
			Data: types.OpsAgentSystemInfo{
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
		return &types.OpsAgentSystemInfoResponse{
			Code:    404,
			Message: err.Error(),
		}, nil
	}

	info, err := client.GetSystemInfo(l.ctx)
	if err != nil {
		return &types.OpsAgentSystemInfoResponse{
			Code:    500,
			Message: "Failed to get system info: " + err.Error(),
		}, nil
	}

	// Cache the result
	l.svcCtx.SystemInfoCache.Set(req.AgentID, info)

	return &types.OpsAgentSystemInfoResponse{
		Code:    0,
		Message: "OK",
		Data: types.OpsAgentSystemInfo{
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
