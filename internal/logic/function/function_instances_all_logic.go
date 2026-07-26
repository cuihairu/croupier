package function

import (
	"context"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionInstancesAllLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取所有函数实例
func NewFunctionInstancesAllLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionInstancesAllLogic {
	return &FunctionInstancesAllLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionInstancesAllLogic) FunctionInstancesAll() (*FunctionInstancesAllResponse, error) {
	if store := l.svcCtx.RegistryStore; store != nil {
		out := make([]RuntimeFunctionInstance, 0)
		store.Mu().RLock()
		for _, sess := range store.AgentsUnsafe() {
			if sess == nil || strings.TrimSpace(sess.AgentID) == "" {
				continue
			}
			agentHealthy := time.Until(sess.ExpireAt) > 0
			agentLastSeen := sess.LastSeen
			if agentLastSeen.IsZero() && !sess.ExpireAt.IsZero() {
				agentLastSeen = sess.ExpireAt.Add(-30 * time.Second)
			}
			for _, p := range sess.Providers {
				if len(p.FunctionIDs) == 0 {
					continue
				}
				for _, fid := range p.FunctionIDs {
					fid = strings.TrimSpace(fid)
					if fid == "" {
						continue
					}
					out = append(out, RuntimeFunctionInstance{
						FunctionID: fid,
						AgentID:    sess.AgentID,
						ProviderID: p.ProviderID,
						Addr:       p.Addr,
						Version:    p.Version,
						LastSeen:   agentLastSeen.Format(time.RFC3339),
						Healthy:    agentHealthy,
						GameID:     sess.GameID,
						Env:        sess.Env,
					})
				}
			}
		}
		store.Mu().RUnlock()
		return &FunctionInstancesAllResponse{Instances: out}, nil
	}

	return &FunctionInstancesAllResponse{Instances: []RuntimeFunctionInstance{}}, nil
}
