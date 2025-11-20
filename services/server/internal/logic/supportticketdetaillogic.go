package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportTicketDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportTicketDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportTicketDetailLogic {
	return &SupportTicketDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportTicketDetailLogic) SupportTicketDetail(req *types.SupportTicketUpdateRequest) (*types.SupportTicketResponse, error) {
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return nil, svc.ErrSupportTicketNotFound
	}
	ticket, err := repo.GetTicket(l.ctx, uint(req.Id))
	if err != nil {
		return nil, err
	}
	return &types.SupportTicketResponse{
		Ticket: supportTicketToType(ticket),
	}, nil
}
