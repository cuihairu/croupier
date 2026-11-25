package monitoring

import (
	"context"
	"net/http"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// serviceAPI is the minimal interface seam between Handler and Service: the
// three Service methods always return a nil error, so the err != nil branch is
// only reachable by injecting a failing implementation in tests.
type serviceAPI interface {
	Healthz(ctx context.Context, req *HealthzRequest) (*HealthzResponse, error)
	Ready(ctx context.Context, req *ReadyRequest) (*ReadyResponse, error)
	Metrics(ctx context.Context, req *MetricsRequest) (*MetricsResponse, error)
	Status(ctx context.Context, req *StatusRequest) (*StatusResponse, error)
}

type Handler struct {
	service serviceAPI
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

// Ready handles the readiness probe (/readyz): 200 when ready, 503 when not.
// The health probe contract (API response contract §4) lets probe endpoints
// write status codes and a minimal payload directly — this endpoint emits
// status/reasons with probe semantics instead of the business envelope.
func (h *Handler) Ready(c *gin.Context) {
	var req ReadyRequest
	resp, err := h.service.Ready(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	status := http.StatusOK
	if resp.Status != "ok" {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, resp)
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
