// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNotificationsGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取通知配置
func NewOpsNotificationsGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNotificationsGetLogic {
	return &OpsNotificationsGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNotificationsGetLogic) OpsNotificationsGet(req *types.OpsNotificationsGetRequest) (*types.OpsNotificationsGetResponse, error) {
	state := snapshotOpsState(l.svcCtx)
	return &types.OpsNotificationsGetResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"channels":  state.Notifications.Channels,
			"rules":     state.Notifications.Rules,
			"updatedAt": utils.FormatTimestamp(state.Notifications.UpdatedAt),
		},
	}, nil
}
