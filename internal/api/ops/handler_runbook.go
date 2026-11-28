package ops

import (
	"net/http"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/ops/runbook"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

// RunbookCreateHandler POST /ops/runbooks — define a runbook (OPS-009).
func (h *Handler) RunbookCreateHandler(c *gin.Context) {
	var req runbook.DefineRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	scope := svc.GameScopeFromContext(c.Request.Context())
	if req.GameID == "" {
		req.GameID = scope.GameID
	}
	if req.Env == "" {
		req.Env = scope.Env
	}
	// a non-empty gameId/env in the body must match the authenticated scope (fail-closed).
	if err := enforceRequestScope(c.Request.Context(), req.GameID, req.Env); err != nil {
		response.Error(c, err)
		return
	}
	if username, err := utils.CurrentUsername(c.Request.Context()); err == nil {
		req.CreatedBy = username
	}
	rb, err := h.service.RunbookCreate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, rb)
}

// RunbookListHandler GET /ops/runbooks — list recent runbooks.
func (h *Handler) RunbookListHandler(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	var req RunbookListRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	items, err := h.service.RunbookList(c.Request.Context(), scope.GameID, scope.Env, req.Limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, items)
}

// RunbookGetHandler GET /ops/runbooks/:id — fetch one runbook.
func (h *Handler) RunbookGetHandler(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	rb, err := h.service.RunbookGet(c.Request.Context(), scope.GameID, scope.Env, c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, rb)
}

// RunbookDeleteHandler DELETE /ops/runbooks/:id — remove a runbook.
func (h *Handler) RunbookDeleteHandler(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	if err := h.service.RunbookDelete(c.Request.Context(), scope.GameID, scope.Env, c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// RunbookRunHandler POST /ops/runbooks/:id/run — one-click execution.
func (h *Handler) RunbookRunHandler(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	var req RunbookRunRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	operator := ""
	if username, err := utils.CurrentUsername(c.Request.Context()); err == nil {
		operator = username
	}
	res, err := h.service.RunbookRun(c.Request.Context(), scope.GameID, scope.Env, c.Param("id"), req.Params, operator)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// RunbookRunGetHandler GET /ops/runbook-runs/:runId — fetch one run.
func (h *Handler) RunbookRunGetHandler(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	run, err := h.service.RunbookRunGet(c.Request.Context(), scope.GameID, scope.Env, c.Param("runId"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, run)
}

// RunbookRunListHandler GET /ops/runbook-runs — list runs of a runbook.
func (h *Handler) RunbookRunListHandler(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	var req RunbookRunListRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	runs, err := h.service.RunbookRunList(c.Request.Context(), scope.GameID, scope.Env, req.RunbookID, req.Limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, runs)
}
