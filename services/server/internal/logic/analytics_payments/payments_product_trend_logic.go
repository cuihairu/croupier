// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_payments

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PaymentsProductTrendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取产品趋势
func NewPaymentsProductTrendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PaymentsProductTrendLogic {
	return &PaymentsProductTrendLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PaymentsProductTrendLogic) PaymentsProductTrend(req *types.PaymentsProductTrendRequest) (resp *types.PaymentsProductTrendResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
