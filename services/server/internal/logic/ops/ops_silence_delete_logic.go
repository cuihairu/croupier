// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
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

func (l *OpsSilenceDeleteLogic) OpsSilenceDelete(req *types.OpsAlertSilenceDeleteRequest) (resp *types.OpsSilenceDeleteResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsSilenceDelete not implemented")
}
