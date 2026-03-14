// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package monitoring

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type MetricsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取系统指标
func NewMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MetricsLogic {
	return &MetricsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MetricsLogic) Metrics(req *types.MetricsRequest) (resp *types.MetricsResponse, err error) {
	dbStatus := checkDatabaseHealth(l.ctx, l.svcCtx)
	registryStatus, snapshots := collectRegistryStats(l.svcCtx.RegistryStore)
	opsStatus := summarizeOpsState(l.svcCtx)

	counts := map[string]interface{}{
		"agents_total":         registryStatus["agents_total"],
		"agents_healthy":       registryStatus["agents_healthy"],
		"functions_registered": registryStatus["functions_registered"],
		"maintenance_windows":  opsStatus["maintenance_windows"],
		"health_checks":        opsStatus["health_checks"],
		"alerts":               opsStatus["alerts"],
	}

	data := map[string]interface{}{
		"timestamp": utils.FormatTimestamp(time.Now()),
		"counts":    counts,
		"database": map[string]interface{}{
			"ok":         dbStatus["ok"],
			"latency_ms": dbStatus["latency_ms"],
			"driver":     dbStatus["driver"],
		},
		"registry": map[string]interface{}{
			"ok":       registryStatus["ok"],
			"agents":   snapshots,
			"metadata": registryStatus,
		},
		"ops": map[string]interface{}{
			"ok":            opsStatus["ok"],
			"mq_type":       opsStatus["mq_type"],
			"mq_lengths":    opsStatus["mq_lengths"],
			"health":        opsStatus["health_status"],
			"notifications": opsStatus["notifications"],
		},
	}

	return &types.MetricsResponse{
		Code:    0,
		Message: "OK",
		Data:    data,
	}, nil
}
