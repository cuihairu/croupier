// Package component 提供组件模板 CRUD API（V4 三层组合）。
package component

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/dbenum"

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
	g.POST("/seed-demo-constants", h.SeedDemoConstants)
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
	// Stale builtin 模板与其依赖契约的当前重算结果不一致（契约已变化，
	// 需「从契约重新生成」刷新）。仅 builtin 模板会标记。
	Stale bool `json:"stale,omitempty"`
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
	staleSet := h.computeStaleKeys(c.GetHeader("X-Game-ID"), c.GetHeader("X-Env"), items)
	response.Success(c, gin.H{
		"items": mapFn(items, func(t model.ComponentTemplate) TemplateDTO {
			dto := toDTO(&t)
			dto.Stale = staleSet[t.Key]
			return dto
		}),
		"total": total,
	})
}

func (h *Handler) listContractsByScope(gameID, env string) ([]*model.FunctionContract, error) {
	if h.contractMdl == nil {
		return nil, nil
	}
	return h.contractMdl.ListByScope(context.Background(), gameID, env)
}

// computeStaleKeys 对 builtin 模板做 stale 检测：按当前 scope 契约在内存中
// 重算同类模板（单函数/查询/CRUD），Tree 不一致即 stale。仅 builtin 参与；
// 契约不可用或无契约时返回空集（不误标）。
func (h *Handler) computeStaleKeys(gameID, env string, items []model.ComponentTemplate) map[string]bool {
	contracts, err := h.listContractsByScope(gameID, env)
	if err != nil || len(contracts) == 0 {
		return nil
	}
	expected := map[string]string{}
	for _, ct := range contracts {
		if ct == nil || strings.TrimSpace(ct.FunctionID) == "" {
			continue
		}
		if tpl := buildSingleFunctionTemplate(ct); tpl != nil {
			expected[tpl.Key] = string(tpl.Tree)
		}
		if tpl := buildQueryTemplate(ct); tpl != nil {
			expected[tpl.Key] = string(tpl.Tree)
		}
	}
	byResource := map[string][]*model.FunctionContract{}
	for _, ct := range contracts {
		if ct == nil || strings.TrimSpace(ct.ResourceKey) == "" {
			continue
		}
		byResource[ct.ResourceKey] = append(byResource[ct.ResourceKey], ct)
	}
	for resource, list := range byResource {
		var listFn, getFn, createFn, updateFn, deleteFn *model.FunctionContract
		for _, ct := range list {
			switch ct.Capability {
			case dbenum.CapabilityCollectionQuery:
				listFn = ct
			case dbenum.CapabilityItemQuery:
				getFn = ct
			case dbenum.CapabilityCreate:
				createFn = ct
			case dbenum.CapabilityUpdate:
				updateFn = ct
			case dbenum.CapabilityDelete:
				deleteFn = ct
			}
		}
		if listFn == nil || getFn == nil {
			continue
		}
		var fns []string
		fns = append(fns, listFn.FunctionID, getFn.FunctionID)
		expected["crud--"+sanitizeKey(resource)] = buildCRUDTree(listFn, getFn, createFn, updateFn, deleteFn, &fns)
	}
	stale := map[string]bool{}
	for _, item := range items {
		if !item.Builtin {
			continue
		}
		exp, ok := expected[item.Key]
		if !ok {
			// 生成条件已消失（函数删除/能力变更导致不再生成该类模板）
			if isBuiltinFamily(item.Key) {
				stale[item.Key] = true
			}
			continue
		}
		if string(item.Tree) != exp {
			stale[item.Key] = true
		}
	}
	return stale
}

// isBuiltinFamily 判断 key 是否属于自动生成家族（fn--/crud--/query--）。
func isBuiltinFamily(key string) bool {
	return strings.HasPrefix(key, "fn--") ||
		strings.HasPrefix(key, "crud--") ||
		strings.HasPrefix(key, "query--")
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
