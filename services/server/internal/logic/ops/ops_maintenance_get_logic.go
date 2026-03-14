// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsMaintenanceGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取维护模式状态
func NewOpsMaintenanceGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsMaintenanceGetLogic {
	return &OpsMaintenanceGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsMaintenanceGetLogic) OpsMaintenanceGet(req *types.OpsMaintenanceGetRequest) (resp *types.OpsMaintenanceGetResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsMaintenanceGet not implemented")
}
