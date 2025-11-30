// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TicketUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新工单
func NewTicketUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketUpdateLogic {
	return &TicketUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketUpdateLogic) TicketUpdate(req *types.TicketUpdateRequest) (*types.TicketDetailResponse, error) {
	id, err := parseTicketID(req.ID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if v := strings.TrimSpace(req.Title); v != "" {
		updates["title"] = v
	}
	if v := strings.TrimSpace(req.Content); v != "" {
		updates["content"] = v
	}
	if v := strings.TrimSpace(req.Category); v != "" {
		updates["category"] = v
	}
	if v := strings.TrimSpace(req.Priority); v != "" {
		updates["priority"] = sanitizePriority(v)
	}
	if v := strings.TrimSpace(req.Assignee); v != "" {
		updates["assignee"] = v
	}
	if req.Tags != nil {
		updates["tags"] = encodeTicketTags(req.Tags)
	}

	if len(updates) == 0 {
		return nil, fmt.Errorf("请提供需要更新的字段")
	}

	if err := l.svcCtx.TicketModel.Update(l.ctx, id, updates); err != nil {
		return nil, err
	}

	ticket, err := l.svcCtx.TicketModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.TicketDetailResponse{
		Ticket: buildTicketDTO(ticket),
	}, nil
}
