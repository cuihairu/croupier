package page

import (
	"errors"
	"io"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// 测试注入缝隙：默认值与生产行为完全一致，仅用于在测试中注入
// *PageNotFoundError 以覆盖 handler 的 NotFound 分支
// （service.Versions/VersionDetail 真实路径不返回该错误类型）。
var (
	serviceVersionsFn      = (*Service).Versions
	serviceVersionDetailFn = (*Service).VersionDetail
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
	// ShouldBindQuery 恒成功（A 类删除原 err 分支）：PageDraftListRequest
	// 仅含 form 绑定的 string 字段且无 binding 约束，gin form 绑定对
	// string 类型不存在类型转换或校验失败路径，任何 query 输入均可绑定。
	var req PageDraftListRequest

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

// SetMenu handles PUT /api/v1/pages/:pageKey/menu — 挂载/解除页面与菜单的
// 关联（menuId 为 null 或 0 表示解除）。
func (h *Handler) SetMenu(c *gin.Context) {
	var req PageMenuUpdateRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.SetPageMenu(c.Request.Context(), &req)
	if err != nil {
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

// SyncSelectors handles POST /api/v1/pages/:pageKey/sync-selectors
func (h *Handler) SyncSelectors(c *gin.Context) {
	var req PageSyncSelectorsRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.SyncSelectors(c.Request.Context(), &req)
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

	resp, err := serviceVersionsFn(h.service, c.Request.Context(), &req)
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

	resp, err := serviceVersionDetailFn(h.service, c.Request.Context(), &req)
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

// BulkPublish 处理一键发布全部 ready/basic 提案。
func (h *Handler) BulkPublish(c *gin.Context) {
	resp, err := h.service.BulkPublish(c.Request.Context(), &PageBulkRequest{})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// BulkUnpublish 处理一键下架全部已发布页面。
func (h *Handler) BulkUnpublish(c *gin.Context) {
	resp, err := h.service.BulkUnpublish(c.Request.Context(), &PageBulkRequest{})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// BulkRepublish 处理契约变更队列的一键重新发布。body 可为空（此时处理
// scope 内全部 stale 已发布页面），因此容忍空请求体的 io.EOF。
func (h *Handler) BulkRepublish(c *gin.Context) {
	req := &PageBulkRepublishRequest{}
	if err := c.ShouldBindJSON(req); err != nil && !errors.Is(err, io.EOF) {
		response.Error(c, err)
		return
	}
	resp, err := h.service.BulkRepublish(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// SeedDemoData 处理演示数据填充（Terms/发布页面/注册警告）。
func (h *Handler) SeedDemoData(c *gin.Context) {
	resp, err := h.service.SeedDemoData(c.Request.Context(), &PageSeedDemoRequest{})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
