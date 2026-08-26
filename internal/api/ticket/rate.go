// Ticket CSAT rating (game-support P2; see
// docs/research/game-support-systems.md §3 item 10).
package ticket

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dbenum"
)

// Rate stores the satisfaction rating for a closed ticket. Re-rating is
// allowed (last write wins) while the ticket stays closed; reopening clears
// the previous rating so stale scores cannot leak into reports.
func (s *Service) Rate(ctx context.Context, req *RateRequest) (*RateResponse, error) {
	id, err := parseTicketID(req.ID)
	if err != nil {
		return nil, err
	}
	if req.Rating < 1 || req.Rating > 5 {
		return nil, errorx.NewBadRequest("评分范围为 1-5")
	}
	ticket, err := s.svcCtx.TicketModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}
	if ticket.Status != dbenum.TicketStatusClosed && ticket.Status != dbenum.TicketStatusResolved {
		return nil, errorx.NewConflict("仅已解决/已关闭的工单可评价")
	}
	if err := s.svcCtx.TicketModel.Update(ctx, id, map[string]interface{}{
		"rating":   req.Rating,
		"rated_by": commentAuthor(ctx),
		"rated_at": time.Now(),
	}); err != nil {
		return nil, err
	}
	return &RateResponse{TicketID: int64(id), Rating: req.Rating}, nil
}
