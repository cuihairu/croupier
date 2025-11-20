package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/agent/internal/logic"
	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func JobExecuteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.JobExecuteRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewJobExecuteLogic(r.Context(), svcCtx)
		resp, err := l.JobExecute(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}