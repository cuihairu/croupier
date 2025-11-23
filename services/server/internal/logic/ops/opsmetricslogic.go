// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取指标
func NewOpsMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsMetricsLogic {
	return &OpsMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsMetricsLogic) OpsMetrics(req *types.OpsMetricsQuery) (resp *types.OpsMetricsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
