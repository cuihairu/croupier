package ops

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/ops/batchop"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

// BatchOpCreateHandler POST /ops/batch — create a batch (dry-run by default).
func (h *Handler) BatchOpCreateHandler(c *gin.Context) {
	var req batchop.CreateRequest
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
	// a non-empty gameId/env in the body must match the authenticated scope, so
	// a scope-restricted user cannot create / dispatch batches to an
	// unauthorized game / environment (fail-closed).
	if err := enforceRequestScope(c.Request.Context(), req.GameID, req.Env); err != nil {
		response.Error(c, err)
		return
	}
	if username, err := utils.CurrentUsername(c.Request.Context()); err == nil {
		req.CreatedBy = username
	}
	op, err := h.service.BatchOpCreate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, op)
}

// BatchOpExecuteHandler POST /ops/batch/:id/execute — execute (or preview).
func (h *Handler) BatchOpExecuteHandler(c *gin.Context) {
	var req BatchOpExecuteRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	scope := svc.GameScopeFromContext(c.Request.Context())
	res, err := h.service.BatchOpExecute(c.Request.Context(), scope.GameID, scope.Env, c.Param("id"), req.DryRun)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// BatchOpGetHandler GET /ops/batch/:id — fetch one batch.
func (h *Handler) BatchOpGetHandler(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	op, err := h.service.BatchOpGet(c.Request.Context(), scope.GameID, scope.Env, c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, op)
}

// BatchOpListHandler GET /ops/batch — list recent batches.
func (h *Handler) BatchOpListHandler(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	var req BatchOpListRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	ops, err := h.service.BatchOpList(c.Request.Context(), scope.GameID, scope.Env, req.Limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, ops)
}
