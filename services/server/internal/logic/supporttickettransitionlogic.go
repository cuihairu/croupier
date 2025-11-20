package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/repo/gorm/support"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportTicketTransitionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportTicketTransitionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportTicketTransitionLogic {
	return &SupportTicketTransitionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportTicketTransitionLogic) SupportTicketTransition(req *types.SupportTicketTransitionRequest) error {
	if req == nil || strings.TrimSpace(req.Status) == "" {
		return ErrInvalidRequest
	}
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return svc.ErrSupportTicketNotFound
	}
	ticket, err := repo.GetTicket(l.ctx, uint(req.Id))
	if err != nil {
		return err
	}
	clean := strings.TrimSpace(req.Status)
	if clean == "" {
		return ErrInvalidRequest
	}
	ticket.Status = clean
	if err := repo.UpdateTicket(l.ctx, ticket); err != nil {
		return err
	}
	if strings.TrimSpace(req.Comment) != "" {
		author := strings.TrimSpace(svc.ActorFromContext(l.ctx))
		if author == "" {
			author = "system"
		}
		comment := &support.TicketComment{
			TicketID: ticket.ID,
			Author:   author,
			Content:  strings.TrimSpace(req.Comment),
		}
		if err := repo.CreateComment(l.ctx, comment); err != nil {
			l.Logger.Errorf("support transition comment failed: %v", err)
		}
	}
	return nil
}
