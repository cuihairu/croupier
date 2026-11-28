package ops

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) OpsMetrics(c *gin.Context) {
	var req OpsMetricsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsMetrics(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Config and notifications handlers

// SystemRuntime serves GET /api/v1/ops/system/runtime (system maintenance #52).
func (h *Handler) SystemRuntime(c *gin.Context) {
	response.Success(c, h.service.SystemRuntime(c.Request.Context()))
}

// SystemCheckUpdate serves POST /api/v1/ops/system/check-update (check only,
// no upgrade; the result carries version notes).
func (h *Handler) SystemCheckUpdate(c *gin.Context) {
	response.Success(c, h.service.SystemCheckUpdate(c.Request.Context()))
}

func (h *Handler) Metrics(c *gin.Context) {
	h.OpsMetrics(c)
}

// ClusterInfo handles GET /ops/cluster: the server instance topology (online /
// offline / agent distribution).
func (h *Handler) ClusterInfo(c *gin.Context) {
	resp, err := h.service.GetClusterInfo(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
