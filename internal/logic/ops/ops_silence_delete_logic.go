package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

type OpsSilenceDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除静默规则
func NewOpsSilenceDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsSilenceDeleteLogic {
	return &OpsSilenceDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsSilenceDeleteLogic) OpsSilenceDelete(req *OpsAlertSilenceDeleteRequest) (resp *OpsSilenceDeleteResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsSilenceDelete not implemented")
}
