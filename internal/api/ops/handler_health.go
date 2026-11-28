package ops

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) OpsHealthGet(c *gin.Context) {
	var req OpsHealthGetRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsHealthGet(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsHealthRun(c *gin.Context) {
	var req OpsHealthRunRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsHealthRun(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsHealthUpdate(c *gin.Context) {
	var req OpsHealthUpdateRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsHealthUpdate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) HealthGet(c *gin.Context) {
	h.OpsHealthGet(c)
}

func (h *Handler) HealthRun(c *gin.Context) {
	h.OpsHealthRun(c)
}

func (h *Handler) HealthUpdate(c *gin.Context) {
	h.OpsHealthUpdate(c)
}
