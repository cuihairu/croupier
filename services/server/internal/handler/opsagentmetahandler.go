package handler

import (
	"errors"
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func OpsAgentMetaHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OpsAgentMetaUpdateRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewOpsAgentMetaLogic(r.Context(), svcCtx)
		resp, err := l.OpsAgentMeta(&req)
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
