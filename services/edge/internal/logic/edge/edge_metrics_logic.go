// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package edge

import (
	"context"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EdgeMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEdgeMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EdgeMetricsLogic {
	return &EdgeMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EdgeMetricsLogic) EdgeMetrics(req *types.EdgeMetricsRequest) (resp *types.EdgeMetricsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
