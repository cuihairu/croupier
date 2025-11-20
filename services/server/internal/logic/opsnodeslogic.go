package logic

import (
	"context"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNodesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsNodesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodesLogic {
	return &OpsNodesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodesLogic) OpsNodes() (*types.OpsNodesResponse, error) {
	resp := &types.OpsNodesResponse{
		Nodes: []types.OpsNode{},
	}
	store := l.svcCtx.RegistryStore
	if store != nil {
		now := time.Now()
		store.Mu().RLock()
		for _, agent := range store.AgentsUnsafe() {
			if agent == nil {
				continue
			}
			expSec := int(time.Until(agent.ExpireAt).Seconds())
			if expSec < 0 {
				expSec = 0
			}
			resp.Nodes = append(resp.Nodes, types.OpsNode{
				Id:           agent.AgentID,
				Type:         "agent",
				GameId:       agent.GameID,
				Env:          agent.Env,
				Addr:         agent.RPCAddr,
				Ip:           hostFromAddr(agent.RPCAddr),
				Version:      agent.Version,
				Region:       agent.Region,
				Zone:         agent.Zone,
				Healthy:      now.Before(agent.ExpireAt),
				ExpiresInSec: expSec,
				Draining:     l.svcCtx.NodeDraining(agent.AgentID),
				Labels:       agent.Labels,
			})
		}
		store.Mu().RUnlock()
	}
	for _, edge := range l.svcCtx.EdgeNodesSnapshot() {
		lastSeen := formatRFC3339(edge.LastSeen)
		healthy := time.Since(edge.LastSeen) <= 90*time.Second
		resp.Nodes = append(resp.Nodes, types.OpsNode{
			Id:       edge.ID,
			Type:     "edge",
			Addr:     edge.Addr,
			HttpAddr: edge.HTTPAddr,
			Ip:       fallback(edge.IP, hostFromAddr(edge.Addr)),
			Version:  edge.Version,
			Region:   edge.Region,
			Zone:     edge.Zone,
			Healthy:  healthy,
			LastSeen: lastSeen,
			Draining: l.svcCtx.NodeDraining(edge.ID),
		})
	}
	return resp, nil
}

func fallback(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
