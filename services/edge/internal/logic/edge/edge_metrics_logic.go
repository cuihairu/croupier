// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package edge

import (
	"context"
	"strings"

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

func (l *EdgeMetricsLogic) EdgeMetrics(req *types.EdgeMetricsRequest) (*types.EdgeMetricsResponse, error) {
	state := l.svcCtx.State
	state.Mu.RLock()
	defer state.Mu.RUnlock()

	totalTunnels := len(state.Tunnels)
	active := 0
	bytesIn := int64(0)
	bytesOut := int64(0)
	for _, t := range state.Tunnels {
		bytesIn += t.BytesIn
		bytesOut += t.BytesOut
		if strings.EqualFold(t.Status, "active") {
			active++
		}
	}

	metrics := map[string]interface{}{
		"total_tunnels":  totalTunnels,
		"active_tunnels": active,
		"bytes_in":       bytesIn,
		"bytes_out":      bytesOut,
		"uptime_sec":     l.svcCtx.Uptime().Seconds(),
	}

	return &types.EdgeMetricsResponse{
		Metrics: metrics,
	}, nil
}
