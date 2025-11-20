package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func writeApprovalsError(ctx context.Context, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, logic.ErrInvalidRequest):
		httpx.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"message": "invalid request"})
	case errors.Is(err, logic.ErrNotFound):
		httpx.WriteJsonCtx(ctx, w, http.StatusNotFound, map[string]string{"message": "not found"})
	case errors.Is(err, logic.ErrUnavailable):
		httpx.WriteJsonCtx(ctx, w, http.StatusServiceUnavailable, map[string]string{"message": "service unavailable"})
	default:
		httpx.ErrorCtx(ctx, w, err)
	}
}
