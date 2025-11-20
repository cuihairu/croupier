package handler

import (
	"net/http"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func OpsNodeCommandsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(svcCtx.AgentMetaToken())
		if token == "" {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusServiceUnavailable, map[string]any{
				"message": "node commands disabled",
			})
			return
		}
		if r.Header.Get("X-Agent-Token") != token {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]any{
				"message": "unauthorized",
			})
			return
		}

		var req types.OpsNodeCommandsQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewOpsNodeCommandsLogic(r.Context(), svcCtx)
		resp, err := l.OpsNodeCommands(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
