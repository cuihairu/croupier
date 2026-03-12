// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_payments

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PaymentsSummaryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取支付摘要
func NewPaymentsSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PaymentsSummaryLogic {
	return &PaymentsSummaryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PaymentsSummaryLogic) PaymentsSummary(req *types.PaymentsSummaryRequest) (resp *types.PaymentsSummaryResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
