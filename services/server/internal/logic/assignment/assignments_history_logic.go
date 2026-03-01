package assignment

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type AssignmentsHistoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssignmentsHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignmentsHistoryLogic {
	return &AssignmentsHistoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AssignmentsHistoryLogic) AssignmentsHistory(req *types.AssignmentsHistoryRequest) (resp *types.AssignmentsHistoryResponse, err error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看分配历史", "admin:all", "assignments:read", "assignments:write"); err != nil {
		return nil, err
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	gameID := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)
	action := strings.TrimSpace(req.Action)
	path := assignmentHistoryPath(l.svcCtx)
	items, err := loadAssignmentHistory(path)
	if err != nil {
		return nil, errorx.NewInternalError("读取分配历史失败")
	}

	filtered := make([]assignmentHistoryEntry, 0, len(items))
	for _, item := range items {
		if gameID != "" && !strings.EqualFold(item.GameID, gameID) {
			continue
		}
		if env != "" && !strings.EqualFold(item.Env, env) {
			continue
		}
		if action != "" && !strings.EqualFold(item.Action, action) {
			continue
		}
		filtered = append(filtered, item)
	}

	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return &types.AssignmentsHistoryResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items":    filtered[start:end],
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	}, nil
}
