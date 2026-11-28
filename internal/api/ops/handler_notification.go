package ops

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) OpsNotificationsGet(c *gin.Context) {
	var req OpsNotificationsGetRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsNotificationsGet(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsNotificationsUpdate(c *gin.Context) {
	var req OpsNotificationsUpdateRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsNotificationsUpdate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Services and functions handlers

func (h *Handler) NotificationsGet(c *gin.Context) {
	h.OpsNotificationsGet(c)
}

func (h *Handler) NotificationsUpdate(c *gin.Context) {
	h.OpsNotificationsUpdate(c)
}
