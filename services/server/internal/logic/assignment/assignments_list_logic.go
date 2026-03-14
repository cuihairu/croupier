// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package assignment

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type AssignmentsListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取分配列表
func NewAssignmentsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignmentsListLogic {
	return &AssignmentsListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AssignmentsListLogic) AssignmentsList(req *types.AssignmentsListRequest) (resp *types.AssignmentsListResponse, err error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看分配列表", "admin:all", "assignments:read", "assignments:write"); err != nil {
		return nil, err
	}

	path := assignmentsPath(l.svcCtx)
	assignments, err := loadAssignments(path)
	if err != nil {
		return nil, errorx.NewInternalError("读取分配数据失败")
	}

	filtered := filterAssignments(assignments, strings.TrimSpace(req.GameId), strings.TrimSpace(req.Env))
	return &types.AssignmentsListResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"assignments": filtered,
			"total":       len(filtered),
			"page":        req.Page,
			"pageSize":    req.PageSize,
		},
	}, nil
}
