package monitoring

import (
	"context"
	"net/http"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// serviceAPI 是 Handler 对 Service 的最小接口缝隙：Service 方法恒返回
// nil error，err != nil 分支仅在测试注入故障实现时可触达。
type serviceAPI interface {
	Healthz(ctx context.Context, req *HealthzRequest) (*HealthzResponse, error)
	Readiness(ctx context.Context, req *ReadinessRequest) (*ReadinessResponse, error)
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

// Readiness handles the readiness probe endpoint: 200 ready（degraded 仍
// 200 且带 degraded=true）、503 not-ready。就绪探针按响应契约豁免走最小
// 载荷直写（CLAUDE.md API Response Contract §4），不经统一信封。
func (h *Handler) Readiness(c *gin.Context) {
	var req ReadinessRequest
	resp, err := h.service.Readiness(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"ready":   false,
			"reasons": []string{err.Error()},
		})
		return
	}
	if !resp.Ready {
		c.JSON(http.StatusServiceUnavailable, resp)
		return
	}
	c.JSON(http.StatusOK, resp)
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
