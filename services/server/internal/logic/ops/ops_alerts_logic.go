// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsAlertsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取告警列表
func NewOpsAlertsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAlertsLogic {
	return &OpsAlertsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAlertsLogic) OpsAlerts(_ *types.OpsAlertsRequest) (*types.OpsAlertsResponse, error) {
	return &types.OpsAlertsResponse{
		Alerts: []types.OpsAlert{},
	}, nil
}
