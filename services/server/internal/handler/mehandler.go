package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func MeProfileHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        user, _, ok := svcCtx.Authenticate(r)
        if !ok {
            httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
            return
        }
        l := logic.NewMeLogic(r.Context(), svcCtx)
        resp, err := l.Profile(user)
        if err != nil {
            writeMeError(r.Context(), w, err)
            return
        }
        httpx.OkJsonCtx(r.Context(), w, resp)
    }
}

func MeGamesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        user, _, ok := svcCtx.Authenticate(r)
        if !ok {
            httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
            return
        }
        l := logic.NewMeLogic(r.Context(), svcCtx)
        resp, err := l.Games(user)
        if err != nil {
            writeMeError(r.Context(), w, err)
            return
        }
        httpx.OkJsonCtx(r.Context(), w, resp)
    }
}

func MeProfileUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        user, _, ok := svcCtx.Authenticate(r)
        if !ok {
            httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
            return
        }
        var req types.MeProfileUpdateRequest
        if err := httpx.Parse(r, &req); err != nil {
            httpx.ErrorCtx(r.Context(), w, err)
            return
        }
        l := logic.NewMeLogic(r.Context(), svcCtx)
        if err := l.UpdateProfile(user, &req); err != nil {
            writeMeError(r.Context(), w, err)
            return
        }
        w.WriteHeader(http.StatusNoContent)
    }
}

func MePasswordHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        user, _, ok := svcCtx.Authenticate(r)
        if !ok {
            httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
            return
        }
        var req types.MePasswordRequest
        if err := httpx.Parse(r, &req); err != nil {
            httpx.ErrorCtx(r.Context(), w, err)
            return
        }
        l := logic.NewMeLogic(r.Context(), svcCtx)
        if err := l.UpdatePassword(user, &req); err != nil {
            writeMeError(r.Context(), w, err)
            return
        }
        w.WriteHeader(http.StatusNoContent)
    }
}

func writeMeError(ctx context.Context, w http.ResponseWriter, err error) {
    switch {
    case errors.Is(err, logic.ErrInvalidRequest):
        httpx.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"message": "invalid request"})
    case errors.Is(err, logic.ErrNotFound):
        httpx.WriteJsonCtx(ctx, w, http.StatusNotFound, map[string]string{"message": "not found"})
    case errors.Is(err, logic.ErrUnavailable):
        httpx.WriteJsonCtx(ctx, w, http.StatusServiceUnavailable, map[string]string{"message": "unavailable"})
    case errors.Is(err, logic.ErrUnauthorized):
        httpx.WriteJsonCtx(ctx, w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
    default:
        httpx.ErrorCtx(ctx, w, err)
    }
}
