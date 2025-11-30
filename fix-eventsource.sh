#!/bin/bash

# Post-build script to fix EventSource MIME type issue
echo "[post-build] Applying EventSource fix..."

HANDLER_FILE="services/server/internal/handler/message/stream_messages_handler.go"

# Create a backup of the original file
cp "$HANDLER_FILE" "${HANDLER_FILE}.backup"

# Create a fixed version
cat > "$HANDLER_FILE" << 'EOF'
// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package message

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/message"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 消息流（实时推送）- 支持 EventSource
func StreamMessagesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if this is an EventSource request by Accept header
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "text/event-stream") {
			// Handle as SSE
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

			// Send initial connection message
			fmt.Fprintf(w, "event: connect\ndata: {\"status\":\"connected\"}\n\n")

			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

			// Keep connection alive
			for {
				select {
				case <-r.Context().Done():
					return
				case <-time.After(30 * time.Second):
					// Send keep-alive ping
					fmt.Fprintf(w, ": keep-alive\n\n")
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
				}
			}
		}

		// Fallback to regular JSON handler
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
EOF

echo "[post-build] EventSource fix applied successfully"
EOF

chmod +x fix-eventsource.sh