package agents

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AgentDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAgentDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentDetailLogic {
	return &AgentDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentDetailLogic) AgentDetail(req *types.AgentDetailRequest) (*types.AgentDetailResponse, error) {
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
		return nil, errorx.NewForbidden("无权查看 Agent 详情")
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

	ttl, healthy := ttlAndHealth(targetSession)

	// Build agent item
	agentItem := types.AgentItem{
		AgentID:      targetSession.AgentID,
		GameID:       targetSession.GameID,
		Env:          targetSession.Env,
		RPCAddr:      targetSession.RPCAddr,
		Functions:    utils.CountEnabledFunctions(targetSession.Functions),
		Healthy:      healthy,
		ExpiresInSec: ttl,
		LastSeen:     targetSession.ExpireAt.Add(-time.Duration(ttl) * time.Second).Format(time.RFC3339),
	}

	// Get version from processes
	if len(targetSession.Processes) > 0 {
		agentItem.Version = targetSession.Processes[0].Version
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

	// Build processes list
	processes := make([]types.AgentProcess, 0, len(targetSession.Processes))
	now := time.Now().Unix()
	for _, p := range targetSession.Processes {
		healthy := p.LastSeenUnix > 0 && now-p.LastSeenUnix <= 60
		processes = append(processes, types.AgentProcess{
			ServiceID:   p.ServiceID,
			Addr:        p.Addr,
			Version:     p.Version,
			FunctionIDs: p.FunctionIDs,
			LastSeen:    time.Unix(p.LastSeenUnix, 0).Format(time.RFC3339),
			Healthy:     healthy,
		})
	}

	return &types.AgentDetailResponse{
		AgentItem: agentItem,
		Functions: functions,
		Processes: processes,
	}, nil
}
