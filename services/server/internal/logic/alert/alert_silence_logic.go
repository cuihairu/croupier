// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package alert

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertSilenceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 静默告警
func NewAlertSilenceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertSilenceLogic {
	return &AlertSilenceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AlertSilenceLogic) AlertSilence(req *types.AlertSilenceRequest) error {
	// todo: add your logic here and delete this line

	return nil
}
