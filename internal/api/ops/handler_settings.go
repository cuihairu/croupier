package ops

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) OpsMaintenanceGet(c *gin.Context) {
	var req OpsMaintenanceGetRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsMaintenanceGet(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsMaintenanceUpdate(c *gin.Context) {
	var req OpsMaintenanceUpdateRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsMaintenanceUpdate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Metrics and monitoring handlers

func (h *Handler) OpsConfig(c *gin.Context) {
	var req OpsConfigRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsConfig(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsFunctions(c *gin.Context) {
	var req OpsFunctionsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsFunctions(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// MQ handler

func (h *Handler) MaintenanceGet(c *gin.Context) {
	h.OpsMaintenanceGet(c)
}

func (h *Handler) MaintenanceUpdate(c *gin.Context) {
	h.OpsMaintenanceUpdate(c)
}

func (h *Handler) Functions(c *gin.Context) {
	h.OpsFunctions(c)
}

func (h *Handler) Config(c *gin.Context) {
	h.OpsConfig(c)
}
