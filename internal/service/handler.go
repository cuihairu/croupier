package service

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

// Handler provides HTTP handlers for Contract API.
type Handler struct {
	service *ContractService
}

// NewContractHandler creates a new handler.
func NewContractHandler(service *ContractService) *Handler {
	return &Handler{service: service}
}

// ListContracts handles GET /api/contracts
func (h *Handler) ListContracts(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())

	resp, err := h.service.ListContracts(c.Request.Context(), scope.GameID, scope.Env)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// GetContract handles GET /api/contracts/:functionId
func (h *Handler) GetContract(c *gin.Context) {
	functionID := c.Param("functionId")
	scope := svc.GameScopeFromContext(c.Request.Context())

	resp, err := h.service.GetContract(c.Request.Context(), scope.GameID, scope.Env, functionID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// ListResourceCapabilities handles GET /api/resource-capabilities
func (h *Handler) ListResourceCapabilities(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())

	resp, err := h.service.ListResourceCapabilities(c.Request.Context(), scope.GameID, scope.Env)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}
