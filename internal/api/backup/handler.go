package backup

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles the request to list backups
func (h *Handler) List(c *gin.Context) {
	var req BackupsListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.List(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Create handles the request to create a backup
func (h *Handler) Create(c *gin.Context) {
	var req BackupCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Delete handles the request to delete a backup
func (h *Handler) Delete(c *gin.Context) {
	var req BackupDeleteRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.Delete(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "操作成功"})
}

// Download handles the request to download a backup
func (h *Handler) Download(c *gin.Context) {
	var req BackupDownloadRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	payload, err := h.service.Download(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	if payload.RedirectURL != "" {
		c.Redirect(302, payload.RedirectURL)
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+payload.Filename)
	c.Header("Content-Type", "application/octet-stream")
	if payload.Reader != nil {
		c.DataFromReader(200, payload.Size, "application/octet-stream", payload.Reader.(interface{ Read([]byte) (int, error) }), nil)
	}
}
