// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
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

func (l *OpsMetricsLogic) OpsMetrics(req *types.OpsMetricsQuery) (resp *types.OpsMetricsResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsMetrics not implemented")
}
