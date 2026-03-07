package workspace

import (
	"context"
	"net/http"
	"strings"
)

const workspaceRequestIDKey = "workspaceRequestID"

func withWorkspaceRequestID(ctx context.Context, requestID string) context.Context {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, workspaceRequestIDKey, requestID)
}

func resolveRequestIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if id := strings.TrimSpace(r.Header.Get("X-Request-Id")); id != "" {
		return id
	}
	if id := strings.TrimSpace(r.Header.Get("X-Trace-Id")); id != "" {
		return id
	}
	return ""
}
