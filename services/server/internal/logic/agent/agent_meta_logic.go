// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"
	"errors"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type AgentMetaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 上报代理元数据
func NewAgentMetaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentMetaLogic {
	return &AgentMetaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentMetaLogic) AgentMeta(req *types.AgentMetaReportRequest) (*types.AgentMetaResponse, error) {
	store := l.svcCtx.RegistryStore
	if store == nil {
		return nil, errors.New("registry store unavailable")
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	agents := make([]map[string]interface{}, 0, len(store.AgentsUnsafe()))
	for _, sess := range store.AgentsUnsafe() {
		if snapshot := utils.BuildOpsAgentSnapshot(sess); snapshot != nil {
			agents = append(agents, snapshot)
		}
	}

	data := map[string]interface{}{
		"agents":    agents,
		"count":     len(agents),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return &types.AgentMetaResponse{
		Code:    0,
		Message: "OK",
		Data:    data,
	}, nil
}
