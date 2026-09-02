package release

import (
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

// NewHandler creates a release handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles GET /releases.
func (h *Handler) List(c *gin.Context) {
	var req ReleaseListRequest
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

// Create handles POST /releases.
func (h *Handler) Create(c *gin.Context) {
	var req ReleaseCreateRequest
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

// Transition handles POST /releases/:id/transition.
func (h *Handler) Transition(c *gin.Context) {
	var req ReleaseTransitionRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义
	if err := c.ShouldBindJSON(&req); err != nil {
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

// UploadArtifact handles POST /releases/:id/artifact (multipart package).
func (h *Handler) UploadArtifact(c *gin.Context) {
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

	manifest := []byte(c.PostForm("manifest"))
	req := &UploadArtifactRequest{
		ID:          uriReq.ID,
		Data:        file,
		Size:        header.Size,
		ContentType: header.Header.Get("Content-Type"),
		Manifest:    manifest,
	}
	if req.ContentType == "" || strings.HasPrefix(req.ContentType, "multipart/") {
		req.ContentType = "application/octet-stream"
	}
	resp, err := h.service.UploadArtifact(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// CheckUpdate handles POST /releases/check (client-facing, public).
func (h *Handler) CheckUpdate(c *gin.Context) {
	var req CheckUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.CheckUpdate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
