package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/repo/gorm/support"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type SupportTicketUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportTicketUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportTicketUpdateLogic {
	return &SupportTicketUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportTicketUpdateLogic) SupportTicketUpdate(req *types.SupportTicketUpdateRequest) error {
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return svc.ErrSupportTicketNotFound
	}
	ticket := &support.Ticket{
		Model:    gorm.Model{ID: uint(req.Id)},
		Title:    strings.TrimSpace(req.Title),
		Content:  req.Content,
		Category: req.Category,
		Priority: req.Priority,
		Status:   req.Status,
		Assignee: req.Assignee,
		Tags:     req.Tags,
		PlayerID: req.PlayerId,
		Contact:  req.Contact,
		GameID:   req.GameId,
		Env:      req.Env,
		Source:   req.Source,
	}
	return repo.UpdateTicket(l.ctx, ticket)
}
