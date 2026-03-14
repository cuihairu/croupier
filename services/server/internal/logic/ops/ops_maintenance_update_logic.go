// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsMaintenanceUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新维护模式
func NewOpsMaintenanceUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsMaintenanceUpdateLogic {
	return &OpsMaintenanceUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsMaintenanceUpdateLogic) OpsMaintenanceUpdate(req *types.OpsMaintenanceUpdateRequest) (resp *types.OpsMaintenanceUpdateResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsMaintenanceUpdate not implemented")
}
