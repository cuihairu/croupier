// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package message

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
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

func (l *MessageDetailLogic) MessageDetail(req *types.MessageDetailRequest) (*types.MessageDetailResponse, error) {
	id, err := utils.ParseUintID(req.ID, "消息ID")
	if err != nil {
		return nil, err
	}

	msg, err := l.svcCtx.MessageModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.MessageDetailResponse{
		Code:    0,
		Message: "OK",
		Data:    utils.BuildMessageDTO(msg),
	}, nil
}
