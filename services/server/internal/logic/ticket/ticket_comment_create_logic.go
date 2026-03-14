// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type TicketCommentCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建工单评论
func NewTicketCommentCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketCommentCreateLogic {
	return &TicketCommentCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketCommentCreateLogic) TicketCommentCreate(req *types.TicketCommentCreateRequest) (*types.TicketCommentsResponse, error) {
	id, err := parseTicketID(req.TicketID)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, errorx.NewBadRequest("评论内容不能为空")
	}

	if _, err := l.svcCtx.TicketModel.FindOne(l.ctx, id); err != nil {
		return nil, err
	}

	comment := addComment(commentAuthor(l.ctx), content, id)
	if err := l.svcCtx.TicketModel.CreateComment(l.ctx, comment); err != nil {
		return nil, err
	}

	comments, err := l.svcCtx.TicketModel.ListComments(l.ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.TicketCommentsResponse{
		Items: buildCommentsDTO(comments),
	}, nil
}
