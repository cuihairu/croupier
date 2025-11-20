package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/api/internal/logic"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func SupportTicketCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SupportTicketCreateRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewSupportTicketCreateLogic(r.Context(), svcCtx)
		resp, err := l.SupportTicketCreate(&req)
		if err != nil {
			writeSupportError(r.Context(), w, err)
		} else {
			w.WriteHeader(http.StatusCreated)
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
