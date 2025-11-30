// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package message

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
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

func (l *MessagesListLogic) MessagesList(req *types.MessagesListRequest) (*types.MessagesListResponse, error) {
	opts := model.ListMessagesOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		Type:     strings.TrimSpace(req.Type),
		Status:   strings.TrimSpace(req.Status),
	}

	messages, total, err := l.svcCtx.MessageModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(messages))
	for i := range messages {
		items = append(items, utils.BuildMessageDTO(&messages[i]))
	}

	return &types.MessagesListResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items": items,
			"total": total,
			"page":  opts.Page,
			"size":  opts.PageSize,
		},
	}, nil
}
