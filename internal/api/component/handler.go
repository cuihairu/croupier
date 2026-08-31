// Package component 提供组件模板 CRUD API（V4 三层组合）。
package component

import (
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
)

// Handler serves /api/v1/component-templates.
type Handler struct {
	model       *model.ComponentTemplateModel
	contractMdl *model.FunctionContractModel
}

// NewHandler creates a handler.
func NewHandler(m *model.ComponentTemplateModel, contractMdl *model.FunctionContractModel) *Handler {
	return &Handler{model: m, contractMdl: contractMdl}
}

// loadContracts loads contracts for the current scope (game/env).
func (h *Handler) loadContracts(c *gin.Context) ([]*model.FunctionContract, error) {
	if h.contractMdl == nil {
		return nil, nil
	}
	gameID := c.GetHeader("X-Game-ID")
	env := c.GetHeader("X-Env")
	return h.contractMdl.ListByScope(c.Request.Context(), gameID, env)
}

// Register wires routes under the given group.
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("", h.List)
	g.GET("/", h.List)
	g.GET("/:key", h.Get)
	g.POST("", h.Create)
	g.PUT("/:key", h.Update)
	g.DELETE("/:key", h.Delete)
	g.POST("/regenerate", h.Regenerate)
}

// TemplateDTO is the wire shape.
type TemplateDTO struct {
	Key               string          `json:"key"`
	Name              json.RawMessage `json:"name"`
	Description       json.RawMessage `json:"description,omitempty"`
	Category          string          `json:"category,omitempty"`
	Icon              string          `json:"icon,omitempty"`
	RequiredFunctions json.RawMessage `json:"requiredFunctions,omitempty"`
	Tree              json.RawMessage `json:"tree"`
	Builtin           bool            `json:"builtin"`
	CreatedBy         string          `json:"createdBy,omitempty"`
}

func toDTO(t *model.ComponentTemplate) TemplateDTO {
	return TemplateDTO{
		Key:               t.Key,
		Name:              json.RawMessage(t.Name),
		Description:       json.RawMessage(t.Description),
		Category:          t.Category,
		Icon:              t.Icon,
		RequiredFunctions: json.RawMessage(t.RequiredFunctions),
		Tree:              json.RawMessage(t.Tree),
		Builtin:           t.Builtin,
		CreatedBy:         t.CreatedBy,
	}
}

// List serves GET /component-templates.
func (h *Handler) List(c *gin.Context) {
	opts := model.ComponentTemplateListOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     queryInt(c, "page", 1),
			PageSize: queryInt(c, "pageSize", 50),
		},
		Category:    c.Query("category"),
		BuiltinOnly: c.Query("builtinOnly") == "true",
	}
	items, total, err := h.model.List(c.Request.Context(), opts)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{
		"items": mapFn(items, func(t model.ComponentTemplate) TemplateDTO {
			return toDTO(&t)
		}),
		"total": total,
	})
}

// Get serves GET /component-templates/:key.
func (h *Handler) Get(c *gin.Context) {
	tpl, err := h.model.FindByKey(c.Request.Context(), c.Param("key"))
	if err != nil {
		response.NotFound(c, "组件模板不存在")
		return
	}
	response.Success(c, toDTO(tpl))
}

// CreateRequest is the POST body.
type CreateRequest struct {
	Key               string          `json:"key" binding:"required"`
	Name              json.RawMessage `json:"name" binding:"required"`
	Description       json.RawMessage `json:"description,omitempty"`
	Category          string          `json:"category,omitempty"`
	Icon              string          `json:"icon,omitempty"`
	RequiredFunctions json.RawMessage `json:"requiredFunctions,omitempty"`
	Tree              json.RawMessage `json:"tree" binding:"required"`
}

// Create serves POST /component-templates.
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if strings.TrimSpace(req.Key) == "" {
		response.BadRequest(c, "key 不能为空")
		return
	}
	tpl := &model.ComponentTemplate{
		Key:               req.Key,
		Name:              model.JSON(req.Name),
		Description:       model.JSON(req.Description),
		Category:          req.Category,
		Icon:              req.Icon,
		RequiredFunctions: model.JSON(req.RequiredFunctions),
		Tree:              model.JSON(req.Tree),
		Builtin:           false,
		CreatedBy:         currentUser(c),
	}
	if err := h.model.Create(c.Request.Context(), tpl); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, toDTO(tpl))
}

// Update serves PUT /component-templates/:key.
func (h *Handler) Update(c *gin.Context) {
	tpl, err := h.model.FindByKey(c.Request.Context(), c.Param("key"))
	if err != nil {
		response.NotFound(c, "组件模板不存在")
		return
	}
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	updates := map[string]interface{}{
		"name":    model.JSON(req.Name),
		"tree":    model.JSON(req.Tree),
		"builtin": false,
	}
	if req.Description != nil {
		updates["description"] = model.JSON(req.Description)
	}
	if req.Category != "" {
		updates["category"] = req.Category
	}
	if req.Icon != "" {
		updates["icon"] = req.Icon
	}
	if req.RequiredFunctions != nil {
		updates["required_functions"] = model.JSON(req.RequiredFunctions)
	}
	if err := h.model.Update(c.Request.Context(), tpl.ID, updates); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"updated": tpl.Key})
}

// Delete serves DELETE /component-templates/:key.
func (h *Handler) Delete(c *gin.Context) {
	tpl, err := h.model.FindByKey(c.Request.Context(), c.Param("key"))
	if err != nil {
		response.NotFound(c, "组件模板不存在")
		return
	}
	if err := h.model.Delete(c.Request.Context(), tpl.ID); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": tpl.Key})
}

func queryInt(c *gin.Context, key string, def int) int {
	v := strings.TrimSpace(c.Query(key))
	if v == "" {
		return def
	}
	n := 0
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return def
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

func mapFn[T any, R any](items []T, fn func(T) R) []R {
	out := make([]R, len(items))
	for i, item := range items {
		out[i] = fn(item)
	}
	return out
}

func currentUser(c *gin.Context) string {
	if v, ok := c.Get("username"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Regenerate serves POST /component-templates/regenerate：
// 扫描当前 scope 全部契约 → 生成/更新内置组件模板。
func (h *Handler) Regenerate(c *gin.Context) {
	// 从 FunctionContractModel 拉取当前 scope 的契约
	contracts, err := h.loadContracts(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := h.RegenerateFromContracts(c.Request.Context(), contracts); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"regenerated": len(contracts)})
}
