package handler

import (
	"net"
	"net/http"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/logic"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AuthLoginHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AuthLoginRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		ip := clientIP(r)
		ua := r.Header.Get("User-Agent")
		l := logic.NewAuthLoginLogic(r.Context(), svcCtx)
		resp, err := l.AuthLogin(&req, ip, ua)
		if err != nil {
			switch err {
			case logic.ErrInvalidRequest:
				httpx.WriteJsonCtx(r.Context(), w, http.StatusBadRequest, map[string]string{"message": "invalid request"})
			case logic.ErrAuthDisabled:
				httpx.WriteJsonCtx(r.Context(), w, http.StatusServiceUnavailable, map[string]string{"message": "auth disabled"})
			case logic.ErrLoginRateLimit:
				httpx.WriteJsonCtx(r.Context(), w, http.StatusTooManyRequests, map[string]string{"message": "too many login attempts"})
			case logic.ErrUnauthorized:
				httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			default:
				httpx.ErrorCtx(r.Context(), w, err)
			}
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func clientIP(r *http.Request) string {
	if xf := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xf != "" {
		parts := strings.Split(xf, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
