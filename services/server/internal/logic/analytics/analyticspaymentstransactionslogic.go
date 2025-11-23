// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsPaymentsTransactionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取支付交易列表
func NewAnalyticsPaymentsTransactionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsPaymentsTransactionsLogic {
	return &AnalyticsPaymentsTransactionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsPaymentsTransactionsLogic) AnalyticsPaymentsTransactions(req *types.AnalyticsPaymentsTransactionsRequest) (resp *types.AnalyticsPaymentsTransactionsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
