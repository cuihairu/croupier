package logic

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
	// TODO: implement the EdgeMetrics logic here
	resp = &types.EdgeMetricsResponse{
		Metrics: map[string]interface{}{
			"cpu_usage":    15.5,
			"memory_usage": 45.2,
			"network_in":   1024000,
			"network_out":  2048000,
			"active_tunnels": 5,
		},
	}
	return
}