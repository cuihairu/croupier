package message

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

// List handles the request to list messages
func (h *Handler) List(c *gin.Context) {
	var req MessagesListRequest
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

// Send handles the request to send a message
func (h *Handler) Send(c *gin.Context) {
	var req MessageSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Send(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Detail handles the request to get message details
func (h *Handler) Detail(c *gin.Context) {
	var req MessageDetailRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Detail(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Read handles the request to mark a message as read
func (h *Handler) Read(c *gin.Context) {
	var req MessageReadRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Read(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// UnreadCount handles the request to get unread message count
func (h *Handler) UnreadCount(c *gin.Context) {
	var req MessagesUnreadCountRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.UnreadCount(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Stream handles the request to stream messages
func (h *Handler) Stream(c *gin.Context) {
	var req StreamMessagesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Stream(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Get alias for route compatibility
func (h *Handler) Get(c *gin.Context) {
	h.Detail(c)
}
