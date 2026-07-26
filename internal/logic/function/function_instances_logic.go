package function

import (
	"context"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionInstancesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数实例
func NewFunctionInstancesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionInstancesLogic {
	return &FunctionInstancesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionInstancesLogic) FunctionInstances(req *FunctionInstancesRequest) (*FunctionInstancesResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	// Prefer runtime registry (SDK->Agent registrations) to power dashboard targeted routing.
	// Fallback to DB-backed instances if registry is not available.
	if store := l.svcCtx.RegistryStore; store != nil {
		out := make([]RuntimeFunctionInstance, 0)
		store.Mu().RLock()
		for _, sess := range store.AgentsUnsafe() {
			if sess == nil || strings.TrimSpace(sess.AgentID) == "" {
				continue
			}
			// Agent session TTL drives liveness; provider last_seen may not be updated on heartbeat.
			agentHealthy := time.Until(sess.ExpireAt) > 0
			agentLastSeen := sess.LastSeen
			if agentLastSeen.IsZero() && !sess.ExpireAt.IsZero() {
				agentLastSeen = sess.ExpireAt.Add(-30 * time.Second)
			}
			for _, p := range sess.Providers {
				has := false
				for _, fid := range p.FunctionIDs {
					if fid == functionID {
						has = true
						break
					}
				}
				if !has {
					continue
				}
				out = append(out, RuntimeFunctionInstance{
					FunctionID: functionID,
					AgentID:    sess.AgentID,
					ProviderID: p.ProviderID,
					Addr:       p.Addr,
					Version:    p.Version,
					LastSeen:   agentLastSeen.Format(time.RFC3339),
					Healthy:    agentHealthy,
				})
			}
		}
		store.Mu().RUnlock()
		return &FunctionInstancesResponse{Instances: out}, nil
	}

	instances, err := l.svcCtx.FunctionModel.ListInstances(l.ctx, functionID)
	if err != nil {
		return nil, err
	}
	items := utils.BuildFunctionInstances(instances)
	out := make([]FunctionInstance, 0, len(items))
	for _, item := range items {
		out = append(out, FunctionInstance{
			AgentId:   item.AgentId,
			AgentName: item.AgentName,
			Status:    item.Status,
			UpdatedAt: item.UpdatedAt,
		})
	}
	return &FunctionInstancesResponse{Items: out}, nil
}
