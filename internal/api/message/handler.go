package message

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/gin-gonic/gin"
)

func currentUsername(c *gin.Context) string {
	if u, ok := c.Get("username"); ok {
		if s, ok := u.(string); ok {
			return s
		}
	}
	return ""
}

type Handler struct {
	service *Service
	sse     config.SSEConfig
}

func NewHandler(service *Service, sse config.SSEConfig) *Handler {
	return &Handler{service: service, sse: sse}
}

// List handles the request to list messages
func (h *Handler) List(c *gin.Context) {
	var req MessagesListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.List(c.Request.Context(), currentUsername(c), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Send handles the request to send a message
//
// Sending (including point-to-point single send) is a back-office operation
// capability, at the same gate as broadcast: only the admin role may call it.
// Previously there was no permission check at all here, so any logged-in user
// could deliver in-app messages to an arbitrary account (docs/BUGS.md BUG-026).
// The receiving side (List/Detail/Read/UnreadCount/Stream) stays available to
// every logged-in user.
func (h *Handler) Send(c *gin.Context) {
	if !isBroadcaster(c, h) {
		response.Forbidden(c, "仅管理员可发送站内消息")
		return
	}
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
	_ = c.ShouldBindUri(&req) // all uri fields are strings without required: binding cannot fail, keep fill-in semantics

	resp, err := h.service.Detail(c.Request.Context(), currentUsername(c), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Read handles the request to mark a message as read
func (h *Handler) Read(c *gin.Context) {
	var req MessageReadRequest
	_ = c.ShouldBindUri(&req) // all uri fields are strings without required: binding cannot fail, keep fill-in semantics

	resp, err := h.service.Read(c.Request.Context(), currentUsername(c), &req)
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

	resp, err := h.service.UnreadCount(c.Request.Context(), currentUsername(c), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Stream handles the SSE stream for messages.
func (h *Handler) Stream(c *gin.Context) {
	username := currentUsername(c)
	if username == "" {
		response.Unauthorized(c, "未授权")
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Status(http.StatusOK)

	// Use the request context — gin cancels it when the client disconnects.
	ctx := c.Request.Context()

	ticker := time.NewTicker(time.Duration(h.sse.GetUpdateInterval()) * time.Second)
	defer ticker.Stop()

	h.sendMessagesEvent(c, username)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.sendMessagesEvent(c, username)
		}
	}
}

// marshalMessagesEvent is the injectable seam for SSE message event
// serialization (the production implementation is json.Marshal). Event
// payloads come from a JSON round-trip, so the marshal-failure branch can only
// be reached via test injection.
var marshalMessagesEvent = json.Marshal

func (h *Handler) sendMessagesEvent(c *gin.Context, username string) {
	resp, err := h.service.Stream(c.Request.Context(), username, &StreamMessagesRequest{})
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "SSE message stream error", "error", err)
		return
	}
	data, err := marshalMessagesEvent(resp)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(c.Writer, "event: messages\ndata: %s\n\n", data)
	c.Writer.Flush()
}

// Get alias for route compatibility
func (h *Handler) Get(c *gin.Context) {
	h.Detail(c)
}

// Broadcast serves POST /messages/broadcast: admins broadcast in-app
// messages (everyone / by role / named users). Only the admin role may call it.
func (h *Handler) Broadcast(c *gin.Context) {
	if !isBroadcaster(c, h) {
		response.Forbidden(c, "仅管理员可群发消息")
		return
	}
	var req BroadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Broadcast(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// isBroadcaster verifies the current user has the admin role (sending is a
// back-office operation capability: broadcast and point-to-point single send
// share the same gate, ordinary users only have receiving semantics).
func isBroadcaster(c *gin.Context, h *Handler) bool {
	admin, roles, err := utils.LoadCurrentAdmin(c.Request.Context(), h.service.svcCtx)
	if err != nil {
		return false
	}
	_ = admin
	for _, r := range roles {
		if r.Name == "admin" {
			return true
		}
	}
	return false
}
