// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type TicketCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建工单
func NewTicketCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketCreateLogic {
	return &TicketCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketCreateLogic) TicketCreate(req *types.TicketCreateRequest) (*types.TicketDetailResponse, error) {
	ticket, err := sanitizeTicketFields(req)
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.TicketModel.Create(l.ctx, ticket); err != nil {
		return nil, err
	}

	return &types.TicketDetailResponse{
		Ticket: buildTicketDTO(ticket),
	}, nil
}
