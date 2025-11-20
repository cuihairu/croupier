package handler

import (
	"net/http"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/logic"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AgentAnalyticsFiltersHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(svcCtx.AgentMetaToken())
		if token == "" {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusServiceUnavailable, map[string]any{
				"message": "agent filters disabled",
			})
			return
		}
		if r.Header.Get("X-Agent-Token") != token {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]any{
				"message": "unauthorized",
			})
			return
		}
		var req types.AnalyticsFiltersQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewAgentAnalyticsFiltersLogic(r.Context(), svcCtx)
		resp, err := l.AgentAnalyticsFilters(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
