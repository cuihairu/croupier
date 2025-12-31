// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package assignment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AssignmentsUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新分配
func NewAssignmentsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignmentsUpdateLogic {
	return &AssignmentsUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AssignmentsUpdateLogic) AssignmentsUpdate(req *types.AssignmentsUpdateRequest) (resp *types.AssignmentsUpdateResponse, err error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权更新分配", "admin:all", "assignments:write"); err != nil {
		return nil, err
	}

	gameID := strings.TrimSpace(req.GameId)
	if gameID == "" {
		return nil, errors.New("game_id不能为空")
	}
	env := strings.TrimSpace(req.Env)

	functions := normalizeFunctions(req.Functions)
	path := assignmentsPath(l.svcCtx)
	assignments, err := loadAssignments(path)
	if err != nil {
		return nil, fmt.Errorf("读取分配数据失败: %w", err)
	}

	known := collectKnownFunctions(l.svcCtx)
	accepted, unknown := splitKnownAndUnknown(functions, known)
	key := buildAssignmentKey(gameID, env)
	assignments[key] = accepted

	if err := saveAssignments(path, assignments); err != nil {
		return nil, fmt.Errorf("保存分配数据失败: %w", err)
	}

	return &types.AssignmentsUpdateResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"ok":          true,
			"unknown":     unknown,
			"assignments": map[string][]string{key: accepted},
		},
	}, nil
}

func collectKnownFunctions(ctx *svc.ServiceContext) map[string]struct{} {
	if ctx == nil || ctx.RegistryStore == nil {
		return nil
	}
	idx := ctx.RegistryStore.BuildFunctionIndex()
	if len(idx) == 0 {
		return nil
	}
	known := make(map[string]struct{}, len(idx))
	for id := range idx {
		known[id] = struct{}{}
	}
	return known
}
