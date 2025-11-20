package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func SupportCommentsListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SupportTicketUpdateRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewSupportCommentsListLogic(r.Context(), svcCtx)
		resp, err := l.SupportCommentsList(&req)
		if err != nil {
			writeSupportError(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

func SupportCommentCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SupportCommentCreateRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewSupportCommentCreateLogic(r.Context(), svcCtx)
		resp, err := l.SupportCommentCreate(&req)
		if err != nil {
			writeSupportError(r.Context(), w, err)
		} else {
			w.WriteHeader(http.StatusCreated)
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
