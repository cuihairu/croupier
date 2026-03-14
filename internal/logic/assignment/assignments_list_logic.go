package assignment

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
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

func (l *AssignmentsListLogic) AssignmentsList(req *AssignmentsListRequest) (resp *AssignmentsListResponse, err error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看分配列表", "admin:all", "assignments:read", "assignments:write"); err != nil {
		return nil, err
	}

	path := assignmentsPath(l.svcCtx)
	assignments, err := loadAssignments(path)
	if err != nil {
		return nil, errorx.NewInternalError("读取分配数据失败")
	}

	filtered := filterAssignments(assignments, strings.TrimSpace(req.GameId), strings.TrimSpace(req.Env))
	return &AssignmentsListResponse{
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
