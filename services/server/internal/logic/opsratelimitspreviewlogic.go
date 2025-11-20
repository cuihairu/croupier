package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsRateLimitsPreviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsRateLimitsPreviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsRateLimitsPreviewLogic {
	return &OpsRateLimitsPreviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsRateLimitsPreviewLogic) OpsRateLimitsPreview(req *types.RateLimitPreviewQuery) (*types.RateLimitPreviewResponse, error) {
	if req == nil {
		req = &types.RateLimitPreviewQuery{}
	}
	if strings.ToLower(strings.TrimSpace(req.Scope)) != "service" {
		return &types.RateLimitPreviewResponse{
			Matched: 0,
			Agents:  []types.RateLimitPreviewAgent{},
		}, nil
	}
	limit := req.LimitQPS
	if limit <= 0 {
		limit = 0
	}
	percent := req.Percent
	if percent <= 0 {
		percent = 100
	}
	registry := l.svcCtx.RegistryStore
	if registry == nil {
		return &types.RateLimitPreviewResponse{Matched: 0, Agents: []types.RateLimitPreviewAgent{}}, nil
	}
	registry.Mu().RLock()
	defer registry.Mu().RUnlock()
	out := []types.RateLimitPreviewAgent{}
	for _, agent := range registry.AgentsUnsafe() {
		if agent == nil {
			continue
		}
		if req.Key != "" && agent.AgentID != req.Key {
			continue
		}
		if req.MatchGameId != "" && agent.GameID != req.MatchGameId {
			continue
		}
		if req.MatchEnv != "" && agent.Env != req.MatchEnv {
			continue
		}
		if req.MatchRegion != "" && agent.Region != req.MatchRegion {
			continue
		}
		if req.MatchZone != "" && agent.Zone != req.MatchZone {
			continue
		}
		eff := limit
		if percent > 0 && percent < 100 {
			eff = eff * percent / 100
		}
		if eff <= 0 {
			eff = limit
		}
		out = append(out, types.RateLimitPreviewAgent{
			AgentId: agent.AgentID,
			GameId:  agent.GameID,
			Env:     agent.Env,
			Region:  agent.Region,
			Zone:    agent.Zone,
			RpcAddr: agent.RPCAddr,
			Qps:     eff,
			Qps1m:   0,
		})
	}
	return &types.RateLimitPreviewResponse{
		Matched: len(out),
		Agents:  out,
	}, nil
}
