// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
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

func (l *OpsMQLogic) OpsMQ(req *types.OpsMQRequest) (resp *types.OpsMQResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsMQ not implemented")
}
