// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package message

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type MessageReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 标记消息已读
func NewMessageReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MessageReadLogic {
	return &MessageReadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MessageReadLogic) MessageRead(req *types.MessageReadRequest) (*types.MessageReadResponse, error) {
	id, err := utils.ParseUintID(req.ID, "消息ID")
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.MessageModel.MarkRead(l.ctx, id); err != nil {
		return nil, err
	}

	msg, err := l.svcCtx.MessageModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.MessageReadResponse{
		Code:    0,
		Message: "OK",
		Data:    utils.BuildMessageDTO(msg),
	}, nil
}
