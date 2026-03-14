// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_payments

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type PaymentsProductTrendLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取产品趋势
func NewPaymentsProductTrendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PaymentsProductTrendLogic {
	return &PaymentsProductTrendLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PaymentsProductTrendLogic) PaymentsProductTrend(req *types.PaymentsProductTrendRequest) (*types.PaymentsProductTrendResponse, error) {
	if l.svcCtx.PaymentsModel == nil {
		return nil, errors.New("payments model unavailable")
	}
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	start, end, err := utils.NormalizeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	items, err := l.svcCtx.PaymentsModel.ListProductTrends(l.ctx, strings.TrimSpace(req.GameId), strings.TrimSpace(req.Env))
	if err != nil {
		return nil, err
	}

	respItems := make([]types.ProductTrend, 0, len(items))
	for _, item := range items {
		if !start.IsZero() && item.WindowEnd.Before(start) {
			continue
		}
		if !end.IsZero() && item.WindowStart.After(end) {
			continue
		}
		respItems = append(respItems, types.ProductTrend{
			ProductId:   item.ProductID,
			ProductName: item.ProductName,
			Revenue:     item.Revenue,
			Sales:       item.Sales,
			Growth:      item.Growth,
		})
		if len(respItems) >= limit {
			break
		}
	}

	return &types.PaymentsProductTrendResponse{
		Items: respItems,
	}, nil
}
