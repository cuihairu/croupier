package assignment

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

// LoadScopeAssignments reads all assignment records (scope key "gameID|env" → function list).
// Shared by the execution gate and page CRUD, backed by the same file source of truth.
func LoadScopeAssignments(svcCtx *svc.ServiceContext) (map[string][]string, error) {
	data, err := os.ReadFile(AssignmentsPath(svcCtx))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string][]string{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return map[string][]string{}, nil
	}
	var assignments map[string][]string
	if err := json.Unmarshal(data, &assignments); err != nil {
		return nil, err
	}
	if assignments == nil {
		assignments = map[string][]string{}
	}
	for key, fns := range assignments {
		if fns == nil {
			delete(assignments, key)
		}
	}
	return assignments, nil
}

// AssignmentsPath resolves the assignments file path from service config.
func AssignmentsPath(svcCtx *svc.ServiceContext) string {
	if svcCtx == nil {
		return filepath.Join("data", "assignments.json")
	}
	path := strings.TrimSpace(svcCtx.Config.Registry.AssignmentsPath)
	if path == "" {
		path = filepath.Join("data", "assignments.json")
	}
	if !filepath.IsAbs(path) {
		if base := strings.TrimSpace(svcCtx.Config.BootstrapData.BaseDir); base != "" && filepath.IsAbs(base) {
			path = filepath.Join(base, path)
		}
	}
	return filepath.Clean(path)
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
		parts := strings.SplitN(key, "|", 2)
		g := strings.TrimSpace(parts[0])
		e := ""
		if len(parts) > 1 {
			e = strings.TrimSpace(parts[1])
		}
		if strings.EqualFold(g, strings.TrimSpace(gameID)) && strings.EqualFold(e, strings.TrimSpace(env)) {
			return functions, true
		}
	}
	return nil, false
}
