// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package message

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MessageReadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 标记消息已读
func NewMessageReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MessageReadLogic {
	return &MessageReadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MessageReadLogic) MessageRead(req *types.MessageReadRequest) (resp *types.MessageReadResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
