package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AgentMetaHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(svcCtx.AgentMetaToken())
		if token == "" {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusServiceUnavailable, map[string]any{
				"message": "agent meta disabled",
			})
			return
		}
		if r.Header.Get("X-Agent-Token") != token {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]any{
				"message": "unauthorized",
			})
			return
		}

		var req types.AgentMetaReportRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewAgentMetaLogic(r.Context(), svcCtx)
		resp, err := l.AgentMeta(&req)
		if err != nil {
			if errors.Is(err, logic.ErrAgentNotFound) {
				httpx.WriteJsonCtx(r.Context(), w, http.StatusNotFound, map[string]any{"message": "agent not found"})
			} else {
				httpx.ErrorCtx(r.Context(), w, err)
			}
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
