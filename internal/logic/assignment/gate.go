package assignment

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

// LoadScopeAssignments reads all assignment records (scope key "gameID|env" → function list).
// Shared by the execution gate and page CRUD, backed by the same file source of truth.
func LoadScopeAssignments(svcCtx *svc.ServiceContext) (map[string][]string, error) {
	return loadAssignments(assignmentsPath(svcCtx))
}

// AssignmentsPath resolves the assignments file path from service config.
func AssignmentsPath(svcCtx *svc.ServiceContext) string {
	return assignmentsPath(svcCtx)
}

// EnsureFunctionAssigned implements the assignment gate (OPEN-ISSUES #36 / BUG-032).
//   - No assignment record for scope (gameID|env) → default open (backward compatible)
//   - Record exists → whitelist: function must be listed, otherwise 403 function_not_assigned
func EnsureFunctionAssigned(ctx context.Context, svcCtx *svc.ServiceContext, functionID, gameID, env string) error {
	if svcCtx == nil {
		return nil
	}
	assignments, err := LoadScopeAssignments(svcCtx)
	if err != nil {
		return err
	}
	functions, ok := lookupScopeList(assignments, gameID, env)
	if !ok {
		return nil
	}
	fnID := strings.TrimSpace(functionID)
	for _, fn := range functions {
		if strings.TrimSpace(fn) == fnID {
			return nil
		}
	}
	return errorx.NewForbiddenWithCode(
		"function_not_assigned",
		"函数未开放执行权限：当前游戏/环境未分配该函数（可在「函数分配」页添加）",
		map[string]any{"functionId": fnID, "gameId": gameID, "env": env},
	)
}

func lookupScopeList(assignments map[string][]string, gameID, env string) ([]string, bool) {
	for key, functions := range assignments {
		scopeGameID, scopeEnv := splitAssignmentKey(key)
		if strings.EqualFold(scopeGameID, strings.TrimSpace(gameID)) && strings.EqualFold(scopeEnv, strings.TrimSpace(env)) {
			return functions, true
		}
	}
	return nil, false
}
