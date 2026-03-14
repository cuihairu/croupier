// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package message

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type StreamMessagesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 消息流（实时推送）
func NewStreamMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StreamMessagesLogic {
	return &StreamMessagesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StreamMessagesLogic) StreamMessages(req *types.StreamMessagesRequest) (*types.StreamMessagesResponse, error) {
	messages, err := l.svcCtx.MessageModel.Recent(l.ctx, 20, "")
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(messages))
	for i := range messages {
		items = append(items, utils.BuildMessageDTO(&messages[i]))
	}

	return &types.StreamMessagesResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items": items,
		},
	}, nil
}
