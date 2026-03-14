
package assignment

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
	
)

type AssignmentsUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新分配
func NewAssignmentsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignmentsUpdateLogic {
	return &AssignmentsUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AssignmentsUpdateLogic) AssignmentsUpdate(req *AssignmentsUpdateRequest) (resp *AssignmentsUpdateResponse, err error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权更新分配", "admin:all", "assignments:write"); err != nil {
		return nil, err
	}

	gameID := strings.TrimSpace(req.GameId)
	if gameID == "" {
		return nil, errorx.NewBadRequest("game_id不能为空")
	}
	env := strings.TrimSpace(req.Env)
	action := strings.TrimSpace(req.Action)

	functions := normalizeFunctions(req.Functions)
	path := assignmentsPath(l.svcCtx)
	assignments, err := loadAssignments(path)
	if err != nil {
		return nil, errorx.NewInternalError("读取分配数据失败")
	}

	known := collectKnownFunctions(l.svcCtx)
	accepted, unknown := splitKnownAndUnknown(functions, known)
	key := buildAssignmentKey(gameID, env)
	before := append([]string(nil), assignments[key]...)
	assignments[key] = accepted
	added, removed := diffFunctions(before, accepted)
	if action == "" {
		if len(accepted) == 0 && len(before) > 0 {
			action = "remove"
		} else {
			action = "assign"
		}
	}

	if err := saveAssignments(path, assignments); err != nil {
		return nil, errorx.NewInternalError("保存分配数据失败")
	}

	username, _ := utils.CurrentUsername(l.ctx)
	if strings.TrimSpace(username) == "" {
		username = "system"
	}
	_ = appendAssignmentHistory(l.svcCtx, assignmentHistoryEntry{
		GameID:     gameID,
		Env:        env,
		FunctionID: "all",
		Action:     action,
		Count:      len(accepted),
		OperatedBy: username,
		Details: map[string]interface{}{
			"before":  before,
			"after":   accepted,
			"added":   added,
			"removed": removed,
			"unknown": unknown,
		},
	})

	return &AssignmentsUpdateResponse{
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
	operations := ctx.RegistryStore.ListOpenAPIOperations()
	if len(operations) == 0 {
		return nil
	}
	known := make(map[string]struct{}, len(operations))
	for id := range operations {
		known[id] = struct{}{}
	}
	return known
}

func diffFunctions(before, after []string) (added, removed []string) {
	beforeSet := make(map[string]struct{}, len(before))
	afterSet := make(map[string]struct{}, len(after))
	for _, fn := range before {
		beforeSet[fn] = struct{}{}
	}
	for _, fn := range after {
		afterSet[fn] = struct{}{}
		if _, ok := beforeSet[fn]; !ok {
			added = append(added, fn)
		}
	}
	for _, fn := range before {
		if _, ok := afterSet[fn]; !ok {
			removed = append(removed, fn)
		}
	}
	return
}
