package ops

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) OpsAlerts(c *gin.Context) {
	var req OpsAlertsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsAlerts(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsAlertSilence(c *gin.Context) {
	var req OpsAlertSilenceRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsAlertSilence(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// OpsAlertHandle handles POST /api/v1/ops/alerts/:id/handle.
// The path parameter :id is read straight from c.Param in the handler
// (BindQueryCompat only binds query, and the uri tag cannot be relied on — the
// same lesson as /nodes/:id/meta); the body carries only action/remark.
func (h *Handler) OpsAlertHandle(c *gin.Context) {
	var req OpsAlertHandleRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsAlertHandle(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsSilenceDelete(c *gin.Context) {
	var req OpsAlertSilenceRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsSilenceDelete(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsSilences(c *gin.Context) {
	var req OpsSilencesRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsSilences(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Node operations handlers

func (h *Handler) Alerts(c *gin.Context) {
	h.OpsAlerts(c)
}

func (h *Handler) AlertSilence(c *gin.Context) {
	h.OpsAlertSilence(c)
}

func (h *Handler) SilenceDelete(c *gin.Context) {
	h.OpsSilenceDelete(c)
}

func (h *Handler) Silences(c *gin.Context) {
	h.OpsSilences(c)
}
