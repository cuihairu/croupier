// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type TicketsListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取工单列表
func NewTicketsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketsListLogic {
	return &TicketsListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketsListLogic) TicketsList(req *types.TicketsListRequest) (*types.TicketsListResponse, error) {
	opts := model.TicketQueryOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		Status:   strings.TrimSpace(req.Status),
		Category: strings.TrimSpace(req.Category),
		Priority: strings.TrimSpace(req.Priority),
		Assignee: strings.TrimSpace(req.Assignee),
	}

	items, total, err := l.svcCtx.TicketModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	dto := make([]types.Ticket, 0, len(items))
	for i := range items {
		dto = append(dto, buildTicketDTO(&items[i]))
	}

	return &types.TicketsListResponse{
		Items: dto,
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}, nil
}
