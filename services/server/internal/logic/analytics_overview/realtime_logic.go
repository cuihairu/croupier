// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_overview

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type RealtimeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取实时数据
func NewRealtimeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RealtimeLogic {
	return &RealtimeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RealtimeLogic) Realtime(req *types.RealtimeRequest) (*types.RealtimeResponse, error) {
	if l.svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior analytics unavailable")
	}

	gameID := ""
	env := ""
	if req != nil {
		gameID = strings.TrimSpace(req.GameId)
		env = strings.TrimSpace(req.Env)
	}

	end := time.Now().UTC()
	start := end.Add(-defaultRealtimeWindow)

	activeUsers, err := l.svcCtx.BehaviorModel.CountDistinctUsers(l.ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}
	eventsCount, err := l.svcCtx.BehaviorModel.CountEvents(l.ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}
	eventTypeCounts, err := l.svcCtx.BehaviorModel.EventTypeCounts(l.ctx, gameID, env, start, end, 5)
	if err != nil {
		return nil, err
	}

	qps := safeDivide(float64(eventsCount), defaultRealtimeWindow.Seconds())
	avgLatency, errorRate := aggregateAgentMetrics(l.svcCtx.RegistryStore, gameID, env)

	return &types.RealtimeResponse{
		RealtimeMetrics: types.RealtimeMetrics{
			OnlineUsers:    int(activeUsers),
			ActiveSessions: int(eventsCount),
			QPS:            qps,
			AvgLatency:     avgLatency,
			ErrorRate:      errorRate,
			TopEvents:      mapTopEvents(eventTypeCounts),
		},
		Timestamp: utils.FormatTimestamp(end),
	}, nil
}
