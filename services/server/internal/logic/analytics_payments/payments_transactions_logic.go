// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_payments

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type PaymentsTransactionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取支付交易列表
func NewPaymentsTransactionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PaymentsTransactionsLogic {
	return &PaymentsTransactionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PaymentsTransactionsLogic) PaymentsTransactions(req *types.PaymentsTransactionsRequest) (*types.PaymentsTransactionsResponse, error) {
	if l.svcCtx.PaymentsModel == nil {
		return nil, errors.New("payments model unavailable")
	}
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	size := req.PageSize
	if size <= 0 {
		size = 20
	}

	start, end, err := utils.NormalizeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	opts := model.PaymentQueryOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     page,
			PageSize: size,
		},
		GameID:    strings.TrimSpace(req.GameId),
		Env:       strings.TrimSpace(req.Env),
		Status:    strings.TrimSpace(req.Status),
		StartTime: start,
		EndTime:   end,
	}

	items, total, err := l.svcCtx.PaymentsModel.ListTransactions(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	respItems := make([]types.PaymentTransaction, 0, len(items))
	for _, tx := range items {
		respItems = append(respItems, convertTransaction(tx))
	}

	return &types.PaymentsTransactionsResponse{
		Items: respItems,
		Total: total,
		Page:  page,
		Size:  size,
	}, nil
}
