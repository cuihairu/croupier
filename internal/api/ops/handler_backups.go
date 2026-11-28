package ops

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) OpsBackupsList(c *gin.Context) {
	var req OpsBackupsListRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsBackupsList(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsBackupCreate(c *gin.Context) {
	var req OpsBackupCreateRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsBackupCreate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsBackupDelete(c *gin.Context) {
	var req OpsBackupDeleteRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsBackupDelete(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsBackupDownload(c *gin.Context) {
	var req OpsBackupDownloadRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsBackupDownload(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Alert operations handlers

func (h *Handler) BackupsList(c *gin.Context) {
	h.OpsBackupsList(c)
}

func (h *Handler) BackupCreate(c *gin.Context) {
	h.OpsBackupCreate(c)
}

func (h *Handler) BackupDelete(c *gin.Context) {
	h.OpsBackupDelete(c)
}

func (h *Handler) BackupDownload(c *gin.Context) {
	h.OpsBackupDownload(c)
}
