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

type StatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取系统状态
func NewStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StatusLogic {
	return &StatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StatusLogic) Status(req *types.StatusRequest) (resp *types.StatusResponse, err error) {
	dbStatus := checkDatabaseHealth(l.ctx, l.svcCtx)
	registryStatus, snapshots := collectRegistryStats(l.svcCtx.RegistryStore)
	opsStatus := summarizeOpsState(l.svcCtx)

	ok := componentHealthy(dbStatus) && componentHealthy(registryStatus) && componentHealthy(opsStatus)

	data := map[string]interface{}{
		"ok":             ok,
		"timestamp":      utils.FormatTimestamp(time.Now()),
		"uptime_seconds": uptimeSeconds(),
		"database":       dbStatus,
		"registry":       registryStatus,
		"ops":            opsStatus,
		"agents":         snapshots,
	}

	return &types.StatusResponse{
		Code:    0,
		Message: "OK",
		Data:    data,
	}, nil
}
