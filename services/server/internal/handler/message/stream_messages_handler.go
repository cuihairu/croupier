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
	ctx := r.Context()
	username, err := utils.CurrentUsername(ctx)
	if err != nil {
		// fall back to broadcasting messages if username missing
		username = ""
	}

	// ✅ Phase 1: Complete all database operations BEFORE writing headers
	// This prevents "superfluous response.WriteHeader call" errors
	var initialMessages interface{}
	var initialUnreadCount int64
	var initErr error

	// Load initial messages
	l := message.NewStreamMessagesLogic(ctx, svcCtx)
	resp, err := l.StreamMessages(&types.StreamMessagesRequest{})
	if err != nil {
		initErr = fmt.Errorf("failed to load recent messages: %w", err)
		logx.WithContext(ctx).Errorf("failed to load recent messages: %v", err)
	} else if resp != nil {
		initialMessages = resp.Data
	}

	// Load unread count
	count, err := svcCtx.MessageModel.CountUnread(ctx, username)
	if err != nil {
		initErr = fmt.Errorf("failed to load unread count: %w", err)
		logx.WithContext(ctx).Errorf("failed to load unread count: %v", err)
	} else {
		initialUnreadCount = count
	}

	// ✅ Check if streaming is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// ✅ If database operations failed, return error BEFORE writing headers
	if initErr != nil {
		logx.WithContext(ctx).Errorf("SSE initialization failed: %v", initErr)
		http.Error(w, "failed to initialize message stream", http.StatusServiceUnavailable)
		return
	}

	// ✅ Phase 2: Now it's safe to write headers
	headers := w.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// ✅ Phase 3: Send initial data (already validated)
	if initialMessages != nil {
		writeSSE(w, flusher, "messages", initialMessages)
	}
	writeSSE(w, flusher, "unread", map[string]interface{}{"count": initialUnreadCount})

	// Helper functions for periodic updates
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

	// 从配置读取 SSE 间隔，使用默认值
	updateInterval := 2 * time.Second     // 默认 2 秒
	keepAliveInterval := 30 * time.Second // 默认 30 秒

	if svcCtx.Config.SSE.UpdateInterval > 0 {
		updateInterval = time.Duration(svcCtx.Config.SSE.UpdateInterval) * time.Second
	}
	if svcCtx.Config.SSE.KeepAliveInterval > 0 {
		keepAliveInterval = time.Duration(svcCtx.Config.SSE.KeepAliveInterval) * time.Second
	}

	// 推送间隔：每 N 秒推送一次消息更新
	updateTicker := time.NewTicker(updateInterval)
	defer updateTicker.Stop()

	// Keep-alive 间隔：每 N 秒发送 ping 保持连接
	keepAliveTicker := time.NewTicker(keepAliveInterval)
	defer keepAliveTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			logx.WithContext(ctx).Infof("SSE connection closed: %v", ctx.Err())
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
