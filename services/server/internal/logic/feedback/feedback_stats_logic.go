// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package feedback

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type FeedbackStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取反馈统计
func NewFeedbackStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FeedbackStatsLogic {
	return &FeedbackStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FeedbackStatsLogic) FeedbackStats(req *types.FeedbackStatsRequest) (resp *types.FeedbackStatsResponse, err error) {
	if l.svcCtx.FeedbackModel == nil {
		return nil, errors.New("反馈模型未初始化")
	}
	if req == nil {
		req = &types.FeedbackStatsRequest{}
	}

	days := req.Days
	if days <= 0 {
		days = 7
	}

	stats, err := l.svcCtx.FeedbackModel.Stats(l.ctx, model.FeedbackStatsOptions{
		GameID: strings.TrimSpace(req.GameId),
		Days:   days,
	})
	if err != nil {
		return nil, err
	}

	byCategory := make(map[string]int, len(stats.ByCategory))
	for k, v := range stats.ByCategory {
		byCategory[k] = int(v)
	}
	byStatus := make(map[string]int, len(stats.ByStatus))
	for k, v := range stats.ByStatus {
		byStatus[k] = int(v)
	}

	response := types.FeedbackStatsResponse{
		FeedbackStats: types.FeedbackStats{
			Total:        int(stats.Total),
			ByCategory:   byCategory,
			ByStatus:     byStatus,
			AvgRating:    stats.AvgRating,
			ResponseRate: 0,
		},
	}
	if stats.Total > 0 {
		response.FeedbackStats.ResponseRate = float64(stats.Responded) / float64(stats.Total)
	}

	return &response, nil
}
