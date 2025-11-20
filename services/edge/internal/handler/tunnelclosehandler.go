package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/edge/internal/logic"
	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func TunnelCloseHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TunnelCloseRequest
		if err := httpx.ParseVars(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewTunnelCloseLogic(r.Context(), svcCtx)
		resp, err := l.TunnelClose(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}