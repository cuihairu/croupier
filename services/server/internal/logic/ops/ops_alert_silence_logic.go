// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsAlertSilenceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 静默告警
func NewOpsAlertSilenceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAlertSilenceLogic {
	return &OpsAlertSilenceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAlertSilenceLogic) OpsAlertSilence(req *types.OpsAlertSilenceRequest) (resp *types.OpsAlertSilenceResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsAlertSilence not implemented")
}
