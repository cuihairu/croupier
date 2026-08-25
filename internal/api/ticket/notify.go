package ticket

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
)

// notifyAssignee sends an in-app message when a ticket is assigned to a user.
// Self-assignment (operator == assignee) is skipped to avoid noise.
func (s *Service) notifyAssignee(ctx context.Context, ticket *model.Ticket, assignee, operator string) {
	assignee = strings.TrimSpace(assignee)
	if assignee == "" || assignee == operator {
		return
	}
	s.sendMessage(ctx, assignee, "ticket.assigned",
		fmt.Sprintf("工单 #%d 已分配给你", ticket.ID),
		fmt.Sprintf("【%s】%s", ticket.Category, ticket.Title),
		map[string]interface{}{
			"ticketId": ticket.ID,
			"status":   ticket.Status,
			"category": ticket.Category,
			"priority": ticket.Priority,
			"gameId":   ticket.GameID,
			"env":      ticket.Env,
		})
}

// notifyTicketEvent notifies the ticket stakeholders (assignee, or the
// creator fallback) about comments and status transitions. The acting
// operator is never notified about their own action.
func (s *Service) notifyTicketEvent(ctx context.Context, ticket *model.Ticket, operator, title, content string) {
	recipients := map[string]struct{}{}
	if name := strings.TrimSpace(ticket.Assignee); name != "" {
		recipients[name] = struct{}{}
	}
	delete(recipients, operator)
	data := map[string]interface{}{
		"ticketId": ticket.ID,
		"status":   ticket.Status,
		"category": ticket.Category,
		"priority": ticket.Priority,
		"gameId":   ticket.GameID,
		"env":      ticket.Env,
	}
	for name := range recipients {
		s.sendMessage(ctx, name, "ticket.updated", title, content, data)
	}
}

// sendMessage persists an in-app message; notification failures never block
// the ticket operation itself.
func (s *Service) sendMessage(ctx context.Context, to, msgType, title, content string, data map[string]interface{}) {
	if s == nil || s.svcCtx == nil || s.svcCtx.MessageModel == nil {
		return
	}
	to = strings.TrimSpace(to)
	if to == "" {
		return
	}
	dataJSON, err := model.EncodeData(data)
	if err != nil {
		return
	}
	msg := &model.Message{
		To:      to,
		Type:    msgType,
		Title:   title,
		Content: content,
		Data:    dataJSON,
		Status:  dbenum.MessageStatusUnread,
	}
	_ = s.svcCtx.MessageModel.Create(ctx, msg)
}

// truncateContent shortens message content for notification previews.
func truncateContent(content string, limit int) string {
	runes := []rune(strings.TrimSpace(content))
	if limit <= 0 || len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "..."
}

// validateAssignee ensures the assignee username exists before it is written
// to a ticket. Returns the trimmed username.
func (s *Service) validateAssignee(ctx context.Context, assignee string) (string, error) {
	assignee = strings.TrimSpace(assignee)
	if assignee == "" {
		return "", nil
	}
	if s.svcCtx.AdminModel == nil {
		return assignee, nil
	}
	if _, err := s.svcCtx.AdminModel.FindByUsername(ctx, assignee); err != nil {
		return "", errors.New("处理人账号不存在: " + assignee)
	}
	return assignee, nil
}
