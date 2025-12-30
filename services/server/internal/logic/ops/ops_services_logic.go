// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsServicesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取服务列表
func NewOpsServicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsServicesLogic {
	return &OpsServicesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsServicesLogic) OpsServices(req *types.OpsServicesRequest) (*types.OpsServicesResponse, error) {
	services := make([]map[string]interface{}, 0)
	if store := l.svcCtx.RegistryStore; store != nil {
		store.Mu().RLock()
		for _, sess := range store.AgentsUnsafe() {
			if snapshot := utils.BuildOpsAgentSnapshot(sess); snapshot != nil {
				status := "expired"
				if healthy, _ := snapshot["healthy"].(bool); healthy {
					status = "healthy"
				}
				services = append(services, map[string]interface{}{
					"id":             snapshot["agent_id"],
					"name":           snapshot["agent_id"],
					"type":           "agent",
					"status":         status,
					"address":        snapshot["rpc_addr"],
					"gameId":         snapshot["game_id"],
					"env":            snapshot["env"],
					"version":        snapshot["version"],
					"region":         snapshot["region"],
					"zone":           snapshot["zone"],
					"labels":         snapshot["labels"],
					"functionsCount": snapshot["functions"],
					"lastSeen":       snapshot["last_seen"],
					"metadata": map[string]interface{}{
						"processes":      snapshot["processes"],
						"processesCount": snapshot["processes_count"],
					},
				})
			}
		}
		store.Mu().RUnlock()
	}

	return &types.OpsServicesResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"services": services,
			"total":    len(services),
		},
	}, nil
}
