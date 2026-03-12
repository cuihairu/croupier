// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TicketCommentsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取工单评论
func NewTicketCommentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketCommentsLogic {
	return &TicketCommentsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketCommentsLogic) TicketComments(req *types.TicketCommentsRequest) (resp *types.TicketCommentsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
