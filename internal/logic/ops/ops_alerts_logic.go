
package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
	
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

func (l *OpsAlertsLogic) OpsAlerts(req *OpsAlertsRequest) (resp *OpsAlertsResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsAlerts not implemented")
}
