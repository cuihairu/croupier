package function_calls

import (
	"context"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionCallStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionCallStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionCallStatsLogic {
	return &FunctionCallStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionCallStatsLogic) FunctionCallStats(req *types.FunctionCallStatsRequest) (*types.FunctionCallStatsResponse, error) {
	// Permission check
	_, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)
	permIDs, err := utils.PermissionIDsFromRoles(l.ctx, l.svcCtx, roles)
	if err != nil {
		return nil, err
	}
	if !utils.HasAdminRole(roleNames) && !utils.HasPermissionID(permIDs, "function_calls:read") && !utils.HasPermissionID(permIDs, "*") {
		return nil, errorx.NewForbidden("无权查看调用统计")
	}

	// Build list options for stats
	opts := &model.ListOptions{
		FunctionID: strings.TrimSpace(req.FunctionID),
		GameID:     strings.TrimSpace(req.GameID),
		Env:        strings.TrimSpace(req.Env),
		ActorID:    strings.TrimSpace(req.ActorID),
	}

	// Parse time range
	if req.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, req.StartTime); err == nil {
			opts.StartTime = t
		} else if t, err := time.Parse("2006-01-02", req.StartTime); err == nil {
			opts.StartTime = t
		}
	}
	if req.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, req.EndTime); err == nil {
			opts.EndTime = t
		} else if t, err := time.Parse("2006-01-02", req.EndTime); err == nil {
			opts.EndTime = t.Add(24 * time.Hour)
		}
	}

	// Query stats from database
	stats, err := l.svcCtx.JobHistoryModel.GetStats(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	return &types.FunctionCallStatsResponse{
		Total:         stats.Total,
		Succeeded:     stats.Succeeded,
		Failed:        stats.Failed,
		Running:       stats.Running,
		Cancelled:     stats.Cancelled,
		Timeout:       stats.Timeout,
		Other:         stats.Other,
		AvgDurationMs: stats.AvgDurationMs,
	}, nil
}
