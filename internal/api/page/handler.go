package page

import (
	"errors"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// Handler handles Page API requests.
type Handler struct {
	service *Service
}

// NewHandler creates a new Page Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListDrafts handles GET /api/v1/pages
func (h *Handler) ListDrafts(c *gin.Context) {
	var req PageDraftListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.ListDrafts(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// GetDraft handles GET /api/v1/pages/:pageKey
func (h *Handler) GetDraft(c *gin.Context) {
	var req PageDraftRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.GetDraft(c.Request.Context(), &req)
	if err != nil {
		var notFound *PageNotFoundError
		if errors.As(err, &notFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// SaveDraft handles PUT /api/v1/pages/:pageKey
func (h *Handler) SaveDraft(c *gin.Context) {
	var req PageSaveRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.SaveDraft(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// RegenerateDraft handles POST /api/v1/pages/:pageKey/regenerate
func (h *Handler) RegenerateDraft(c *gin.Context) {
	var req PageRegenerateRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.RegenerateDraft(c.Request.Context(), &req)
	if err != nil {
		var notFound *PageNotFoundError
		if errors.As(err, &notFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// RebuildProposals handles POST /api/v1/pages/proposals/rebuild
func (h *Handler) RebuildProposals(c *gin.Context) {
	resp, err := h.service.RebuildAllProposals(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Validate handles POST /api/v1/pages/:pageKey/validate
func (h *Handler) Validate(c *gin.Context) {
	var req PageValidateRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Validate(c.Request.Context(), &req)
	if err != nil {
		var notFound *PageNotFoundError
		if errors.As(err, &notFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Preview handles POST /api/v1/pages/:pageKey/preview
func (h *Handler) Preview(c *gin.Context) {
	var req PagePreviewRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Preview(c.Request.Context(), &req)
	if err != nil {
		var notFound *PageNotFoundError
		if errors.As(err, &notFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Publish handles POST /api/v1/pages/:pageKey/publish
func (h *Handler) Publish(c *gin.Context) {
	var req PagePublishRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Publish(c.Request.Context(), &req)
	if err != nil {
		var notFound *PageNotFoundError
		if errors.As(err, &notFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Unpublish handles POST /api/v1/pages/:pageKey/unpublish
func (h *Handler) Unpublish(c *gin.Context) {
	var req PageUnpublishRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Unpublish(c.Request.Context(), &req)
	if err != nil {
		var notFound *PageNotFoundError
		if errors.As(err, &notFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Versions handles GET /api/v1/pages/:pageKey/versions
func (h *Handler) Versions(c *gin.Context) {
	var req PageVersionsRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	// limit/offset 走 query string，需要单独绑定
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Versions(c.Request.Context(), &req)
	if err != nil {
		var notFound *PageNotFoundError
		if errors.As(err, &notFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// VersionDetail handles GET /api/v1/pages/:pageKey/versions/:versionId
func (h *Handler) VersionDetail(c *gin.Context) {
	var req PageVersionDetailRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.VersionDetail(c.Request.Context(), &req)
	if err != nil {
		var notFound *PageNotFoundError
		if errors.As(err, &notFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Rollback handles POST /api/v1/pages/:pageKey/rollback
func (h *Handler) Rollback(c *gin.Context) {
	var req PageRollbackRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Rollback(c.Request.Context(), &req)
	if err != nil {
		var notFound *PageNotFoundError
		if errors.As(err, &notFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
