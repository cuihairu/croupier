// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package monitoring

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type HealthzLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 健康检查
func NewHealthzLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HealthzLogic {
	return &HealthzLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *HealthzLogic) Healthz(req *types.HealthzRequest) (resp *types.HealthzResponse, err error) {
	dbStatus := checkDatabaseHealth(l.ctx, l.svcCtx)
	registryStatus, _ := collectRegistryStats(l.svcCtx.RegistryStore)
	opsStatus := summarizeOpsState(l.svcCtx)

	ok := componentHealthy(dbStatus) && componentHealthy(registryStatus) && componentHealthy(opsStatus)
	message := "OK"
	if !ok {
		message = "DEGRADED"
	}

	data := map[string]interface{}{
		"ok":             ok,
		"timestamp":      utils.FormatTimestamp(time.Now()),
		"uptime_seconds": uptimeSeconds(),
		"components": map[string]interface{}{
			"database": dbStatus,
			"registry": registryStatus,
			"ops":      opsStatus,
		},
	}

	return &types.HealthzResponse{
		Code:    0,
		Message: message,
		Data:    data,
	}, nil
}
