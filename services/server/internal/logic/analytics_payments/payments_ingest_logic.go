// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_payments

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PaymentsIngestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 采集支付数据
func NewPaymentsIngestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PaymentsIngestLogic {
	return &PaymentsIngestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PaymentsIngestLogic) PaymentsIngest(req *types.PaymentsIngestRequest) (resp *types.PaymentsIngestResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
