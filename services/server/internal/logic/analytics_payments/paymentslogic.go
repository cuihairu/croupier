// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_payments

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PaymentsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取支付分析
func NewPaymentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PaymentsLogic {
	return &PaymentsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PaymentsLogic) Payments(req *types.PaymentsRequest) (resp *types.PaymentsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
