package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
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

func (l *OpsSilencesLogic) OpsSilences(req *OpsSilencesRequest) (resp *OpsSilencesResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsSilences not implemented")
}
