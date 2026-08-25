package ticket

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dbenum"
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

func sanitizeTicketStatus(status string) (dbenum.TicketStatus, error) {
	s := strings.ToLower(strings.TrimSpace(status))
	if s == "" {
		return 0, errorx.NewBadRequest("工单状态不能为空")
	}
	parsed, err := dbenum.ParseTicketStatus(s)
	if err != nil {
		return 0, errorx.NewBadRequest("工单状态无效: " + status)
	}
	return parsed, nil
}

func buildTicketDTO(ticket *model.Ticket) Ticket {
	return Ticket{
		Id:       int64(ticket.ID),
		Title:    ticket.Title,
		Content:  ticket.Content,
		Category: ticket.Category,
		Priority: ticket.Priority,
		Status:   ticket.Status.String(),
		Assignee: ticket.Assignee,
		Tags:     decodeTicketTags(ticket.Tags),
		PlayerId: ticket.PlayerID,
		Contact:  ticket.Contact,
		GameId:   ticket.GameID,
		Env:      ticket.Env,
		Source:   ticket.Source,
		// 玩家上下文（game-support P1）
		ServerId:    ticket.ServerID,
		PlayerLevel: ticket.PlayerLevel,
		DeviceOS:    ticket.DeviceOS,
		DeviceModel: ticket.DeviceModel,
		Language:    ticket.Language,
		Extra:       decodeTicketExtra(ticket.Extra),
		CreatedAt:   utils.FormatTimestamp(ticket.CreatedAt),
		UpdatedAt:   utils.FormatTimestamp(ticket.UpdatedAt),
	}
}

// decodeTicketExtra unmarshals the free-form context payload for the API
// response; nil when absent.
func decodeTicketExtra(data datatypes.JSON) map[string]interface{} {
	if len(data) == 0 {
		return nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
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
		Status:   dbenum.TicketStatusOpen,
		Assignee: strings.TrimSpace(req.Assignee),
		Tags:     encodeTicketTags(req.Tags),
		PlayerID: strings.TrimSpace(req.PlayerId),
		Contact:  strings.TrimSpace(req.Contact),
		GameID:   strings.TrimSpace(req.GameId),
		Env:      strings.TrimSpace(req.Env),
		Source:   "api",
		// 玩家上下文（game-support P1）
		ServerID:    strings.TrimSpace(req.ServerId),
		PlayerLevel: sanitizePlayerLevel(req.PlayerLevel),
		DeviceOS:    strings.TrimSpace(req.DeviceOS),
		DeviceModel: strings.TrimSpace(req.DeviceModel),
		Language:    strings.TrimSpace(req.Language),
		Extra:       encodeTicketExtra(req.Extra),
	}, nil
}

// sanitizePlayerLevel clamps the level into a sane range (0 = unknown).
func sanitizePlayerLevel(level int) int {
	if level < 0 {
		return 0
	}
	if level > 10000 {
		return 10000
	}
	return level
}

// encodeTicketExtra marshals the free-form context payload; nil when empty.
func encodeTicketExtra(extra map[string]interface{}) datatypes.JSON {
	if len(extra) == 0 {
		return nil
	}
	bytes, err := json.Marshal(extra)
	if err != nil {
		return nil
	}
	return datatypes.JSON(bytes)
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
