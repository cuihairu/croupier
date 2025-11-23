// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsMaintenanceUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新维护模式
func NewOpsMaintenanceUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsMaintenanceUpdateLogic {
	return &OpsMaintenanceUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsMaintenanceUpdateLogic) OpsMaintenanceUpdate(req *types.OpsMaintenanceUpdateRequest) (resp *types.OpsMaintenanceUpdateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
