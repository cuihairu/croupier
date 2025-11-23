// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsPaymentsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取支付分析
func NewAnalyticsPaymentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsPaymentsLogic {
	return &AnalyticsPaymentsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsPaymentsLogic) AnalyticsPayments(req *types.AnalyticsPaymentsRequest) (resp *types.AnalyticsPaymentsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
