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
	// uri fields are all string and not required: binding cannot fail, so keep
	// the fill semantics
	_ = c.ShouldBindUri(&req)

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
	// uri fields are all string and not required: binding cannot fail, so keep
	// the fill semantics
	_ = c.ShouldBindUri(&req)
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
	// uri fields are all string and not required: binding cannot fail, so keep
	// the fill semantics
	_ = c.ShouldBindUri(&req)
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

// bindApproveBody reads the approve request body tolerantly: when Web has no
// otp the data is undefined and no body is sent (Content-Length 0), so a
// direct ShouldBindJSON would fail with EOF and report 400 — let the empty
// body through, leave otp empty and let the service decide by policy; a
// present body that does not parse as JSON still yields 400 per contract.
func bindApproveBody(c *gin.Context, req *ApprovalApproveRequest) error {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return nil
	}
	if err := c.ShouldBindJSON(req); err != nil {
		// chunked transfer has no Content-Length and an empty body shows up as
		// EOF: let it through as well
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}
		return err
	}
	return nil
}
