package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
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

func (l *OpsNotificationsUpdateLogic) OpsNotificationsUpdate(req *OpsNotificationsUpdateRequest) (resp *OpsNotificationsUpdateResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsNotificationsUpdate not implemented")
}
