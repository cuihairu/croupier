package monitoring

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Healthz handles health check endpoint
func (h *Handler) Healthz(c *gin.Context) {
	var req HealthzRequest
	resp, err := h.service.Healthz(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Metrics handles system metrics endpoint
func (h *Handler) Metrics(c *gin.Context) {
	var req MetricsRequest
	resp, err := h.service.Metrics(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Status handles system status endpoint
func (h *Handler) Status(c *gin.Context) {
	var req StatusRequest
	resp, err := h.service.Status(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
