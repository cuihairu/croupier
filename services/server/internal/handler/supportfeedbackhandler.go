package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/api/internal/logic"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func SupportFeedbackListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SupportFeedbackQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewSupportFeedbackListLogic(r.Context(), svcCtx)
		resp, err := l.SupportFeedbackList(&req)
		if err != nil {
			writeSupportError(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

func SupportFeedbackCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SupportFeedbackCreateRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewSupportFeedbackCreateLogic(r.Context(), svcCtx)
		resp, err := l.SupportFeedbackCreate(&req)
		if err != nil {
			writeSupportError(r.Context(), w, err)
		} else {
			w.WriteHeader(http.StatusCreated)
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

func SupportFeedbackUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SupportFeedbackUpdateRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewSupportFeedbackUpdateLogic(r.Context(), svcCtx)
		if err := l.SupportFeedbackUpdate(&req); err != nil {
			writeSupportError(r.Context(), w, err)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

func SupportFeedbackDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SupportFeedbackDeleteRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewSupportFeedbackDeleteLogic(r.Context(), svcCtx)
		if err := l.SupportFeedbackDelete(&req); err != nil {
			writeSupportError(r.Context(), w, err)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}
