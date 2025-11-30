// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_overview

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FiltersGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取分析过滤器
func NewFiltersGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FiltersGetLogic {
	return &FiltersGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FiltersGetLogic) FiltersGet(req *types.FiltersGetRequest) (*types.FiltersGetResponse, error) {
	gameID := ""
	if req != nil {
		gameID = strings.TrimSpace(req.GameId)
	}

	path := utils.ResolveAnalyticsFiltersPath(l.svcCtx.Config)

	var (
		items []types.AnalyticsFilters
		err   error
	)

	if lock := l.svcCtx.AnalyticsFiltersLock; lock != nil {
		lock.RLock()
		defer lock.RUnlock()
		items, err = utils.LoadAnalyticsFilters(path)
	} else {
		items, err = utils.LoadAnalyticsFilters(path)
	}
	if err != nil {
		return nil, err
	}

	filtered := filterAnalyticsFilters(items, gameID)

	return &types.FiltersGetResponse{
		Items: filtered,
	}, nil
}

func filterAnalyticsFilters(items []types.AnalyticsFilters, gameID string) []types.AnalyticsFilters {
	if gameID == "" {
		return items
	}
	filtered := make([]types.AnalyticsFilters, 0, 1)
	for _, item := range items {
		if strings.EqualFold(item.GameId, gameID) {
			filtered = append(filtered, item)
			break
		}
	}
	return filtered
}
