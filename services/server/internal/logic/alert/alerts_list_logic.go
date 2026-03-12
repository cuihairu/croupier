// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package alert

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取告警列表
func NewAlertsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertsListLogic {
	return &AlertsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AlertsListLogic) AlertsList(req *types.AlertsListRequest) (resp *types.AlertsListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
