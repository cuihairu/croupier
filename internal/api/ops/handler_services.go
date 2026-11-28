package ops

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) OpsServices(c *gin.Context) {
	var req OpsServicesRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsServices(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsMQ(c *gin.Context) {
	var req OpsMQRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsMQ(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Alias methods for route compatibility

func (h *Handler) MQ(c *gin.Context) {
	h.OpsMQ(c)
}

func (h *Handler) Services(c *gin.Context) {
	h.OpsServices(c)
}
