// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsMQLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取消息队列状态
func NewOpsMQLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsMQLogic {
	return &OpsMQLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsMQLogic) OpsMQ(req *types.OpsMQRequest) (resp *types.OpsMQResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
