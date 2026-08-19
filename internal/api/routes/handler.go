package routes

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// routesService decouples the handler from the concrete Service so tests can
// inject failures (the real Service is a static catalog that never errors).
type routesService interface {
	GetRoutes(ctx context.Context) (*GetRoutesResponse, error)
}

type Handler struct {
	service routesService
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetRoutes returns the available routes
func (h *Handler) GetRoutes(c *gin.Context) {
	resp, err := h.service.GetRoutes(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
