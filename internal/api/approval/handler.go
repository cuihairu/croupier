package approval

import (
	"errors"
	"io"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles the request to list approvals
func (h *Handler) List(c *gin.Context) {
	var req ApprovalsListRequest
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

// Get handles the request to get approval details
func (h *Handler) Get(c *gin.Context) {
	var req ApprovalGetRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义

	resp, err := h.service.Get(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Approve handles the request to approve an approval
func (h *Handler) Approve(c *gin.Context) {
	var req ApprovalApproveRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义
	if err := bindApproveBody(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Approve(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Reject handles the request to reject an approval
func (h *Handler) Reject(c *gin.Context) {
	var req ApprovalRejectRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Reject(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// bindApproveBody 容忍性读批准请求体：Web 无 otp 时 data 为 undefined 不发
// body（Content-Length 0），直接 ShouldBindJSON 会以 EOF 报 400——空体放行，
// otp 留空交由 service 按策略判定；有 body 但解不出 JSON 仍按契约 400。
func bindApproveBody(c *gin.Context, req *ApprovalApproveRequest) error {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return nil
	}
	if err := c.ShouldBindJSON(req); err != nil {
		// chunked 传输无 Content-Length，空体表现为 EOF：同样放行
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}
		return err
	}
	return nil
}
