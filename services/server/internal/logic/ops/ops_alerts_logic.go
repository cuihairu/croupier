// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsAlertsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取告警列表
func NewOpsAlertsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAlertsLogic {
	return &OpsAlertsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAlertsLogic) OpsAlerts(req *types.OpsAlertsRequest) (resp *types.OpsAlertsResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsAlerts not implemented")
}
