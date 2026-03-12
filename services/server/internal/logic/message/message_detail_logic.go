// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package message

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MessageDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取消息详情
func NewMessageDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MessageDetailLogic {
	return &MessageDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MessageDetailLogic) MessageDetail(req *types.MessageDetailRequest) (resp *types.MessageDetailResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
