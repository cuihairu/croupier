// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsAgentsListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Agent 列表
func NewOpsAgentsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentsListLogic {
	return &OpsAgentsListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentsListLogic) OpsAgentsList(req *types.OpsAgentsListRequest) (*types.OpsAgentsListResponse, error) {
	agents := make([]types.OpsAgentInfo, 0)

	store := l.svcCtx.RegistryStore
	if store == nil {
		return &types.OpsAgentsListResponse{
			Code:    0,
			Message: "OK",
			Data:    agents,
		}, nil
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	now := time.Now()
	for agentID, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}

		// Check if agent is connected (session not expired)
		connected := sess.ExpireAt.After(now)

		// Collect function IDs
		functions := make([]string, 0, len(sess.Functions))
		for fid := range sess.Functions {
			functions = append(functions, fid)
		}

		// Collect provider IDs
		providers := make([]string, 0, len(sess.Providers))
		for _, p := range sess.Providers {
			providers = append(providers, p.ProviderID)
		}

		agents = append(agents, types.OpsAgentInfo{
			AgentID:   agentID,
			GameID:    sess.GameID,
			Env:       sess.Env,
			Version:   sess.Version,
			RPCAddr:   sess.RPCAddr,
			Connected: connected,
			LastSeen:  sess.ExpireAt.Format(time.RFC3339),
			Functions: functions,
			Processes: providers,
			Labels:    sess.Labels,
		})
	}

	return &types.OpsAgentsListResponse{
		Code:    0,
		Message: "OK",
		Data:    agents,
	}, nil
}
