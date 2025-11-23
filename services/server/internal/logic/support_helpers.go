package logic

import (
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/repo/gorm/support"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

func splitTags(tags string) []string {
	if tags == "" {
		return []string{}
	}
	parts := strings.Split(tags, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func supportTicketToType(t *support.Ticket) types.SupportTicket {
	return types.SupportTicket{
		Id:        int64(t.ID),
		Title:     t.Title,
		Content:   t.Content,
		Category:  t.Category,
		Priority:  t.Priority,
		Status:    t.Status,
		Assignee:  t.Assignee,
		Tags:      splitTags(t.Tags),
		PlayerId:  t.PlayerID,
		Contact:   t.Contact,
		GameId:    t.GameID,
		Env:       t.Env,
		Source:    t.Source,
		CreatedAt: formatSupportTime(t.CreatedAt),
		UpdatedAt: formatSupportTime(t.UpdatedAt),
	}
}

func supportFAQToType(f *support.FAQ) types.SupportFAQ {
	return types.SupportFAQ{
		Id:        int64(f.ID),
		Question:  f.Question,
		Answer:    f.Answer,
		Category:  f.Category,
		Tags:      splitTags(f.Tags),
		Visible:   f.Visible,
		Sort:      f.Sort,
		CreatedAt: formatSupportTime(f.CreatedAt),
		UpdatedAt: formatSupportTime(f.UpdatedAt),
	}
}

func supportFeedbackToType(f *support.Feedback) types.SupportFeedback {
	return types.SupportFeedback{
		Id:        int64(f.ID),
		PlayerId:  f.PlayerID,
		Contact:   f.Contact,
		Content:   f.Content,
		Category:  f.Category,
		Priority:  f.Priority,
		Status:    f.Status,
		Attach:    f.Attach,
		GameId:    f.GameID,
		Env:       f.Env,
		CreatedAt: formatSupportTime(f.CreatedAt),
		UpdatedAt: formatSupportTime(f.UpdatedAt),
	}
}

func formatSupportTime(tp time.Time) string {
	if tp.IsZero() {
		return ""
	}
	return tp.Format(time.RFC3339)
}
