package function

import (
	"context"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
)

type DescriptorsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数描述符列表
func NewDescriptorsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DescriptorsLogic {
	return &DescriptorsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// DescriptorsV2 returns FunctionSpec list from persisted FunctionContract.
// It must not use runtime descriptor collectors as a second Dashboard fact source.
func (l *DescriptorsLogic) DescriptorsV2(req *DescriptorsRequest) (*DescriptorsV2Result, error) {
	if err := l.checkReadPermission(); err != nil {
		return nil, err
	}
	gameID, env, err := l.requireScope(req)
	if err != nil {
		return nil, err
	}

	resource := ""
	if req != nil {
		resource = strings.TrimSpace(firstNonEmpty(req.Type, req.Resource))
	}

	if l.svcCtx == nil || l.svcCtx.DB == nil {
		return nil, errorx.NewInternalError("function contract database is not initialized")
	}
	functionsByID, err := contractsvc.FunctionSpecsByScope(l.ctx, model.NewFunctionContractModel(l.svcCtx.DB), gameID, env)
	if err != nil {
		return nil, err
	}

	functions := make([]spec.FunctionSpec, 0, len(functionsByID))
	for _, fn := range functionsByID {
		if fn.ID == "" {
			continue
		}
		if resource != "" && fn.Resource != resource {
			continue
		}
		functions = append(functions, fn)
	}

	sort.Slice(functions, func(i, j int) bool {
		return functions[i].ID < functions[j].ID
	})

	return &DescriptorsV2Result{
		Functions: functions,
	}, nil
}

func (l *DescriptorsLogic) requireScope(req *DescriptorsRequest) (string, string, error) {
	scope := svc.GameScopeFromContext(l.ctx)
	gameID := scope.GameID
	env := scope.Env
	if req != nil && strings.TrimSpace(req.GameId) != "" {
		gameID = strings.TrimSpace(req.GameId)
	}
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	if gameID == "" {
		return "", "", errorx.NewBadRequest("X-Game-ID is required")
	}
	if env == "" {
		return "", "", errorx.NewBadRequest("X-Env is required")
	}
	return gameID, env, nil
}

func (l *DescriptorsLogic) checkReadPermission() error {
	username, _ := utils.CurrentUsername(l.ctx)
	if username == "" {
		return nil
	}

	_, rolesFromDB, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return err
	}
	roleNames := utils.RoleNamesFromModels(rolesFromDB)
	permIDs, err := utils.PermissionIDsFromRoles(l.ctx, l.svcCtx, rolesFromDB)
	if err != nil {
		return err
	}
	if utils.HasAdminRole(roleNames) || utils.HasPermissionID(permIDs, "functions:read") || utils.HasPermissionID(permIDs, "*") {
		return nil
	}
	return errorx.NewForbidden("无权访问函数目录")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}
