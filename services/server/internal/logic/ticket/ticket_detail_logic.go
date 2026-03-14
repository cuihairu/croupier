// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type TicketDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取工单详情
func NewTicketDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketDetailLogic {
	return &TicketDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketDetailLogic) TicketDetail(req *types.TicketDetailRequest) (*types.TicketDetailResponse, error) {
	id, err := parseTicketID(req.ID)
	if err != nil {
		return nil, err
	}

	ticket, err := l.svcCtx.TicketModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	comments, err := l.svcCtx.TicketModel.ListComments(l.ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.TicketDetailResponse{
		Ticket:   buildTicketDTO(ticket),
		Comments: buildCommentsDTO(comments),
	}, nil
}
