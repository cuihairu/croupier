package versioning

import (
	"github.com/cuihairu/croupier/internal/common/requestbind"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

// Handler provides HTTP handlers for Versioning API.
type Handler struct {
	service *Service
}

// NewHandler creates a new handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetChangeChain handles GET /api/versioning/pages/:pageKey/chain
func (h *Handler) GetChangeChain(c *gin.Context) {
	var req GetChangeChainRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	// Get scope from context
	gameID, env := getScope(c)
	req.GameID = gameID
	req.Env = env

	resp, err := h.service.GetChangeChain(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// Diff handles GET /api/versioning/pages/:pageKey/diff
func (h *Handler) Diff(c *gin.Context) {
	var req DiffRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := requestbind.BindQueryCompat(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	// Get scope from context
	gameID, env := getScope(c)
	req.GameID = gameID
	req.Env = env

	resp, err := h.service.Diff(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// Merge handles POST /api/versioning/pages/:pageKey/merge
func (h *Handler) Merge(c *gin.Context) {
	var req MergeRequest
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

	resp, err := h.service.Merge(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// RollbackDraft handles POST /api/versioning/pages/:pageKey/rollback-draft
func (h *Handler) RollbackDraft(c *gin.Context) {
	var req RollbackRequest
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

	resp, err := h.service.RollbackDraft(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// RollbackPublish handles POST /api/versioning/pages/:pageKey/rollback-publish
func (h *Handler) RollbackPublish(c *gin.Context) {
	var req RollbackRequest
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

	resp, err := h.service.RollbackPublish(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// RegenerateProposal handles POST /api/versioning/pages/:pageKey/regenerate
func (h *Handler) RegenerateProposal(c *gin.Context) {
	var req RegenerateProposalRequest
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

	resp, err := h.service.RegenerateProposal(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// Republish handles POST /api/versioning/pages/:pageKey/republish
func (h *Handler) Republish(c *gin.Context) {
	var req RepublishRequest
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

	resp, err := h.service.Republish(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// getScope extracts game_id and env from context.
func getScope(c *gin.Context) (string, string) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	return scope.GameID, scope.Env
}
