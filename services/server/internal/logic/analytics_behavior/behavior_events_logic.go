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

type BehaviorEventsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取行为事件
func NewBehaviorEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BehaviorEventsLogic {
	return &BehaviorEventsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BehaviorEventsLogic) BehaviorEvents(req *types.BehaviorEventsRequest) (*types.BehaviorEventsResponse, error) {
	if l.svcCtx.BehaviorModel == nil {
		return nil, errors.New("analytics model unavailable")
	}
	if req == nil {
		return nil, errors.New("缺少请求参数")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	start, end, err := utils.NormalizeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	opts := model.BehaviorEventOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     1,
			PageSize: limit,
		},
		GameID:    strings.TrimSpace(req.GameId),
		Env:       strings.TrimSpace(req.Env),
		EventType: strings.TrimSpace(req.EventType),
		StartTime: start,
		EndTime:   end,
	}

	events, total, err := l.svcCtx.BehaviorModel.ListEvents(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]types.BehaviorEvent, 0, len(events))
	for i := range events {
		ev := events[i]
		var payload interface{} = map[string]interface{}{}
		if ev.Data != nil {
			payload = map[string]interface{}(ev.Data)
		}
		items = append(items, types.BehaviorEvent{
			EventType: ev.EventType,
			UserId:    ev.UserID,
			Data:      payload,
			Timestamp: utils.FormatTimestamp(ev.OccurredAt),
		})
	}

	return &types.BehaviorEventsResponse{
		Items: items,
		Total: total,
	}, nil
}
