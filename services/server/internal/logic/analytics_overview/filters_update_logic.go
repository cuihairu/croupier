// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_overview

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type FiltersUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新分析过滤器
func NewFiltersUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FiltersUpdateLogic {
	return &FiltersUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FiltersUpdateLogic) FiltersUpdate(req *types.FiltersUpdateRequest) (*types.FiltersGetResponse, error) {
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	gameID := strings.TrimSpace(req.GameId)
	if gameID == "" {
		return nil, errors.New("gameId 不能为空")
	}
	if req.Filters == nil {
		return nil, errors.New("filters 不能为空")
	}

	path := utils.ResolveAnalyticsFiltersPath(l.svcCtx.Config)

	if lock := l.svcCtx.AnalyticsFiltersLock; lock != nil {
		lock.Lock()
		defer lock.Unlock()
	}

	items, err := utils.LoadAnalyticsFilters(path)
	if err != nil {
		return nil, err
	}

	items = upsertAnalyticsFilter(items, gameID, req.Filters)
	if err := utils.SaveAnalyticsFilters(path, items); err != nil {
		return nil, err
	}

	return &types.FiltersGetResponse{
		Items: filterAnalyticsFilters(items, gameID),
	}, nil
}

func upsertAnalyticsFilter(items []types.AnalyticsFilters, gameID string, filters interface{}) []types.AnalyticsFilters {
	replaced := false
	for i := range items {
		if strings.EqualFold(items[i].GameId, gameID) {
			items[i].Filters = filters
			replaced = true
			break
		}
	}
	if !replaced {
		items = append(items, types.AnalyticsFilters{
			GameId:  gameID,
			Filters: filters,
		})
	}
	return items
}
