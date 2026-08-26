// Player-facing support API for in-game SDK integration (game-support P2;
// see docs/research/game-support-systems.md §3 item 5). Endpoints are
// public like /releases/check — the game client has no admin token — but
// scoped to one player via playerId, rate-limit friendly and read-mostly.
package support

import (
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

// PlayerHandler serves /api/v1/public/support/*.
type PlayerHandler struct {
	svcCtx *svc.ServiceContext
}

// NewPlayerHandler creates a player support handler.
func NewPlayerHandler(svcCtx *svc.ServiceContext) *PlayerHandler {
	return &PlayerHandler{svcCtx: svcCtx}
}

// RegisterPlayerRoutes mounts the player endpoints.
func RegisterPlayerRoutes(g *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	h := NewPlayerHandler(svcCtx)
	pg := g.Group("/public/support")
	{
		pg.GET("/faqs", h.ListFAQs)
		pg.POST("/tickets", h.CreateTicket)
		pg.GET("/tickets", h.ListMyTickets)
	}
}

// ListFAQs serves GET /public/support/faqs?category=&keyword=&tag=.
// Only visible entries are exposed.
func (h *PlayerHandler) ListFAQs(c *gin.Context) {
	if h.svcCtx == nil || h.svcCtx.FAQModel == nil {
		response.Success(c, gin.H{"items": []any{}})
		return
	}
	visible := true
	opts := model.ListFAQOptions{
		PaginationOptions: model.NewPagination(queryInt(c, "page", 1), queryInt(c, "pageSize", 20)),
		Category:          strings.TrimSpace(c.Query("category")),
		Keyword:           strings.TrimSpace(c.Query("keyword")),
		Tag:               strings.TrimSpace(c.Query("tag")),
		Visible:           &visible,
	}
	items, total, err := h.svcCtx.FAQModel.List(c.Request.Context(), opts)
	if err != nil {
		response.Error(c, err)
		return
	}
	type faqItem struct {
		ID           int64    `json:"id"`
		Slug         string   `json:"slug,omitempty"`
		Question     string   `json:"question"`
		Answer       string   `json:"answer"`
		Category     string   `json:"category"`
		Tags         []string `json:"tags,omitempty"`
		HelpfulCount int      `json:"helpfulCount"`
		UnhelpfulCnt int      `json:"unhelpfulCount"`
	}
	out := make([]faqItem, 0, len(items))
	for i := range items {
		out = append(out, faqItem{
			ID: int64(items[i].ID), Slug: items[i].Slug,
			Question: items[i].Question, Answer: items[i].Answer,
			Category: items[i].Category, Tags: decodeFAQTags(items[i].Tags),
			HelpfulCount: items[i].HelpfulCount, UnhelpfulCnt: items[i].UnhelpfulCount,
		})
	}
	response.Success(c, gin.H{"items": out, "total": total, "page": opts.Page, "pageSize": opts.PageSize})
}

// CreateTicketRequest is the in-game submission payload. Contact is optional;
// playerId is the anchor (the game knows who is asking).
type CreateTicketRequest struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	Category string `json:"category,optional"`
	// Player context (game-support P1 vocabulary).
	PlayerID    string `json:"playerId"`
	ServerID    string `json:"serverId,optional"`
	PlayerLevel int    `json:"playerLevel,optional"`
	DeviceOS    string `json:"deviceOs,optional"`
	DeviceModel string `json:"deviceModel,optional"`
	Language    string `json:"language,optional"`
	Contact     string `json:"contact,optional"`
}

// CreateTicket serves POST /public/support/tickets.
func (h *PlayerHandler) CreateTicket(c *gin.Context) {
	if h.svcCtx == nil || h.svcCtx.TicketModel == nil {
		response.Error(c, errorx.NewBadRequest("客服系统未启用"))
		return
	}
	var req CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	req.PlayerID = strings.TrimSpace(req.PlayerID)
	if req.Title == "" || req.Content == "" || req.PlayerID == "" {
		response.Error(c, errorx.NewBadRequest("title/content/playerId 不能为空"))
		return
	}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = "player"
	}

	ticket := &model.Ticket{
		Title:       req.Title,
		Content:     req.Content,
		Category:    category,
		Priority:    "normal",
		Status:      dbenum.TicketStatusOpen,
		Source:      "player",
		PlayerID:    req.PlayerID,
		Contact:     strings.TrimSpace(req.Contact),
		GameID:      strings.TrimSpace(c.Query("gameId")),
		Env:         strings.TrimSpace(c.Query("env")),
		ServerID:    strings.TrimSpace(req.ServerID),
		PlayerLevel: clampLevel(req.PlayerLevel),
		DeviceOS:    strings.TrimSpace(req.DeviceOS),
		DeviceModel: strings.TrimSpace(req.DeviceModel),
		Language:    strings.TrimSpace(req.Language),
	}
	if err := h.svcCtx.TicketModel.Create(c.Request.Context(), ticket); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{
		"ticketId": ticket.ID,
		"status":   ticket.Status.String(),
	})
}

// ListMyTickets serves GET /public/support/tickets?playerId=...: the
// player's own submissions with their public status only (no assignee or
// internal comments).
func (h *PlayerHandler) ListMyTickets(c *gin.Context) {
	if h.svcCtx == nil || h.svcCtx.TicketModel == nil {
		response.Success(c, gin.H{"items": []any{}})
		return
	}
	playerID := strings.TrimSpace(c.Query("playerId"))
	if playerID == "" {
		response.Error(c, errorx.NewBadRequest("playerId 不能为空"))
		return
	}
	opts := model.TicketQueryOptions{
		PaginationOptions: model.NewPagination(queryInt(c, "page", 1), queryInt(c, "pageSize", 20)),
		Query:             "",
		PlayerID:          playerID,
	}
	items, total, err := h.svcCtx.TicketModel.List(c.Request.Context(), opts)
	if err != nil {
		response.Error(c, err)
		return
	}
	type ticketItem struct {
		ID        int64  `json:"id"`
		Title     string `json:"title"`
		Status    string `json:"status"`
		Category  string `json:"category"`
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
	}
	out := make([]ticketItem, 0, len(items))
	for i := range items {
		out = append(out, ticketItem{
			ID: int64(items[i].ID), Title: items[i].Title,
			Status: items[i].Status.String(), Category: items[i].Category,
		})
	}
	response.Success(c, gin.H{"items": out, "total": total, "page": opts.Page, "pageSize": opts.PageSize})
}

func queryInt(c *gin.Context, key string, def int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return def
	}
	n := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func clampLevel(level int) int {
	if level < 0 {
		return 0
	}
	if level > 10000 {
		return 10000
	}
	return level
}

func decodeFAQTags(data interface{}) []string {
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
	default:
		return nil
	}
}
