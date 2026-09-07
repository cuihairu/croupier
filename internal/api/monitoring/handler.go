package monitoring

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// serviceAPI 是 Handler 对 Service 的最小接口缝隙：Service 三个方法恒返回
// nil error，err != nil 分支仅在测试注入故障实现时可触达。
type serviceAPI interface {
	Healthz(ctx context.Context, req *HealthzRequest) (*HealthzResponse, error)
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
