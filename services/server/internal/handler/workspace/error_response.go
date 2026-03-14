package workspace

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
)

func writeWorkspaceError(w http.ResponseWriter, r *http.Request, err error, operation string) {
	status := http.StatusInternalServerError
	message := "workspace operation failed"
	errorCode := "internal_error"

	var codeErr *errorx.CodeError
	if errors.As(err, &codeErr) {
		status = codeErr.Code
		message = codeErr.Message
		errorCode = codeErr.ErrorCode()
	} else if err != nil && err.Error() != "" {
		message = err.Error()
	}

	code := mapWorkspaceErrorCode(status, errorCode, operation)
	requestID := resolveRequestID(r)
	if err != nil {
		if status >= http.StatusInternalServerError {
			slog.LogAttrs(r.Context(), slog.LevelError, "workspace request failed",
				slog.String("operation", operation),
				slog.Int("status", status),
				slog.String("code", code),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("request_id", requestID),
				slog.String("error", err.Error()),
			)
		} else {
			slog.LogAttrs(r.Context(), slog.LevelInfo, "workspace request rejected",
				slog.String("operation", operation),
				slog.Int("status", status),
				slog.String("code", code),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("request_id", requestID),
				slog.String("error", err.Error()),
			)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":       code,
		"error":      code,
		"message":    message,
		"request_id": requestID,
	})
}

func mapWorkspaceErrorCode(status int, rawCode, operation string) string {
	if operation == "rollback" || operation == "versions_detail" {
		if status == http.StatusNotFound || status == http.StatusBadRequest || status == http.StatusUnprocessableEntity {
			return "workspace_version_not_found"
		}
	}

	switch status {
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "workspace_not_found"
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return "workspace_invalid_config"
	}

	if operation == "publish" || operation == "unpublish" {
		return "workspace_publish_failed"
	}
	if rawCode != "" {
		return rawCode
	}
	return "internal_error"
}

func resolveRequestID(r *http.Request) string {
	if r == nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	if id := r.Header.Get("X-Request-Id"); id != "" {
		return id
	}
	if id := r.Header.Get("X-Trace-Id"); id != "" {
		return id
	}
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}
