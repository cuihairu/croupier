// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsAlertSilenceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 静默告警
func NewOpsAlertSilenceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAlertSilenceLogic {
	return &OpsAlertSilenceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAlertSilenceLogic) OpsAlertSilence(req *types.OpsAlertSilenceRequest) (resp *types.OpsAlertSilenceResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
