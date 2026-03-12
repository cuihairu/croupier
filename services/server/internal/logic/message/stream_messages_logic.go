// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package message

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type StreamMessagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 消息流（实时推送）
func NewStreamMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StreamMessagesLogic {
	return &StreamMessagesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StreamMessagesLogic) StreamMessages(req *types.StreamMessagesRequest) (resp *types.StreamMessagesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
