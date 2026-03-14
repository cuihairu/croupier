package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
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

func (l *OpsAlertSilenceLogic) OpsAlertSilence(req *OpsAlertSilenceRequest) (resp *OpsAlertSilenceResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsAlertSilence not implemented")
}
