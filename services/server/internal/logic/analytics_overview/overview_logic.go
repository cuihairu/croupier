// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_overview

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OverviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取分析概览
func NewOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OverviewLogic {
	return &OverviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OverviewLogic) Overview(req *types.OverviewRequest) (*types.OverviewResponse, error) {
	if l.svcCtx.BehaviorModel == nil || l.svcCtx.PaymentsModel == nil || l.svcCtx.PlayerModel == nil {
		return nil, errors.New("analytics models unavailable")
	}
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	start, end, err := resolveRange(req.StartDate, req.EndDate, defaultOverviewWindowDays)
	if err != nil {
		return nil, err
	}
	gameID := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)

	dauStart := end.Add(-24 * time.Hour)
	mauStart := end.Add(-30 * 24 * time.Hour)

	var (
		dau, mau, activeUsers, newUsers int64
	)

	if dau, err = l.svcCtx.BehaviorModel.CountDistinctUsers(l.ctx, gameID, env, dauStart, end); err != nil {
		return nil, err
	}
	if mau, err = l.svcCtx.BehaviorModel.CountDistinctUsers(l.ctx, gameID, env, mauStart, end); err != nil {
		return nil, err
	}
	if activeUsers, err = l.svcCtx.BehaviorModel.CountDistinctUsers(l.ctx, gameID, env, start, end); err != nil {
		return nil, err
	}
	if newUsers, err = l.svcCtx.PlayerModel.CountNewPlayers(l.ctx, gameID, start, end); err != nil {
		return nil, err
	}

	revenueAgg, err := l.svcCtx.PaymentsModel.AggregateRevenue(l.ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}

	arpu := safeDivide(revenueAgg.Revenue, float64(activeUsers))
	arppu := safeDivide(revenueAgg.Revenue, float64(revenueAgg.Payers))
	payingRate := safeDivide(float64(revenueAgg.Payers), float64(activeUsers))

	activityStats, err := l.svcCtx.BehaviorModel.DailyActivity(l.ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}
	revenueStats, err := l.svcCtx.PaymentsModel.DailyRevenue(l.ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}
	newPlayerStats, err := l.svcCtx.PlayerModel.DailyNewPlayers(l.ctx, gameID, start, end)
	if err != nil {
		return nil, err
	}

	trends := map[string]interface{}{
		"activeUsers": summarizeBehaviorStats(activityStats, func(stat model.BehaviorDailyStat) int64 { return stat.ActiveUsers }),
		"events":      summarizeBehaviorStats(activityStats, func(stat model.BehaviorDailyStat) int64 { return stat.Events }),
		"newUsers":    summarizePlayerStats(newPlayerStats),
		"revenue":     summarizeRevenueStats(revenueStats),
	}

	return &types.OverviewResponse{
		Metrics: types.OverviewMetrics{
			DAU:        int(dau),
			MAU:        int(mau),
			NewUsers:   int(newUsers),
			Revenue:    revenueAgg.Revenue,
			ARPU:       arpu,
			ARPPU:      arppu,
			PayingRate: payingRate,
		},
		Trends: trends,
	}, nil
}
