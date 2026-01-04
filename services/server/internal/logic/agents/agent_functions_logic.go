package agents

import (
	"context"
	"sort"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AgentFunctionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAgentFunctionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentFunctionsLogic {
	return &AgentFunctionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentFunctionsLogic) AgentFunctions(req *types.AgentFunctionsRequest) (*types.AgentFunctionsResponse, error) {
	// Permission check
	_, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)
	permIDs, err := utils.PermissionIDsFromRoles(l.ctx, l.svcCtx, roles)
	if err != nil {
		return nil, err
	}
	if !utils.HasAdminRole(roleNames) && !utils.HasPermissionID(permIDs, "agents:read") && !utils.HasPermissionID(permIDs, "*") {
		return nil, errorx.NewForbidden("无权查看 Agent 函数")
	}

	if l.svcCtx.RegistryStore == nil {
		return nil, errorx.NewInternalError("Registry 未初始化")
	}

	store := l.svcCtx.RegistryStore
	store.Mu().RLock()
	defer store.Mu().RUnlock()

	// Find agent by ID
	var targetSession *registry.AgentSession
	for _, sess := range store.AgentsUnsafe() {
		if sess != nil && sess.AgentID == req.ID {
			targetSession = sess
			break
		}
	}

	if targetSession == nil {
		return nil, errorx.NewNotFound("Agent 不存在")
	}

	// Build functions list
	functions := make([]types.AgentFunction, 0, len(targetSession.Functions))
	for fnID, meta := range targetSession.Functions {
		functions = append(functions, types.AgentFunction{
			FunctionID: fnID,
			Version:    meta.Version,
			Enabled:    meta.Enabled,
		})
	}

	// Sort by function ID
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].FunctionID < functions[j].FunctionID
	})

	return &types.AgentFunctionsResponse{
		Functions: functions,
	}, nil
}
