// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNotificationsUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新通知配置
func NewOpsNotificationsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNotificationsUpdateLogic {
	return &OpsNotificationsUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNotificationsUpdateLogic) OpsNotificationsUpdate(req *types.OpsNotificationsUpdateRequest) (resp *types.OpsNotificationsUpdateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
