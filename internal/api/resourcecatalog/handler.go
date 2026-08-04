package resourcecatalog

import (
	"github.com/cuihairu/croupier/internal/common/requestbind"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// Handler provides HTTP handlers for Resource Catalog API.
type Handler struct {
	service *Service
}

// NewHandler creates a new handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles GET /api/resource-catalog
func (h *Handler) List(c *gin.Context) {
	var req ListRequest
	if err := requestbind.BindQueryCompat(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	// Get scope from context
	gameID, env := getScope(c)
	req.GameID = gameID
	req.Env = env

	resp, err := h.service.List(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// Detail handles GET /api/resource-catalog/:resourceKey
func (h *Handler) Detail(c *gin.Context) {
	var req DetailRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	// Get scope from context
	gameID, env := getScope(c)
	req.GameID = gameID
	req.Env = env

	resp, err := h.service.Detail(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// UpdateSemantics handles PUT /api/resource-catalog/:resourceKey/semantics
func (h *Handler) UpdateSemantics(c *gin.Context) {
	var req UpdateSemanticsRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	// Get scope from context
	gameID, env := getScope(c)
	req.GameID = gameID
	req.Env = env

	resp, err := h.service.UpdateSemantics(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// ListConflicts handles GET /api/resource-catalog/:resourceKey/conflicts
func (h *Handler) ListConflicts(c *gin.Context) {
	var req ListConflictsRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	// Get scope from context
	gameID, env := getScope(c)
	req.GameID = gameID
	req.Env = env

	resp, err := h.service.ListConflicts(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// ResolveConflict handles POST /api/resource-catalog/:resourceKey/conflicts/:field/resolve
func (h *Handler) ResolveConflict(c *gin.Context) {
	var req ResolveConflictRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	// Get scope from context
	gameID, env := getScope(c)
	req.GameID = gameID
	req.Env = env

	resp, err := h.service.ResolveConflict(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// getScope extracts game_id and env from context.
func getScope(c *gin.Context) (string, string) {
	gameID := c.GetString("game_id")
	env := c.GetString("env")
	return gameID, env
}
