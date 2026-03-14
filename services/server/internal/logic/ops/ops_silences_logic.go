// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsSilencesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取静默规则列表
func NewOpsSilencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsSilencesLogic {
	return &OpsSilencesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsSilencesLogic) OpsSilences(req *types.OpsSilencesRequest) (resp *types.OpsSilencesResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsSilences not implemented")
}
