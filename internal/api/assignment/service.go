package assignment

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns the list of assignments
func (s *Service) List(ctx context.Context, req *AssignmentsListRequest) (*AssignmentsListResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看分配列表", "admin:all", "assignments:read", "assignments:write"); err != nil {
		return nil, err
	}

	path := assignmentsPath(s.svcCtx)
	assignments, err := loadAssignments(path)
	if err != nil {
		return nil, errorx.NewInternalError("读取分配数据失败")
	}

	gameID := svc.ResolveGameID(ctx, req.GameId)
	env := svc.ResolveEnv(ctx, req.Env)
	filtered := filterAssignments(assignments, gameID, env)
	return &AssignmentsListResponse{
		Assignments: filtered,
		Total:       len(filtered),
		Page:        req.Page,
		PageSize:    req.PageSize,
	}, nil
}

// History returns the history of assignments
func (s *Service) History(ctx context.Context, req *AssignmentsHistoryRequest) (*AssignmentsHistoryResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看分配历史", "admin:all", "assignments:read", "assignments:write"); err != nil {
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

	gameID := svc.ResolveGameID(ctx, req.GameId)
	env := svc.ResolveEnv(ctx, req.Env)
	action := strings.TrimSpace(req.Action)
	path := assignmentHistoryPath(s.svcCtx)
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

	return &AssignmentsHistoryResponse{
		Items:    filtered[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// Update updates assignments
func (s *Service) Update(ctx context.Context, req *AssignmentsUpdateRequest) (*AssignmentsUpdateResponse, error) {
	roles, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权更新分配", "admin:all", "assignments:write")
	if err != nil {
		return nil, err
	}

	gameID := svc.ResolveGameID(ctx, req.GameId)
	if gameID == "" {
		return nil, errorx.NewBadRequest("game_id不能为空")
	}
	env := svc.ResolveEnv(ctx, req.Env)
	action := strings.TrimSpace(req.Action)
	if action == "clone" {
		targetEnv := strings.TrimSpace(req.TargetEnv)
		if targetEnv == "" {
			return nil, errorx.NewBadRequest("target_env不能为空")
		}
		if err := s.authorizeCloneTarget(ctx, gameID, targetEnv, roles); err != nil {
			return nil, err
		}
		env = targetEnv
	}

	functions := normalizeFunctions(req.Functions)
	path := assignmentsPath(s.svcCtx)
	assignments, err := loadAssignments(path)
	if err != nil {
		return nil, errorx.NewInternalError("读取分配数据失败")
	}

	known := collectKnownFunctions(s.svcCtx)
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

	username, _ := utils.CurrentUsername(ctx)
	if strings.TrimSpace(username) == "" {
		username = "system"
	}
	_ = appendAssignmentHistory(s.svcCtx, assignmentHistoryEntry{
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
		OK:          true,
		Unknown:     unknown,
		Assignments: map[string][]string{key: accepted},
	}, nil
}

// authorizeCloneTarget validates the explicitly requested destination scope.
// Unlike game_id/env, target_env is an operation parameter and is never used
// as the source request scope.
func (s *Service) authorizeCloneTarget(ctx context.Context, gameID, targetEnv string, roles []model.Role) error {
	if s == nil || s.svcCtx == nil || s.svcCtx.GameModel == nil {
		return errorx.NewInternalError("游戏环境模型未初始化")
	}

	bound, err := s.svcCtx.GameModel.HasEnvBinding(ctx, gameID, targetEnv)
	if err != nil {
		return errorx.NewInternalError("校验目标环境失败")
	}
	if !bound {
		return errorx.NewBadRequest("目标环境不存在")
	}

	admin, _, err := utils.LoadCurrentAdmin(ctx, s.svcCtx)
	if err != nil {
		return err
	}
	return utils.RequireGameEnvScope(ctx, s.svcCtx, admin.ID, utils.RoleNamesFromModels(roles), gameID, targetEnv)
}
