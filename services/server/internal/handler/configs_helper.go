package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/cuihairu/croupier/services/api/internal/logic"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func writeConfigError(ctx context.Context, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, svc.ErrConfigNotFound):
		httpx.WriteJsonCtx(ctx, w, http.StatusNotFound, map[string]any{"message": "config not found"})
	case errors.Is(err, svc.ErrConfigVersionMissing):
		httpx.WriteJsonCtx(ctx, w, http.StatusNotFound, map[string]any{"message": "version not found"})
	case errors.Is(err, svc.ErrConfigVersionConflict):
		httpx.WriteJsonCtx(ctx, w, http.StatusConflict, map[string]any{"message": "version conflict"})
	case errors.Is(err, svc.ErrConfigInvalidInput), errors.Is(err, logic.ErrInvalidRequest):
		httpx.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]any{"message": "invalid request"})
	default:
		httpx.ErrorCtx(ctx, w, err)
	}
}
