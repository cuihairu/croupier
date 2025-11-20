package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/api/internal/logic"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AuthMeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		l := logic.NewAuthMeLogic(r.Context())
		resp, err := l.AuthMe(user, roles)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
