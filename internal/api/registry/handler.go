package registry

import (
	"github.com/cuihairu/croupier/internal/pkg2/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetRegistry retrieves registry information including agents, functions, and coverage
func (h *Handler) GetRegistry(c *gin.Context) {
	var req RegistryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// For GET requests, use empty request
		req = RegistryRequest{}
	}

	resp, err := h.service.GetRegistry(c.Request.Context(), &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}

	response.Success(c, resp)
}
