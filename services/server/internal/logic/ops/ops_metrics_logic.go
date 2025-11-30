// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"
	"math"
	"time"

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

func (l *OpsMetricsLogic) OpsMetrics(req *types.OpsMetricsQuery) (*types.OpsMetricsResponse, error) {
	now := time.Now().UTC()
	qps := buildMetricSeries(now, 60, 15*time.Second, func(i int) float64 {
		return 10 + 3*math.Sin(float64(i)/6.0)
	})
	errRate := buildMetricSeries(now, 60, 15*time.Second, func(i int) float64 {
		return 0.01 + 0.005*math.Sin(float64(i)/8.0+1)
	})
	p95 := buildMetricSeries(now, 60, 15*time.Second, func(i int) float64 {
		return 80 + 30*math.Abs(math.Sin(float64(i)/10.0))
	})

	return &types.OpsMetricsResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"qps":      qps,
			"err_rate": errRate,
			"p95_ms":   p95,
		},
	}, nil
}

func buildMetricSeries(base time.Time, samples int, step time.Duration, fn func(idx int) float64) [][]interface{} {
	if samples <= 0 {
		return nil
	}
	points := make([][]interface{}, 0, samples)
	for i := samples - 1; i >= 0; i-- {
		ts := base.Add(-time.Duration(samples-1-i) * step).Unix()
		val := fn(i)
		points = append(points, []interface{}{float64(ts), val})
	}
	return points
}
