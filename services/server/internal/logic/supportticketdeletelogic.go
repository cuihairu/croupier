package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportTicketDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportTicketDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportTicketDeleteLogic {
	return &SupportTicketDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportTicketDeleteLogic) SupportTicketDelete(req *types.SupportTicketUpdateRequest) error {
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return svc.ErrSupportTicketNotFound
	}
	return repo.DeleteTicket(l.ctx, uint(req.Id))
}
