package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AgentAnalyticsFiltersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAgentAnalyticsFiltersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentAnalyticsFiltersLogic {
	return &AgentAnalyticsFiltersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentAnalyticsFiltersLogic) AgentAnalyticsFilters(req *types.AnalyticsFiltersQuery) (*types.AnalyticsFiltersResponse, error) {
	if req == nil {
		req = &types.AnalyticsFiltersQuery{}
	}
	filter := l.svcCtx.AnalyticsFilter(req.GameId, req.Env)
	return &types.AnalyticsFiltersResponse{
		Events:          append([]string{}, filter.Events...),
		PaymentsEnabled: filter.PaymentsEnabled,
		SampleGlobal:    filter.SampleGlobal,
	}, nil
}
