package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/edge/internal/logic"
	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func EdgeMetricsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.EdgeMetricsRequest
		if err := httpx.ParseForm(r); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewEdgeMetricsLogic(r.Context(), svcCtx)
		resp, err := l.EdgeMetrics(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}