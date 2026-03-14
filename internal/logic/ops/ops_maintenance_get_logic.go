package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
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

func (l *OpsMaintenanceGetLogic) OpsMaintenanceGet(req *OpsMaintenanceGetRequest) (resp *OpsMaintenanceGetResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsMaintenanceGet not implemented")
}
