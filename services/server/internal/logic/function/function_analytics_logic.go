package function

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionAnalyticsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionAnalyticsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionAnalyticsLogic {
	return &FunctionAnalyticsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionAnalyticsLogic) FunctionAnalytics(req *types.FunctionAnalyticsRequest) (*types.FunctionAnalyticsResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}
	if _, err := getOrCreateFunctionRecord(l.ctx, l.svcCtx, functionID); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	dayStart := now.Add(-24 * time.Hour)
	weekStart := now.Add(-7 * 24 * time.Hour)
	monthStart := now.Add(-30 * 24 * time.Hour)

	var total, today, week, month int64
	if l.svcCtx.ConfigVersionModel != nil {
		countInRange := func(key string, from time.Time) (int64, error) {
			versions, listErr := l.svcCtx.ConfigVersionModel.List(l.ctx, key)
			if listErr != nil {
				return 0, listErr
			}
			var count int64
			for _, v := range versions {
				if v.CreatedAt.UTC().After(from) {
					count++
				}
			}
			return count, nil
		}

		keys := []string{functionUIHistoryKey(functionID), functionRouteHistoryKey(functionID)}
		for _, key := range keys {
			versions, listErr := l.svcCtx.ConfigVersionModel.List(l.ctx, key)
			if listErr != nil {
				return nil, listErr
			}
			total += int64(len(versions))

			cDay, listErr := countInRange(key, dayStart)
			if listErr != nil {
				return nil, listErr
			}
			today += cDay
			cWeek, listErr := countInRange(key, weekStart)
			if listErr != nil {
				return nil, listErr
			}
			week += cWeek
			cMonth, listErr := countInRange(key, monthStart)
			if listErr != nil {
				return nil, listErr
			}
			month += cMonth
		}
	}

	return &types.FunctionAnalyticsResponse{
		TotalCalls:     total,
		SuccessRate:    100,
		AvgLatency:     0,
		CallsToday:     today,
		CallsThisWeek:  week,
		CallsThisMonth: month,
	}, nil
}
