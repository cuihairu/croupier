// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AgentAnalyticsFiltersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取分析过滤器
func NewAgentAnalyticsFiltersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentAnalyticsFiltersLogic {
	return &AgentAnalyticsFiltersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentAnalyticsFiltersLogic) AgentAnalyticsFilters(req *types.AnalyticsFiltersQuery) (*types.AgentAnalyticsFiltersResponse, error) {
	path := utils.ResolveAnalyticsFiltersPath(l.svcCtx.Config)

	if lock := l.svcCtx.AnalyticsFiltersLock; lock != nil {
		lock.RLock()
		defer lock.RUnlock()
	}

	items, err := utils.LoadAnalyticsFilters(path)
	if err != nil {
		return nil, err
	}

	return &types.AgentAnalyticsFiltersResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items": items,
			"count": len(items),
		},
	}, nil
}
