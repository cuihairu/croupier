package function_calls

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionCallsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionCallsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionCallsListLogic {
	return &FunctionCallsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionCallsListLogic) FunctionCallsList(req *types.FunctionCallsListRequest) (*types.FunctionCallsListResponse, error) {
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
		return nil, errorx.NewForbidden("无权查看调用历史")
	}

	// Build list options
	opts := &model.ListOptions{
		FunctionID: strings.TrimSpace(req.FunctionID),
		GameID:     strings.TrimSpace(req.GameID),
		Env:        strings.TrimSpace(req.Env),
		Status:     strings.TrimSpace(req.Status),
		ActorID:    strings.TrimSpace(req.ActorID),
		AgentID:    strings.TrimSpace(req.AgentID),
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
			opts.EndTime = t.Add(24 * time.Hour) // End of day
		}
	}

	// Pagination
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}
	opts.Offset = (req.Page - 1) * req.PageSize
	opts.Limit = req.PageSize

	// Query database
	histories, total, err := l.svcCtx.JobHistoryModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	// Convert to response
	calls := make([]types.FunctionCallItem, 0, len(histories))
	for _, h := range histories {
		calls = append(calls, convertToFunctionCallItem(h))
	}

	return &types.FunctionCallsListResponse{
		Calls:    calls,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func convertToFunctionCallItem(h *model.JobHistory) types.FunctionCallItem {
	item := types.FunctionCallItem{
		ID:         h.ID,
		JobID:      h.JobID,
		FunctionID: h.FunctionID,
		GameID:     h.GameID,
		Env:        h.Env,
		ActorID:    h.ActorID,
		ActorType:  h.ActorType,
		Status:     h.Status,
		AgentID:    h.AgentID,
		ServiceID:  h.ServiceID,
		DurationMs: h.DurationMs,
		ErrorMsg:   h.ErrorMsg,
		RetryCount: h.RetryCount,
		CreatedAt:  h.CreatedAt.Format(time.RFC3339),
	}

	if h.StartedAt != nil {
		item.StartedAt = h.StartedAt.Format(time.RFC3339)
	}
	if h.FinishedAt != nil {
		item.FinishedAt = h.FinishedAt.Format(time.RFC3339)
	}
	if h.Payload != nil {
		_ = json.Unmarshal(h.Payload, &item.Payload)
	}
	if h.Result != nil {
		_ = json.Unmarshal(h.Result, &item.Result)
	}

	return item
}

// Helper for ID conversion
func parseID(idStr string) (int64, error) {
	return strconv.ParseInt(idStr, 10, 64)
}
