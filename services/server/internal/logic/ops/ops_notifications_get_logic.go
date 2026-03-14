// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
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

func (l *OpsNotificationsGetLogic) OpsNotificationsGet(req *types.OpsNotificationsGetRequest) (resp *types.OpsNotificationsGetResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsNotificationsGet not implemented")
}
