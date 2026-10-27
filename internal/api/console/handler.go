package console

import (
	"errors"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// Handler handles Console API requests.
type Handler struct {
	service *Service
}

// NewHandler creates a new Console Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Menu handles GET /api/v1/console/menu
func (h *Handler) Menu(c *gin.Context) {
	var req ConsoleMenuRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Menu(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Pages handles GET /api/v1/console/pages
func (h *Handler) Pages(c *gin.Context) {
	var req ConsolePagesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Pages(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Page handles GET /api/v1/console/pages/:pageKey
func (h *Handler) Page(c *gin.Context) {
	var req ConsolePageRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Page(c.Request.Context(), &req)
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

// ExecuteDraftCompositeBinding handles POST /api/v1/versioning/pages/composite/preview/execute
//
// 草稿预览执行：与组合页预览端点同形请求体，服务端重建草稿 spec（发布级校验）
// 后走与已发布页面相同的执行门禁（权限/审批/契约快照），让真实预览在发布前
// 也能安全执行——产物不落库，执行审计以草稿来源标记。
func (h *Handler) ExecuteDraftCompositeBinding(c *gin.Context) {
	var req ConsoleDraftCompositeExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	req.IdempotencyKey = normalizeIdempotencyKey(c.GetHeader("Idempotency-Key"))
	resp, err := h.service.ExecuteDraftCompositeBinding(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ExecuteBinding handles POST /api/v1/console/pages/:pageKey/bindings/:bindingId/execute
func (h *Handler) ExecuteBinding(c *gin.Context) {
	var req ConsoleExecuteBindingRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	req.IdempotencyKey = normalizeIdempotencyKey(c.GetHeader("Idempotency-Key"))
	resp, err := h.service.ExecuteBinding(c.Request.Context(), &req)
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
