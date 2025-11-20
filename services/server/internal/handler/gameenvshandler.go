package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/api/internal/logic"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GameEnvsListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GameDetailRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewGameEnvsListLogic(r.Context(), svcCtx)
		resp, err := l.GameEnvsList(&req)
		if err != nil {
			writeGameError(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

func GameEnvAddHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GameEnvAddRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewGameEnvAddLogic(r.Context(), svcCtx)
		if err := l.GameEnvAdd(&req); err != nil {
			writeGameError(r.Context(), w, err)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

func GameEnvUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GameEnvUpdateRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewGameEnvUpdateLogic(r.Context(), svcCtx)
		if err := l.GameEnvUpdate(&req); err != nil {
			writeGameError(r.Context(), w, err)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

func GameEnvDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GameEnvDeleteRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewGameEnvDeleteLogic(r.Context(), svcCtx)
		if err := l.GameEnvDelete(&req); err != nil {
			writeGameError(r.Context(), w, err)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}
