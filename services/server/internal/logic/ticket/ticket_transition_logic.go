// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type TicketTransitionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 工单状态转换
func NewTicketTransitionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketTransitionLogic {
	return &TicketTransitionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketTransitionLogic) TicketTransition(req *types.TicketTransitionRequest) (*types.TicketDetailResponse, error) {
	id, err := parseTicketID(req.ID)
	if err != nil {
		return nil, err
	}

	status, err := sanitizeTicketStatus(req.Status)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"status": status,
	}
	if note := strings.TrimSpace(req.Note); note != "" {
		comment := addComment(commentAuthor(l.ctx), fmt.Sprintf("[状态变更] %s", note), id)
		if err := l.svcCtx.TicketModel.CreateComment(l.ctx, comment); err != nil {
			return nil, err
		}
	}

	if err := l.svcCtx.TicketModel.Update(l.ctx, id, updates); err != nil {
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
