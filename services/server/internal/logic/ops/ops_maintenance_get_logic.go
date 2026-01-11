// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsMaintenanceGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取维护模式状态
func NewOpsMaintenanceGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsMaintenanceGetLogic {
	return &OpsMaintenanceGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsMaintenanceGetLogic) OpsMaintenanceGet(req *types.OpsMaintenanceGetRequest) (resp *types.OpsMaintenanceGetResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
