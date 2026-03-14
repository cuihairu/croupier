package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

type OpsMQLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取消息队列状态
func NewOpsMQLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsMQLogic {
	return &OpsMQLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsMQLogic) OpsMQ(req *OpsMQRequest) (resp *OpsMQResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsMQ not implemented")
}
