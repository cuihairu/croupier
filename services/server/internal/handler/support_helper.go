package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func writeSupportError(ctx context.Context, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, logic.ErrInvalidRequest):
		httpx.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"message": "invalid request"})
	case errors.Is(err, svc.ErrSupportTicketNotFound),
		errors.Is(err, svc.ErrSupportFAQNotFound),
		errors.Is(err, svc.ErrSupportFeedbackNotFound):
		httpx.WriteJsonCtx(ctx, w, http.StatusNotFound, map[string]string{"message": "not found"})
	default:
		httpx.ErrorCtx(ctx, w, err)
	}
}
