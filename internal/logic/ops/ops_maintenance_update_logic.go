
package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
	
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

func (l *OpsMaintenanceUpdateLogic) OpsMaintenanceUpdate(req *OpsMaintenanceUpdateRequest) (resp *OpsMaintenanceUpdateResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsMaintenanceUpdate not implemented")
}
