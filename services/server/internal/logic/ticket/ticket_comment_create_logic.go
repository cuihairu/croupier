// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TicketCommentCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建工单评论
func NewTicketCommentCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketCommentCreateLogic {
	return &TicketCommentCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketCommentCreateLogic) TicketCommentCreate(req *types.TicketCommentCreateRequest) (resp *types.TicketCommentsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
