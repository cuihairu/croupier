package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportTicketsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportTicketsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportTicketsListLogic {
	return &SupportTicketsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportTicketsListLogic) SupportTicketsList(req *types.SupportTicketsQuery) (*types.SupportTicketsResponse, error) {
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return &types.SupportTicketsResponse{Tickets: []types.SupportTicket{}}, nil
	}
	opts := svc.SupportListOptions{
		Query:    req.Q,
		Status:   req.Status,
		Priority: req.Priority,
		Category: req.Category,
		Assignee: req.Assignee,
		GameID:   req.GameId,
		Env:      req.Env,
		Page:     req.Page,
		Size:     req.Size,
	}
	items, total, err := repo.ListTickets(l.ctx, opts)
	if err != nil {
		return nil, err
	}
	out := make([]types.SupportTicket, 0, len(items))
	for _, t := range items {
		out = append(out, supportTicketToType(t))
	}
	page := opts.Page
	if page <= 0 {
		page = 1
	}
	size := opts.Size
	if size <= 0 || size > 200 {
		size = 20
	}
	return &types.SupportTicketsResponse{
		Tickets: out,
		Total:   total,
		Page:    page,
		Size:    size,
	}, nil
}
