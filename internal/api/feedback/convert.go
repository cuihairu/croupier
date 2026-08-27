// Feedback → Ticket conversion (game-support P2; see
// docs/research/game-support-systems.md §3 item 6).
package feedback

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
)

// ConvertToTicket moves one feedback entry into a support ticket, carrying
// the player context. The feedback itself is marked triaged so the queue
// stops surfacing it (the default queue already excludes triaged items).
func (s *Service) ConvertToTicket(ctx context.Context, req *ConvertRequest) (*ConvertResponse, error) {
	id, err := utils.ParseUintID(req.ID, "反馈 ID")
	if err != nil {
		return nil, err
	}
	fb, err := s.svcCtx.FeedbackModel.FindByID(ctx, id)
	if err != nil {
		return nil, errorx.NewBadRequest("反馈不存在")
	}

	// Idempotent: converting an already-converted feedback returns the
	// existing ticket instead of duplicating it.
	if fb.Status == dbenum.FeedbackStatusTriaged {
		if ticketID := extractConvertedTicketID(fb.Reply); ticketID != "" {
			return &ConvertResponse{AlreadyConverted: true, TicketID: ticketID}, nil
		}
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = fmt.Sprintf("[反馈#%d] %s", fb.ID, truncateRunes(firstLine(fb.Content), 40))
	}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = firstNonEmpty(fb.Category, "feedback")
	}
	priority := strings.ToLower(strings.TrimSpace(req.Priority))
	if priority == "" {
		priority = "normal"
	}

	ticket := &model.Ticket{
		Title:    title,
		Content:  buildConvertedContent(fb, req.Note),
		Category: category,
		Priority: priority,
		Status:   dbenum.TicketStatusOpen,
		Source:   "feedback",
		PlayerID: fb.PlayerID,
		Contact:  fb.Contact,
		GameID:   fb.GameID,
		Env:      fb.Env,
		Extra:    convertExtra(fb),
	}
	if err := s.svcCtx.TicketModel.Create(ctx, ticket); err != nil {
		return nil, err
	}

	// Mark the feedback triaged and record the ticket id in the reply field
	// (the historical reply text is preserved above it).
	marker := fmt.Sprintf("[已转工单 #%d]", ticket.ID)
	newReply := marker
	if reply := strings.TrimSpace(req.Note); reply != "" {
		newReply = marker + "\n" + reply
	}
	if err := s.svcCtx.FeedbackModel.Update(ctx, fb.ID, map[string]interface{}{
		"status": dbenum.FeedbackStatusTriaged,
		"reply":  newReply,
	}); err != nil {
		return nil, err
	}

	return &ConvertResponse{TicketID: fmt.Sprint(ticket.ID)}, nil
}

func buildConvertedContent(fb *model.Feedback, note string) string {
	var b strings.Builder
	b.WriteString("由玩家反馈转化。\n\n")
	b.WriteString("【玩家反馈原文】\n")
	b.WriteString(fb.Content)
	if fb.Rating > 0 {
		fmt.Fprintf(&b, "\n\n评分: %d/5", fb.Rating)
	}
	if strings.TrimSpace(note) != "" {
		fmt.Fprintf(&b, "\n\n【处理备注】\n%s", strings.TrimSpace(note))
	}
	return b.String()
}

// convertExtra copies structured context the ticket UI understands.
func convertExtra(fb *model.Feedback) model.JSON {
	extra := map[string]interface{}{}
	if fb.PlayerID != "" {
		extra["playerId"] = fb.PlayerID
	}
	if fb.Attach != "" {
		extra["feedbackAttachment"] = fb.Attach
	}
	extra["feedbackId"] = fb.ID
	if len(extra) == 0 {
		return nil
	}
	bytes, _ := json.Marshal(extra)
	return bytes
}

// extractConvertedTicketID reads back the marker written on conversion.
// Marker format: "[已转工单 #<id>]" possibly followed by a note block.
func extractConvertedTicketID(reply string) string {
	reply = strings.TrimSpace(reply)
	const prefix = "[已转工单 #"
	if !strings.HasPrefix(reply, prefix) {
		return ""
	}
	rest := reply[len(prefix):]
	if i := strings.IndexByte(rest, ']'); i > 0 {
		return rest[:i]
	}
	return ""
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
