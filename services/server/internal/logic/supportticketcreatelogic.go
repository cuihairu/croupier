package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/repo/gorm/support"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportTicketCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportTicketCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportTicketCreateLogic {
	return &SupportTicketCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportTicketCreateLogic) SupportTicketCreate(req *types.SupportTicketCreateRequest) (*types.SupportTicketCreateResponse, error) {
	if req == nil || strings.TrimSpace(req.Title) == "" {
		return nil, ErrInvalidRequest
	}
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return nil, ErrInvalidRequest
	}
	ticket := &support.Ticket{
		Title:    strings.TrimSpace(req.Title),
		Content:  req.Content,
		Category: req.Category,
		Priority: defaultPriority(req.Priority),
		Status:   defaultStatus(req.Status),
		Assignee: req.Assignee,
		Tags:     req.Tags,
		PlayerID: req.PlayerId,
		Contact:  req.Contact,
		GameID:   req.GameId,
		Env:      req.Env,
		Source:   req.Source,
	}
	if err := repo.CreateTicket(l.ctx, ticket); err != nil {
		return nil, err
	}
	return &types.SupportTicketCreateResponse{Id: int64(ticket.ID)}, nil
}

func defaultStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "open"
	}
	return status
}

func defaultPriority(priority string) string {
	priority = strings.TrimSpace(priority)
	if priority == "" {
		return "normal"
	}
	return priority
}
