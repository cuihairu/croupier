// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsNotificationsUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新通知配置
func NewOpsNotificationsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNotificationsUpdateLogic {
	return &OpsNotificationsUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNotificationsUpdateLogic) OpsNotificationsUpdate(req *types.OpsNotificationsUpdateRequest) (resp *types.OpsNotificationsUpdateResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsNotificationsUpdate not implemented")
}
