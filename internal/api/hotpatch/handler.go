package hotpatch

import (
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

// NewHandler creates a hotpatch handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles GET /hotpatches.
func (h *Handler) List(c *gin.Context) {
	var req ListRequest
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

// Create handles POST /hotpatches.
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
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

// UploadPackage handles POST /hotpatches/:id/package (multipart).
func (h *Handler) UploadPackage(c *gin.Context) {
	var uriReq struct {
		ID string `uri:"id"`
	}
	if err := c.ShouldBindUri(&uriReq); err != nil {
		response.Error(c, err)
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.Error(c, errorx.NewBadRequest("缺少 file 字段"))
		return
	}
	defer file.Close()
	req := &UploadRequest{
		ID:          uriReq.ID,
		Data:        file,
		Size:        header.Size,
		ContentType: header.Header.Get("Content-Type"),
	}
	if req.ContentType == "" || strings.HasPrefix(req.ContentType, "multipart/") {
		req.ContentType = "application/octet-stream"
	}
	resp, err := h.service.UploadPackage(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Transition handles POST /hotpatches/:id/transition.
func (h *Handler) Transition(c *gin.Context) {
	var req TransitionRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义
	if err := c.ShouldBindJSON(&req); err != nil && c.Request.ContentLength > 0 {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Transition(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ReportResult handles POST /hotpatches/:id/results.
func (h *Handler) ReportResult(c *gin.Context) {
	var req ResultRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := h.service.ReportResult(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "已记录"})
}
