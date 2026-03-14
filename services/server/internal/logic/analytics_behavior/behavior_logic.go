// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_behavior

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type BehaviorLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取行为分析
func NewBehaviorLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BehaviorLogic {
	return &BehaviorLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BehaviorLogic) Behavior(req *types.BehaviorRequest) (*types.BehaviorResponse, error) {
	if l.svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior analytics unavailable")
	}
	if req == nil {
		return nil, errors.New("缺少请求参数")
	}

	start, end, err := utils.NormalizeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	opts := model.BehaviorEventOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     1,
			PageSize: 3000,
		},
		GameID:    strings.TrimSpace(req.GameId),
		Env:       strings.TrimSpace(req.Env),
		StartTime: start,
		EndTime:   end,
	}

	events, _, err := l.svcCtx.BehaviorModel.ListEvents(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	segments := breakdownBySegment(events)
	heatmap := breakdownByTime(events, start, end)

	return &types.BehaviorResponse{
		TopActions: topMapPairs(segments.Actions, 20),
		UserFlows: map[string]interface{}{
			"regions":   topMapPairs(segments.Regions, 10),
			"platforms": topMapPairs(segments.Platforms, 10),
		},
		HeatMap: map[string]interface{}{
			"points": heatmap,
		},
	}, nil
}
