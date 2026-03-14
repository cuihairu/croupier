// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type TicketCommentsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取工单评论
func NewTicketCommentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketCommentsLogic {
	return &TicketCommentsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketCommentsLogic) TicketComments(req *types.TicketCommentsRequest) (*types.TicketCommentsResponse, error) {
	id, err := parseTicketID(req.TicketID)
	if err != nil {
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
