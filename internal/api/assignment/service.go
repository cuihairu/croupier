package assignment

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
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

	gameID := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)
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

// Update updates assignments
func (s *Service) Update(ctx context.Context, req *AssignmentsUpdateRequest) (*AssignmentsUpdateResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权更新分配", "admin:all", "assignments:write"); err != nil {
		return nil, err
	}

	gameID := strings.TrimSpace(req.GameId)
	if gameID == "" {
		return nil, errorx.NewBadRequest("game_id不能为空")
	}
	env := strings.TrimSpace(req.Env)
	action := strings.TrimSpace(req.Action)

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
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"ok":          true,
			"unknown":     unknown,
			"assignments": map[string][]string{key: accepted},
		},
	}, nil
}
