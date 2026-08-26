// Ticket → Bug conversion (bug-tracking P2; see
// docs/research/bug-tracking-design.md §3.2).
package ticket

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
)

// ConvertToBugRequest escalates one ticket into a tracked defect.
type ConvertToBugRequest struct {
	ID string `uri:"id"`
	// Optional overrides; defaults derive from the ticket.
	Severity   *string `json:"severity,optional"`
	Platform   *string `json:"platform,optional"`
	Steps      *string `json:"steps,optional"`
	FixVersion *string `json:"fixVersion,optional"`
}

// ConvertToBugResponse returns the created bug id.
type ConvertToBugResponse struct {
	BugID string `json:"bugId"`
}

// ConvertToBug files a Bug from a ticket, carrying the full player context
// (P1 columns + extra JSON) and linking back via source=ticket so "which
// player reports produced this defect" stays answerable.
func (s *Service) ConvertToBug(ctx context.Context, req *ConvertToBugRequest) (*ConvertToBugResponse, error) {
	id, err := parseTicketID(req.ID)
	if err != nil {
		return nil, err
	}
	ticket, err := s.svcCtx.TicketModel.FindOne(ctx, id)
	if err != nil {
		return nil, errorx.NewBadRequest("工单不存在")
	}

	bug := &model.Bug{
		Title:          truncate(fmt.Sprintf("[工单#%d] %s", ticket.ID, ticket.Title), 255),
		Content:        buildBugContent(ticket),
		Status:         model.BugStatusTriage,
		Severity:       derefOr(req.Severity, ""),
		Priority:       mapTicketPriority(ticket.Priority),
		GameID:         ticket.GameID,
		Env:            ticket.Env,
		ServerID:       ticket.ServerID,
		Platform:       derefOr(req.Platform, ""),
		Device:         ticket.DeviceModel,
		OS:             ticket.DeviceOS,
		Steps:          derefOr(req.Steps, ticket.Content),
		Source:         "ticket",
		SourceTicketID: ticket.ID,
		PlayerID:       ticket.PlayerID,
		FixVersion:     derefOr(req.FixVersion, ""),
		Extra:          decodeTicketExtra(ticket.Extra),
		CreatedBy:      commentAuthor(ctx),
	}
	if err := s.svcCtx.BugModel.Create(ctx, bug); err != nil {
		return nil, err
	}

	// Leave an audit trail comment on the ticket (non-fatal on failure).
	comment := addComment(commentAuthor(ctx), fmt.Sprintf("[升级缺陷] 已创建缺陷追踪 #%d", bug.ID), id)
	_ = s.svcCtx.TicketModel.CreateComment(ctx, comment)
	return &ConvertToBugResponse{BugID: fmt.Sprint(bug.ID)}, nil
}

func buildBugContent(ticket *model.Ticket) string {
	var b strings.Builder
	b.WriteString("由客服工单升级。\n\n")
	b.WriteString("【工单原文】\n")
	b.WriteString(ticket.Content)
	if ticket.PlayerID != "" || ticket.ServerID != "" {
		fmt.Fprintf(&b, "\n\n玩家: %s 区服: %s 设备: %s (%s)",
			ticket.PlayerID, ticket.ServerID, ticket.DeviceModel, ticket.DeviceOS)
	}
	return b.String()
}

// mapTicketPriority converts ticket priority vocabulary to bug defaults:
// tickets keep their urgency, defects inherit it unless overridden.
func mapTicketPriority(p string) string {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "urgent":
		return "urgent"
	case "high":
		return "high"
	case "low":
		return "low"
	default:
		return "normal"
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func derefOr(v *string, def string) string {
	if v == nil {
		return def
	}
	return strings.TrimSpace(*v)
}
