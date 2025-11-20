package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/api/internal/logic"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func OpsMaintenanceGetHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewOpsMaintenanceGetLogic(r.Context(), svcCtx)
		resp, err := l.OpsMaintenanceGet()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
