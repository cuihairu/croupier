package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func HealthzHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewHealthzLogic(r.Context(), svcCtx)

		// Check if detailed health is requested
		if r.URL.Query().Get("format") == "json" {
			resp, err := l.DetailedHealth()
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			httpx.OkJsonCtx(r.Context(), w, resp)
			return
		}

		// Basic health check
		resp, err := l.Healthz()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(resp))
	}
}
