
package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
	
)

type OpsMetricsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取指标
func NewOpsMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsMetricsLogic {
	return &OpsMetricsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsMetricsLogic) OpsMetrics(req *OpsMetricsQuery) (resp *OpsMetricsResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsMetrics not implemented")
}
