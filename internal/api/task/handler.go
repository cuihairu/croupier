package task

import (
	"strconv"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

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

func (h *Handler) Start(c *gin.Context) {
	var req StartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Start(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Detail(c *gin.Context) {
	var req DetailRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义
	resp, err := h.service.Detail(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Events(c *gin.Context) {
	var req EventsRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}
	// L3 SDKs use snake_case query names to match the public Server HTTP
	// contract. Keep camelCase compatibility for existing console callers.
	if raw, ok := c.GetQuery("after_seq"); ok {
		afterSeq, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || afterSeq < 0 {
			response.BadRequest(c, "after_seq 必须是非负整数")
			return
		}
		req.AfterSeq = afterSeq
	}
	resp, err := h.service.Events(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Cancel(c *gin.Context) {
	var req CancelRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义
	if err := h.service.Cancel(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "操作成功"})
}

func (h *Handler) CancelByBody(c *gin.Context) {
	var req CancelBodyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := h.service.Cancel(c.Request.Context(), &CancelRequest{ID: req.ID}); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "操作成功"})
}
