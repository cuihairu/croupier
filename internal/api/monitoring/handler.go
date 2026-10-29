package monitoring

import (
	"context"
	"net/http"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// serviceAPI 是 Handler 对 Service 的最小接口缝隙：Service 三个方法恒返回
// nil error，err != nil 分支仅在测试注入故障实现时可触达。
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

// Ready 处理就绪探针（/readyz）：就绪 200，未就绪 503。
// 健康探针契约（API response contract §4）允许探针端点直接写状态码与
// 最小负载——本端点按探针语义输出 status/reasons，不走业务信封。
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
