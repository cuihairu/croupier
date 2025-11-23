// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsPaymentsIngestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 采集支付数据
func NewAnalyticsPaymentsIngestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsPaymentsIngestLogic {
	return &AnalyticsPaymentsIngestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsPaymentsIngestLogic) AnalyticsPaymentsIngest(req *types.AnalyticsPaymentsIngestRequest) (resp *types.AnalyticsPaymentsIngestResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
