package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

type OpsNotificationsGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取通知配置
func NewOpsNotificationsGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNotificationsGetLogic {
	return &OpsNotificationsGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNotificationsGetLogic) OpsNotificationsGet(req *OpsNotificationsGetRequest) (resp *OpsNotificationsGetResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsNotificationsGet not implemented")
}
