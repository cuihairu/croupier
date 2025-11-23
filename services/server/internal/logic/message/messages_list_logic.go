// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package message

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MessagesListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取消息列表
func NewMessagesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MessagesListLogic {
	return &MessagesListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MessagesListLogic) MessagesList(req *types.MessagesListRequest) (resp *types.MessagesListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
