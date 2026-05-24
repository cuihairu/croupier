package workspace

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
	"strings"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListConfigs handles the request to list all workspace configurations
func (h *Handler) ListConfigs(c *gin.Context) {
	var req ListConfigsRequest
	resp, err := h.service.ListConfigs(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ListPublished handles the request to list published workspace configurations
func (h *Handler) ListPublished(c *gin.Context) {
	var req ListPublishedRequest
	resp, err := h.service.ListPublished(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// GetConfig handles the request to get a workspace configuration
func (h *Handler) GetConfig(c *gin.Context) {
	var req GetConfigRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.GetConfig(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// SaveConfig handles the request to save a workspace configuration
func (h *Handler) SaveConfig(c *gin.Context) {
	var req SaveConfigRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.SaveConfig(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// DeleteConfig handles the request to delete a workspace configuration
func (h *Handler) DeleteConfig(c *gin.Context) {
	var req DeleteConfigRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.DeleteConfig(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Publish handles the request to publish a workspace configuration
func (h *Handler) Publish(c *gin.Context) {
	var req PublishRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	// PublishedBy is optional, bind JSON if body exists
	if c.Request.ContentLength > 0 {
		var bodyReq struct {
			PublishedBy string `json:"publishedBy"`
		}
		if err := c.ShouldBindJSON(&bodyReq); err == nil {
			req.PublishedBy = bodyReq.PublishedBy
		}
	}

	resp, err := h.service.Publish(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Unpublish handles the request to unpublish a workspace configuration
func (h *Handler) Unpublish(c *gin.Context) {
	var req UnpublishRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Unpublish(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Versions handles the request to list workspace versions
func (h *Handler) Versions(c *gin.Context) {
	var req VersionsRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Versions(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// VersionDetail handles the request to get workspace version detail
func (h *Handler) VersionDetail(c *gin.Context) {
	var req VersionDetailRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.VersionDetail(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Rollback handles the request to rollback a workspace to a specific version
func (h *Handler) Rollback(c *gin.Context) {
	var uriReq struct {
		ObjectKey string `uri:"objectKey" binding:"required"`
	}
	if err := c.ShouldBindUri(&uriReq); err != nil {
		response.Error(c, err)
		return
	}
	req := RollbackRequest{ObjectKey: uriReq.ObjectKey}
	if c.Request.ContentLength > 0 {
		var bodyReq struct {
			VersionID string `json:"versionId"`
		}
		if err := c.ShouldBindJSON(&bodyReq); err != nil {
			response.Error(c, err)
			return
		}
		if strings.TrimSpace(bodyReq.VersionID) != "" {
			req.VersionID = bodyReq.VersionID
		}
	}

	resp, err := h.service.Rollback(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
