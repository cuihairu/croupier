// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package message

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MessagesUnreadCountLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取未读消息数量
func NewMessagesUnreadCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MessagesUnreadCountLogic {
	return &MessagesUnreadCountLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MessagesUnreadCountLogic) MessagesUnreadCount(req *types.MessagesUnreadCountRequest) (*types.MessagesUnreadCountResponse, error) {
	count, err := l.svcCtx.MessageModel.CountUnread(l.ctx, "")
	if err != nil {
		return nil, err
	}

	return &types.MessagesUnreadCountResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"count": count,
		},
	}, nil
}
