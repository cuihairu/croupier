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

type BehaviorAdoptionBreakdownLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取采用率明细
func NewBehaviorAdoptionBreakdownLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BehaviorAdoptionBreakdownLogic {
	return &BehaviorAdoptionBreakdownLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BehaviorAdoptionBreakdownLogic) BehaviorAdoptionBreakdown(req *types.BehaviorAdoptionBreakdownRequest) (resp *types.BehaviorAdoptionBreakdownResponse, err error) {
	if l.svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior analytics unavailable")
	}
	if req == nil {
		return nil, errors.New("缺少请求参数")
	}
	if strings.TrimSpace(req.Feature) == "" {
		return nil, errors.New("feature 参数不能为空")
	}

	gameID := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)
	feature := strings.TrimSpace(req.Feature)

	start, end, err := utils.NormalizeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	opts := model.BehaviorEventOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     1,
			PageSize: 5000,
		},
		GameID:    gameID,
		Env:       env,
		EventType: feature,
		StartTime: start,
		EndTime:   end,
	}

	events, _, err := l.svcCtx.BehaviorModel.ListEvents(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	segment := breakdownBySegment(events)
	series := breakdownByTime(events, start, end)

	return &types.BehaviorAdoptionBreakdownResponse{
		BySegment: map[string]interface{}{
			"totalUsers": segment.TotalUsers,
			"regions":    segment.Regions,
			"platforms":  segment.Platforms,
			"roles":      segment.Roles,
		},
		ByTime: map[string]interface{}{
			"intervals": series,
			"range": map[string]string{
				"start": utils.FormatTimestamp(start),
				"end":   utils.FormatTimestamp(end),
			},
		},
	}, nil
}
