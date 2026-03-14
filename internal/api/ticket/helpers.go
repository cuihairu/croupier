package ticket

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/datatypes"
)

var allowedTicketStatuses = map[string]struct{}{
	"open":        {},
	"in_progress": {},
	"resolved":    {},
	"closed":      {},
}

func parseTicketID(id string) (uint, error) {
	return utils.ParseUintID(id, "工单ID")
}

func sanitizeTicketStatus(status string) (string, error) {
	s := strings.TrimSpace(status)
	if s == "" {
		return "", errorx.NewBadRequest("工单状态不能为空")
	}
	s = strings.ToLower(s)
	if _, ok := allowedTicketStatuses[s]; !ok {
		return "", errorx.NewBadRequest("工单状态无效: " + status)
	}
	return s, nil
}

func buildTicketDTO(ticket *model.Ticket) Ticket {
	return Ticket{
		Id:        int64(ticket.ID),
		Title:     ticket.Title,
		Content:   ticket.Content,
		Category:  ticket.Category,
		Priority:  ticket.Priority,
		Status:    ticket.Status,
		Assignee:  ticket.Assignee,
		Tags:      decodeTicketTags(ticket.Tags),
		PlayerId:  ticket.PlayerID,
		Contact:   ticket.Contact,
		GameId:    ticket.GameID,
		Env:       ticket.Env,
		Source:    ticket.Source,
		CreatedAt: utils.FormatTimestamp(ticket.CreatedAt),
		UpdatedAt: utils.FormatTimestamp(ticket.UpdatedAt),
	}
}

func decodeTicketTags(data interface{}) []string {
	switch v := data.(type) {
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case json.RawMessage:
		var arr []string
		if err := json.Unmarshal(v, &arr); err == nil {
			return arr
		}
	case []byte:
		var arr []string
		if err := json.Unmarshal(v, &arr); err == nil {
			return arr
		}
	}
	return nil
}

func encodeTicketTags(tags []string) datatypes.JSON {
	if len(tags) == 0 {
		return nil
	}
	unique := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		unique = append(unique, trimmed)
	}
	if len(unique) == 0 {
		return nil
	}
	bytes, _ := json.Marshal(unique)
	return datatypes.JSON(bytes)
}

func sanitizePriority(priority string) string {
	p := strings.TrimSpace(priority)
	if p == "" {
		return "medium"
	}
	switch strings.ToLower(p) {
	case "low", "medium", "high", "critical":
		return strings.ToLower(p)
	default:
		return "medium"
	}
}

func buildCommentsDTO(comments []model.TicketComment) []Comment {
	dto := make([]Comment, 0, len(comments))
	for _, c := range comments {
		dto = append(dto, Comment{
			Id:        int64(c.ID),
			Content:   c.Content,
			Author:    c.Author,
			CreatedAt: utils.FormatTimestamp(c.CreatedAt),
		})
	}
	return dto
}

func sanitizeTicketFields(req *CreateRequest) (*model.Ticket, error) {
	title := strings.TrimSpace(req.Title)
	content := strings.TrimSpace(req.Content)
	category := strings.TrimSpace(req.Category)

	if title == "" {
		return nil, errorx.NewBadRequest("工单标题不能为空")
	}
	if content == "" {
		return nil, errorx.NewBadRequest("工单内容不能为空")
	}
	if category == "" {
		return nil, errorx.NewBadRequest("工单分类不能为空")
	}

	return &model.Ticket{
		Title:    title,
		Content:  content,
		Category: category,
		Priority: sanitizePriority(req.Priority),
		Status:   "open",
		Assignee: "",
		Tags:     encodeTicketTags(req.Tags),
		PlayerID: strings.TrimSpace(req.PlayerId),
		Contact:  strings.TrimSpace(req.Contact),
		GameID:   strings.TrimSpace(req.GameId),
		Env:      strings.TrimSpace(req.Env),
		Source:   "api",
	}, nil
}

func addComment(author, content string, ticketID uint) *model.TicketComment {
	return &model.TicketComment{
		TicketID: ticketID,
		Author:   author,
		Content:  strings.TrimSpace(content),
	}
}

func commentAuthor(ctx context.Context) string {
	if name := contextUsername(ctx); name != "" {
		return name
	}
	return "system"
}

func contextUsername(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if username, ok := ctx.Value("username").(string); ok {
		return strings.TrimSpace(username)
	}
	return ""
}
