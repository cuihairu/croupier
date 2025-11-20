package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/cuihairu/croupier/services/api/internal/logic"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func writeGameError(ctx context.Context, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, svc.ErrGameNotFound):
		httpx.WriteJsonCtx(ctx, w, http.StatusNotFound, map[string]any{"message": "game not found"})
	case errors.Is(err, svc.ErrGameInvalidEnv):
		httpx.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]any{"message": "invalid env"})
	case errors.Is(err, logic.ErrInvalidRequest):
		httpx.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]any{"message": "invalid request"})
	default:
		httpx.ErrorCtx(ctx, w, err)
	}
}
