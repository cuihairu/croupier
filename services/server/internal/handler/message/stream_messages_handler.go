// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package message

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/message"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 消息流（实时推送）
func StreamMessagesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isEventStreamRequest(r) {
			handleMessagesSSE(w, r, svcCtx)
			return
		}

		var req types.StreamMessagesRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := message.NewStreamMessagesLogic(r.Context(), svcCtx)
		resp, err := l.StreamMessages(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

func isEventStreamRequest(r *http.Request) bool {
	accept := strings.ToLower(r.Header.Get("Accept"))
	return strings.Contains(accept, "text/event-stream")
}

func handleMessagesSSE(w http.ResponseWriter, r *http.Request, svcCtx *svc.ServiceContext) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	headers := w.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	username, err := utils.CurrentUsername(ctx)
	if err != nil {
		// fall back to broadcasting messages if username missing
		username = ""
	}

	sendMessagesSnapshot := func() {
		l := message.NewStreamMessagesLogic(ctx, svcCtx)
		resp, err := l.StreamMessages(&types.StreamMessagesRequest{})
		if err != nil {
			logx.WithContext(ctx).Errorf("failed to load recent messages: %v", err)
			return
		}
		if resp != nil {
			writeSSE(w, flusher, "messages", resp.Data)
		}
	}

	sendUnreadCount := func() {
		count, err := svcCtx.MessageModel.CountUnread(ctx, username)
		if err != nil {
			logx.WithContext(ctx).Errorf("failed to load unread count: %v", err)
			return
		}
		writeSSE(w, flusher, "unread", map[string]interface{}{"count": count})
	}

	// send initial payloads
	sendUnreadCount()
	sendMessagesSnapshot()

	updateTicker := time.NewTicker(60 * time.Second)
	defer updateTicker.Stop()
	keepAliveTicker := time.NewTicker(60 * time.Second)
	defer keepAliveTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-updateTicker.C:
			sendUnreadCount()
			sendMessagesSnapshot()
		case <-keepAliveTicker.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, payload interface{}) {
	if event != "" {
		fmt.Fprintf(w, "event: %s\n", event)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		logx.Errorf("failed to encode SSE payload: %v", err)
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}
