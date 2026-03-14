// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_overview

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type RealtimeSeriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取实时序列数据
func NewRealtimeSeriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RealtimeSeriesLogic {
	return &RealtimeSeriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RealtimeSeriesLogic) RealtimeSeries(req *types.RealtimeSeriesRequest) (*types.RealtimeSeriesResponse, error) {
	if l.svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior analytics unavailable")
	}

	gameID := ""
	env := ""
	if req != nil {
		gameID = strings.TrimSpace(req.GameId)
		env = strings.TrimSpace(req.Env)
	}

	interval := resolveRealtimeInterval("")
	window := clampRealtimeDuration(60)
	if req != nil {
		interval = resolveRealtimeInterval(req.Interval)
		window = clampRealtimeDuration(req.Duration)
	}

	end := time.Now().UTC()
	start := end.Add(-window)

	var eventsSeries, usersSeries []bucketPoint

	for cursor := start; cursor.Before(end); cursor = cursor.Add(interval) {
		bucketEnd := cursor.Add(interval)
		if bucketEnd.After(end) {
			bucketEnd = end
		}

		eventsCount, err := l.svcCtx.BehaviorModel.CountEvents(l.ctx, gameID, env, cursor, bucketEnd)
		if err != nil {
			return nil, err
		}
		usersCount, err := l.svcCtx.BehaviorModel.CountDistinctUsers(l.ctx, gameID, env, cursor, bucketEnd)
		if err != nil {
			return nil, err
		}

		eventsSeries = append(eventsSeries, bucketPoint{
			Timestamp: bucketEnd,
			Value:     eventsCount,
		})
		usersSeries = append(usersSeries, bucketPoint{
			Timestamp: bucketEnd,
			Value:     usersCount,
		})
	}

	series := map[string]interface{}{
		"events": buildRealtimeSeriesPoints(eventsSeries),
		"users":  buildRealtimeSeriesPoints(usersSeries),
	}

	return &types.RealtimeSeriesResponse{
		Series: series,
	}, nil
}
